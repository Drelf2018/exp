package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/exp/qiniu"
	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/cookie"
	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"github.com/playwright-community/playwright-go"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	sloglogrus "github.com/samber/slog-logrus/v2"
)

type Options struct {
	Me       int                      `long:"me" description:"你的微博 UID"`
	Target   int                      `long:"target" description:"监控目标 UID"`
	Logger   string                   `long:"logger" description:"日志文件路径"`
	Crontab  string                   `long:"crontab" description:"刷新 Cookie 任务"`
	Database model.Database           `long:"database" description:"数据库文件路径"`
	DingTalk *dingtalk.Bot            `group:"DingTalk" description:"钉钉机器人"`
	Qiniu    *qiniu.TemporaryUploader `group:"Qiniu" description:"七牛云凭证"`
}

var (
	options Options
	logger  *logrus.Logger
	bot     *logrus.Entry
	jar     *CookieJar
	db      *gorm.DB
	tmpl    *template.Template
)

// 获取运行参数
func init() {
	// 解析默认配置文件
	err := flags.IniParse("config.ini", &options)
	if err != nil {
		logrus.Panic(err)
	}
	// 解析命令行参数
	_, err = flags.Parse(&options)
	if err != nil {
		logrus.Panic(err)
	}
	if options.Target == 0 {
		logrus.Panic("no target")
	}
	// 初始化日志
	ding := hook.NewDingTalkHook(options.DingTalk)
	logger = hook.New(logrus.InfoLevel, hook.NewDailyFileHook(options.Logger), ding)
	bot = ding.Bind(logger)
}

// 初始化浏览器
func init() {
	logger.Info("安装 playwright")
	err := playwright.Install(&playwright.RunOptions{
		Verbose: true,
		Logger:  slog.New(sloglogrus.Option{Logger: logger}.NewLogrusHandler()),
	})
	if err != nil {
		logger.Panicln("安装 playwright 失败:", err)
	}
	logger.Info("启动 playwright")
	pw, err := playwright.Run()
	if err != nil {
		logger.Panicln("启动 playwright 失败:", err)
	}
	logger.Info("启动 Chromium 浏览器")
	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Args: []string{
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-features=AutomationControlled",
		},
	})
	if err != nil {
		logger.Panicln("启动 Chromium 浏览器失败:", err)
	}
	logger.Info("获取 Cookie 对象")
	jar, err = NewCookieJar(options.Me)
	if err != nil {
		logger.Panicln("读取 cookie 失败:", err)
	}
	logger.Info("初始化 Cookie")
	err = RefreshWeiboCookie(context.Background(), jar)
	if err != nil {
		logger.Panicln("刷新 Cookie 失败:", err)
	}
	logger.Infoln("开启 Cookie 保活:", options.Crontab)
	c := cron.New()
	_, err = c.AddJob(options.Crontab, &cookie.KeepaliveCookieJar{
		CookieJar: jar,
		Refresher: cookie.ForcedRefresher(RefreshWeiboCookie),
		OnError:   func(err error) { bot.WithError(err).Error("微博保活失败") },
	})
	if err != nil {
		logger.Panicln("添加任务失败:", err)
	}
	c.Start()
}

// 初始化数据库
func init() {
	var err error
	logger.Info("初始化数据库")
	db, err = options.Database.Open(&model.Blog{})
	if err != nil {
		logger.Panicln("初始化数据库失败:", err)
	}
}

// 初始化博文模板
func init() {
	var err error
	funcMap := template.FuncMap{"suffix": strings.HasSuffix, "prefix": hook.Prefix}
	tmpl, err = template.New("").Funcs(funcMap).Parse("{{if .Banner}}![]({{.Banner}})\n\n{{end}}{{template \"blog\" .}}\n\n###### {{.Time.Format \"2006-01-02 15:04:05\"}}")
	if err != nil {
		logger.Panicln("创建博文模板失败:", err)
	}
	_, err = tmpl.New("blog").Parse(`### {{.Name}}{{if (and .Title (ne .Type "like"))}} {{.Title}}{{end}}

{{prefix .Plaintext "#### "}}{{range .Assets}}{{if or (suffix . ".jpg") (suffix . ".jpeg") (suffix . ".png")}}

![]({{.}}){{end}}{{end}}{{if .Reply}}

{{template "blog" .Reply}}{{end}}`)
	if err != nil {
		logger.Panicln("创建博文子模板失败:", err)
	}
}

// sendLink 发送链接
func sendLink(ctx context.Context, blog *model.Blog) {
	err := options.DingTalk.SendLinkWithContext(ctx, blog.Name, blog.Plaintext, blog.URL, blog.Avatar)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			err = urlErr.Unwrap()
		}
		bot.WithError(err).Error("发送链接失败")
	}
}

// send 发送通知
func send(ctx context.Context, blog *model.Blog, jar http.CookieJar) {
	if blog.Type == "like" {
		wrapper := &model.Blog{
			UID:       strconv.Itoa(options.Target),
			Avatar:    blog.Avatar,
			URL:       blog.URL,
			Time:      blog.Time,
			Plaintext: blog.Title,
			Extra:     model.Extra{},
		}
		SetProfileInfo(ctx, wrapper, jar)
		wrapper.Reply = blog
		blog = wrapper
	}
	var b strings.Builder
	err := tmpl.Execute(&b, blog)
	if err != nil {
		// 执行模板失败，退避为发送链接
		bot.WithError(err).Error("执行模板失败")
		sendLink(ctx, blog)
		return
	}
	// 重试三次，如果一直系统繁忙则切换发送方式
	msgUUID := dingtalk.UUID(uuid.NewString())
	for i := range 3 {
		if i != 0 {
			time.Sleep((1 << i) * time.Second)
		}
		// 发送成功，直接返回
		err = options.DingTalk.SendActionCard(" "+blog.String(), b.String(), "阅读全文", blog.URL, msgUUID)
		if err == nil {
			return
		}
		// 服务器系统繁忙，等待后重试
		if respErr, ok := err.(dingtalk.SendError); ok && respErr.ErrCode == -1 {
			continue
		}
		// 其他错误，不再重试
		if urlErr, ok := err.(*url.Error); ok {
			err = urlErr.Unwrap()
		}
		break
	}
	// 发送卡片失败，退避为发送链接
	bot.WithError(err).Error("发送微博失败")
	sendLink(ctx, blog)
}

// 轮询获取微博
func main() {
	logger.Info("轮询获取微博")
	var last time.Time
	now := time.Now()
	bgCtx := context.Background()
	for range req.WithDelay(req.RandomDelayer{7 * time.Second, 10 * time.Second}) {
		last, now = now, time.Now()
		logger.Debugf("获取微博 (+%s)", now.Sub(last))
		for mblog := range GetMymlogIter(bgCtx, options.Target, jar) {
			blog := mblog.ToBlog()
			// 当前博文未保存则写入数据库，会比较编辑次数是否有差异，如果有差异会重新写入
			result := db.Scopes(blog.Match).Limit(1).Find(&model.Blog{})
			if result.Error != nil {
				bot.WithError(result.Error).Error("微博查询失败")
				continue
			}
			// 已经保存过则跳过
			if result.RowsAffected != 0 {
				continue
			}
			// 否则补充博主信息
			SetProfileInfo(bgCtx, blog, jar)
			logger.Infoln("保存微博:", blog)
			// 异步通知
			sendBlog := *blog
			go send(bgCtx, &sendBlog, jar)
			// 写入数据库
			err := db.Create(blog).Error
			if err != nil {
				bot.WithError(err).Error("微博保存失败")
			}
		}
	}
}
