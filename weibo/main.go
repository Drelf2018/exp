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
	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/cookie"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"github.com/playwright-community/playwright-go"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	sloglogrus "github.com/samber/slog-logrus/v2"
)

type Options struct {
	Me       int           `short:"m" long:"me" description:"你的微博 UID"`
	Target   int           `short:"t" long:"target" description:"监控目标 UID"`
	Logger   string        `short:"l" long:"logger" description:"日志文件路径"`
	Crontab  string        `short:"c" long:"crontab" description:"刷新 Cookie 任务"`
	Database string        `short:"d" long:"database" description:"数据库文件路径"`
	DingTalk *dingtalk.Bot `group:"DingTalk" description:"钉钉机器人"`
}

var (
	options Options
	logger  *logrus.Logger
	bot     *logrus.Entry
	jar     *CookieJar
	db      *gorm.DB
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
		OnError:   func(err error) { bot.WithField("title", "微博保活失败").Error(err) },
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
	db, err = gorm.Open(sqlite.Open(options.Database))
	if err != nil {
		logger.Panicln("创建数据库失败:", err)
	}
	err = db.AutoMigrate(&model.Blog{})
	if err != nil {
		logger.Panicln("自动迁移数据库失败:", err)
	}
}

// BlogMsg 博文模板
type BlogMsg dingtalk.ActionCard

func (BlogMsg) Type() dingtalk.MsgType {
	return dingtalk.MsgActionCard
}

// 初始化博文模板
func init() {
	err := options.DingTalk.Funcs(template.FuncMap{"suffix": strings.HasSuffix, "prefix": hook.Prefix}).Parse(BlogMsg{
		Title:     " {{.}}",
		Text:      "{{if .Banner}}![]({{.Banner}})\n\n{{end}}{{template \"blog\" .}}\n\n###### {{.Time.Format \"2006-01-02 15:04:05\"}}",
		SingleURL: "{{.URL}}",
	})
	if err != nil {
		logger.Panicln("创建博文模板失败:", err)
	}
	err = options.DingTalk.NewTemplate("blog", `### {{.Name}}{{if (and .Title (ne .Type "like"))}} {{.Title}}{{end}}

{{prefix .Plaintext "#### "}}{{range $idx, $asset := .Assets}}{{if or (suffix $asset ".jpg") (suffix $asset ".jpeg") (suffix $asset ".png")}}

![]({{$asset}}){{end}}{{end}}{{if .Reply}}

{{template "blog" .Reply}}{{end}}`)
	if err != nil {
		logger.Panicln("创建博文子模板失败:", err)
	}
}

// send 发送通知
func send(ctx context.Context, blog *model.Blog, jar http.CookieJar) {
	if blog.Type == "like" {
		wrapper := &model.Blog{
			UID:       strconv.Itoa(options.Target),
			Avatar:    blog.Avatar,
			URL:       blog.URL,
			Plaintext: blog.Title,
			Extra:     model.Extra{},
		}
		SetProfileInfo(ctx, wrapper, jar)
		wrapper.Reply = blog
		blog = wrapper
	}
	msg := &BlogMsg{SingleTitle: "阅读全文"}
	_, err := options.DingTalk.Fill(blog, msg)
	if err != nil {
		bot.WithField("title", "执行模板失败").Error(err)
	} else {
		// 重试三次，如果一直系统繁忙则切换发送方式
		handler := dingtalk.UUID(uuid.NewString())
		for i := range 3 {
			if i != 0 {
				time.Sleep((1 << i) * time.Second)
			}
			err := options.DingTalk.Send(msg, handler)
			if err == nil {
				return
			}
			if respErr, ok := err.(dingtalk.SendError); ok && respErr.ErrCode == -1 {
				continue
			}
			if urlErr, ok := err.(*url.Error); ok {
				err = urlErr.Unwrap()
			}
			bot.WithField("title", "发送微博失败").Error(err)
			break
		}
	}
	err = options.DingTalk.SendLink(blog.Name, blog.Plaintext, blog.URL, blog.Avatar)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			err = urlErr.Unwrap()
		}
		bot.WithField("title", "发送链接失败").Error(err)
	}
}

// 轮询获取微博
func main() {
	logger.Info("轮询获取微博")
	var now time.Time
	last := time.Now()
	bgCtx := context.Background()
	fetchTicker := req.NewTicker(req.RandomTicker{7 * time.Second, 10 * time.Second})
	defer fetchTicker.Stop()
	for now = range fetchTicker.C {
		logger.Debugf("获取微博 (+%s)", now.Sub(last))
		last = now
		for mblog := range GetMymlogIter(bgCtx, options.Target, jar) {
			blog := mblog.ToBlog()
			// 当前博文未保存则写入数据库，会比较编辑次数是否有差异，如果有差异会重新写入
			result := db.Scopes(blog.Match).Limit(1).Find(&model.Blog{})
			if result.Error != nil {
				bot.WithField("title", "微博查询失败").Error(result.Error)
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
			go send(bgCtx, blog, jar)
			// 写入数据库
			err := db.Create(blog).Error
			if err != nil {
				bot.WithField("title", "微博保存失败").Error(err)
			}
		}
	}
}
