package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/req/cookie"
	"github.com/gofiber/fiber/v2"
	"github.com/jessevdk/go-flags"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

type Options struct {
	Me     int           `short:"m" long:"me" description:"你的微博 UID"`
	Target int           `short:"t" long:"target" description:"监控目标 UID"`
	Port   uint16        `short:"p" long:"port" description:"后端端口"`
	Saki   *dingtalk.Bot `group:"Saki" description:"小祥钉钉机器人"`
}

var (
	opts   Options
	saki   *logrus.Entry
	logger *logrus.Logger
)

func init() {
	// 解析默认配置文件
	err := flags.IniParse("config.ini", &opts)
	if err != nil {
		logger.Panic(err)
	}
	// 解析命令行参数
	_, err = flags.Parse(&opts)
	if err != nil {
		logger.Panic(err)
	}
	if opts.Target == 0 {
		logger.Panic(errors.New("no target"))
	}
	// 初始化日志
	logger = hook.New(logrus.InfoLevel, hook.NewDailyFileHook("logs/2006-01-02.log"), hook.NewDingTalkHook(opts.Saki))
	saki = logger.WithField(hook.DingTalk, opts.Saki.Name)
}

func init() {
	logger.Debug("安装 playwright")
	// 安装 playwright
	err := playwright.Install()
	if err != nil {
		logger.Panicln("could not install playwright:", err)
	}
	// 启动 playwright
	logger.Debug("启动 playwright")
	pw, err := playwright.Run()
	if err != nil {
		logger.Panicln("could not start playwright:", err)
	}
	// 启动 Chromium 浏览器
	logger.Debug("启动 Chromium 浏览器")
	browser, err = pw.Chromium.Launch()
	if err != nil {
		logger.Panicln("could not launch browser:", err)
	}
}

func fetch(ctx context.Context, jar http.CookieJar, target int) {
	blogs := make(map[string]Mblog)
	cleanTicker := time.NewTicker(24 * time.Hour)
	defer cleanTicker.Stop()
	fetchTicker := time.NewTicker(10 * time.Second)
	defer fetchTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanTicker.C:
			for k, v := range blogs {
				created, err := time.Parse(time.RubyDate, v.CreatedAt)
				if err != nil || time.Since(created) > 14*24*time.Hour {
					delete(blogs, k)
					logger.Infoln("Blog", k, "has been deleted")
				}
			}
		case <-fetchTicker.C:
			logger.Debugf("Getting blogs (%d blogs have been saved)", len(blogs))
			for mblog := range GetMymlogIter(ctx, target, jar) {
				// 当前博文未保存则写入文件，并且保存在映射中，会比较编辑次数是否有差异，如果有差异则写入新文件
				if v, ok := blogs[mblog.Mid]; ok && v.EditCount == mblog.EditCount {
					continue
				}
				blogs[mblog.Mid] = mblog
				filename := "./blogs/" + mblog.Mid
				if mblog.EditCount != 0 {
					filename += "." + strconv.Itoa(mblog.EditCount)
				}
				// 已经保存过文件
				if _, err := os.Stat(filename + ".json"); err == nil {
					continue
				}
				// 推送
				logger.Infoln("Saving blog", mblog.Mid)
				blog := mblog.ToBlog(ctx, jar)
				go func(blog *model.Blog) {
					var b strings.Builder
					RenderMarkdown(&b, blog, 0)
					err := opts.Saki.SendSingleActionCardWithContext(ctx, fmt.Sprintln(blog.Name, blog.Plaintext), b.String(), "阅读全文", blog.URL)
					if err != nil {
						saki.Error(err)
						err = opts.Saki.SendLinkWithContext(ctx, blog.Name, blog.Plaintext, blog.Avatar, blog.URL)
						if err != nil {
							saki.Error(err)
						}
					}
				}(blog)
				// 保存至文件
				b, err := json.MarshalIndent(blog, "", "\t")
				if err != nil {
					saki.Error(err)
					continue
				}
				err = os.WriteFile(filename+".json", b, os.ModePerm)
				if err != nil {
					saki.Error(err)
				}
			}
		}
	}
}

func main() {
	// 获取 Cookie
	c, err := NewCookieJar(opts.Me)
	if err != nil {
		logger.Panic(err)
	}
	// 初始化博文保存目录
	err = os.MkdirAll("./blogs", os.ModePerm)
	if err != nil {
		logger.Panic(err)
	}
	err = os.MkdirAll("./blogs/screenshots", os.ModePerm)
	if err != nil {
		logger.Panic(err)
	}
	// 开启 Cookie 保活
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(NextTimeDuration(3, 0, 0), func() {
		k := &cookie.KeepaliveCookieJar{CookieJar: c, Refresher: Refresher}
		go k.Keepalive(ctx, 6*time.Hour, true)
	})
	// 轮询获取博文
	go fetch(ctx, c, opts.Target)
	// 创建后端
	app := fiber.New()
	app.Static("/", "./blogs", fiber.Static{Browse: true})
	// 监听程序关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// 阻塞等待信号
		<-sigCh
		cancel()
		close(sigCh)
		app.Shutdown()
	}()
	// 开启后端
	app.Listen(fmt.Sprintf("0.0.0.0:%d", opts.Port))
}
