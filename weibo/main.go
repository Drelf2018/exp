package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/cookie"
	"github.com/glebarez/sqlite"
	"github.com/jessevdk/go-flags"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Options struct {
	Me       int           `short:"m" long:"me" description:"你的微博 UID"`
	Target   int           `short:"t" long:"target" description:"监控目标 UID"`
	Database string        `short:"d" long:"database" description:"数据库文件路径"`
	Saki     *dingtalk.Bot `group:"Saki" description:"小祥钉钉机器人"`
}

var (
	options Options
	logger  *logrus.Logger
	saki    *logrus.Entry
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
	logger = hook.New(logrus.InfoLevel, hook.NewDailyFileHook("logs/2006-01-02.log"), hook.NewDingTalkHook(options.Saki))
	saki = logger.WithField(hook.DingTalk, options.Saki.Name)
}

// 初始化数据库
func init() {
	logger.Debug("初始化数据库")
	var err error
	db, err = gorm.Open(sqlite.Open(options.Database))
	if err != nil {
		logger.Panicln("创建数据库失败:", err)
	}
	err = db.AutoMigrate(&model.Blog{})
	if err != nil {
		logger.Panicln("自动迁移数据库失败:", err)
	}
}

// NextTimeDuration 返回距离下次指定时间的间隔
func NextTimeDuration(hour, min, sec int) time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())
	switch {
	case now.Hour() > hour,
		now.Hour() == hour && now.Minute() > min,
		now.Hour() == hour && now.Minute() == min && now.Second() > sec:
		next = next.AddDate(0, 0, 1)
	}
	return time.Until(next)
}

// 初始化浏览器
func init() {
	logger.Debug("安装 playwright")
	// 安装 playwright
	err := playwright.Install()
	if err != nil {
		logger.Panicln("安装 playwright 失败:", err)
	}
	// 启动 playwright
	logger.Debug("启动 playwright")
	pw, err := playwright.Run()
	if err != nil {
		logger.Panicln("启动 playwright 失败:", err)
	}
	// 启动 Chromium 浏览器
	logger.Debug("启动 Chromium 浏览器")
	browser, err = pw.Chromium.Launch()
	if err != nil {
		logger.Panicln("启动 Chromium 浏览器失败:", err)
	}
	// 开启 Cookie 保活
	jar, err = NewCookieJar(options.Me)
	if err != nil {
		logger.Panicln("读取 cookie 失败:", err)
	}
	time.AfterFunc(NextTimeDuration(3, 0, 0), func() { cookie.Keepalive(jar, Refresher, 6*time.Hour) })
}

// RenderMarkdown 将通用博文模型格式化成 Markdown
func RenderMarkdown(b *strings.Builder, blog *model.Blog, depth int) {
	prefix := strings.Repeat(">", depth) + " "
	if depth == 0 && blog.Banner != "" {
		if depth != 0 {
			b.WriteString(prefix)
		}
		b.WriteString("![](")
		b.WriteString(blog.Banner)
		b.WriteString(")\n")
		if depth == 0 {
			b.WriteByte('\n')
		}
	}
	if depth != 0 {
		b.WriteString(prefix)
	}
	b.WriteString("### ")
	b.WriteString(blog.Name)
	if blog.Title != "" {
		b.WriteString(" ")
		b.WriteString(blog.Title)
	}
	b.WriteByte('\n')
	if depth == 0 {
		b.WriteByte('\n')
	} else {
		b.WriteString(prefix)
	}
	b.WriteString(strings.TrimSpace(blog.Content))
	b.WriteByte('\n')
	if depth == 0 {
		b.WriteByte('\n')
	}
	for _, a := range blog.Assets {
		ext := filepath.Ext(a)
		if ext == "" || (ext != ".jpg" && ext != ".jpeg" && ext != ".png") {
			continue
		}
		if depth != 0 {
			b.WriteString(prefix)
		}
		b.WriteString("![](")
		b.WriteString(a)
		b.WriteByte(')')
		if depth == 0 {
			b.WriteByte('\n')
		}
	}
	if blog.Reply != nil {
		RenderMarkdown(b, blog.Reply, depth+1)
	}
}

// send 发送通知
func send(ctx context.Context, blog *model.Blog) {
	var b strings.Builder
	RenderMarkdown(&b, blog, 0)
	err := options.Saki.SendSingleActionCardWithContext(ctx, " "+blog.String(), b.String(), "阅读全文", blog.URL)
	if err != nil {
		saki.Error(err)
		err = options.Saki.SendLinkWithContext(ctx, blog.Name, blog.Plaintext, blog.Avatar, blog.URL)
		if err != nil {
			saki.Error(err)
		}
	}
}

// 轮询获取博文
func main() {
	var (
		now    time.Time
		ctx    context.Context
		cancel context.CancelFunc
		last   = time.Now()
		bg     = context.Background()
	)
	fetchTicker := req.NewTicker(req.RandomTicker{7 * time.Second, 10 * time.Second})
	defer fetchTicker.Stop()
	for now = range fetchTicker.C {
		ctx, cancel = context.WithDeadline(bg, now.Add(7*time.Second))
		logger.Debugf("获取微博 (+%s)", now.Sub(last))
		last = now
		for mblog := range GetMymlogIter(ctx, options.Target, jar) {
			blog := mblog.ToBlog()
			// 当前博文未保存则写入数据库，会比较编辑次数是否有差异，如果有差异会重新写入
			result := db.Scopes(blog.Match).Limit(1).Find(&model.Blog{})
			if result.Error != nil {
				saki.WithField("title", "微博查询失败").Error(result.Error)
				continue
			}
			// 已经保存过
			if result.RowsAffected != 0 {
				continue
			}
			// 否则先补充博主信息
			SetProfileInfo(ctx, blog, jar)
			logger.Infoln("保存微博:", blog)
			// 通知
			go send(ctx, blog)
			// 保存至数据库
			err := db.Create(blog).Error
			if err != nil {
				saki.WithField("title", "微博保存失败").Error(err)
			}
		}
		cancel()
	}
}
