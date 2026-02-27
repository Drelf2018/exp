package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"time"

	"github.com/Drelf2018/exp/app/weibo/config"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/req"
	"github.com/playwright-community/playwright-go"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	sloglogrus "github.com/samber/slog-logrus/v2"
)

var (
	logger         *logrus.Logger
	entry          *logrus.Entry
	browserContext playwright.BrowserContext
)

// Screenshot 访问主页并截图
func Screenshot(ctx context.Context, homepage string) ([]byte, http.CookieJar, error) {
	// 新建页面
	logger.Debug("新建页面")
	page, err := browserContext.NewPage()
	if err != nil {
		return nil, nil, fmt.Errorf("新建页面失败: %w", err)
	}
	defer page.Close()
	// 访问微博主页实现 Cookie 刷新
	logger.Debug("访问微博主页")
	_, err = page.Goto(homepage)
	if err != nil {
		return nil, nil, fmt.Errorf("访问微博主页失败: %w", err)
	}
	// 轮询等待页面加载后获取 Cookie
	logger.Debug("轮询等待页面加载")
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("获取 Cookie 超时: %w", ctx.Err())
		case <-ticker.C:
			logger.Debug("开始获取 Cookie")
			pwCookies, err := browserContext.Cookies(homepage)
			if err != nil {
				return nil, nil, fmt.Errorf("获取 Cookie 失败: %w", err)
			}
			var hasToken bool
			cookies := make([]*http.Cookie, 0, len(pwCookies))
			for _, cookie := range pwCookies {
				logger.Debugln(cookie.Name, "=", cookie.Value)
				if cookie.Name == "XSRF-TOKEN" {
					hasToken = true
				}
				cookies = append(cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value})
			}
			// 缺少 XSRF-TOKEN
			if !hasToken {
				logger.Debug("缺少 XSRF-TOKEN")
				continue
			}
			logger.Debug("验证 Cookie")
			jar, err := cookiejar.New(nil)
			if err != nil {
				return nil, nil, fmt.Errorf("创建 CookieJar 失败: %w", err)
			}
			jar.SetCookies(session.BaseURL, cookies)
			logger.Debug("主页截图")
			img, err := page.Screenshot()
			if err != nil {
				return nil, jar, err
			}
			return img, jar, nil
		}
	}
}

func Run(ctx context.Context, options config.Options) error {
	// 初始化日志
	level, err := logrus.ParseLevel(options.Logger.Level)
	if err != nil {
		return fmt.Errorf("日志等级错误: %w", err)
	}
	ding := hook.NewDingTalkHook(options.DingTalk)
	logger = hook.New(level, hook.NewDailyFileHook(options.Logger.Filename), ding)
	entry = ding.Bind(logger)
	// 初始化浏览器
	logger.Debug("安装 playwright")
	err = playwright.Install(&playwright.RunOptions{
		Verbose: true,
		Logger:  slog.New(sloglogrus.Option{Logger: logger}.NewLogrusHandler()),
	})
	if err != nil {
		return fmt.Errorf("安装 playwright 失败: %w", err)
	}
	logger.Debug("启动 playwright")
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("启动 playwright 失败: %w", err)
	}
	logger.Debug("启动 Chromium 浏览器")
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Args: []string{
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-features=AutomationControlled",
			"--disable-blink-features=AutomationControlled", // 移除自动化控制特征
			"--disable-features=IsolateOrigins,site-per-process",
			"--disable-features=BlockInsecurePrivateNetworkRequests",
			"--window-size=1920,1080", // 模拟有头窗口大小
			"--start-maximized",
		},
	})
	if err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}
	defer browser.Close()
	logger.Debug("读取浏览器状态")
	b, err := os.ReadFile(options.State)
	if err != nil {
		return fmt.Errorf("读取浏览器状态失败: %w", err)
	}
	logger.Debug("反序列化浏览器状态")
	var storageState playwright.OptionalStorageState
	err = json.Unmarshal(b, &storageState)
	if err != nil {
		return fmt.Errorf("反序列化浏览器状态失败: %w", err)
	}
	logger.Debug("创建浏览器上下文")
	browserContext, err = browser.NewContext(playwright.BrowserNewContextOptions{StorageState: &storageState})
	if err != nil {
		return fmt.Errorf("创建浏览器上下文失败: %w", err)
	}
	logger.Debug("初始化 Cookie")
	_, jar, err := Screenshot(ctx, session.BaseURL.String())
	if jar == nil {
		return fmt.Errorf("获取 Cookie 失败: %w", err)
	}
	_, err = GetMymlog(ctx, options.Target, jar)
	if err != nil {
		return fmt.Errorf("验证 Cookie 失败: %w", err)
	}
	logger.Debugln("开启 Cookie 保活:", options.Crontab)
	c := cron.New()
	_, err = c.AddFunc(options.Crontab, func() {
		img, c, err := Screenshot(ctx, session.BaseURL.String())
		if c != nil {
			jar = c
		}
		if err != nil {
			if c == nil {
				entry.WithError(err).Error("微博保活失败")
			} else {
				entry.WithError(err).Error("微博截图失败")
			}
			return
		}
		objectName := time.Now().Format("weibo/2006_01_02_15_04_05.jpg")
		err = options.Qiniu.Upload(ctx, bytes.NewReader(img), objectName)
		if err != nil {
			entry.WithError(err).Error("截屏上传失败")
		} else {
			entry.WithFields(logrus.Fields{
				"banner": fmt.Sprintf("![](https://yun.nana7mi.link/%s)", objectName),
				"title":  "微博刷新成功",
			}).Info()
		}
	})
	if err != nil {
		return fmt.Errorf("添加任务失败: %w", err)
	}
	c.Start()
	// 初始化数据库
	logger.Debug("初始化数据库")
	db, err := options.Database.Open(&model.Blog{})
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	// 轮询获取微博
	logger.Info("开始获取微博")
	var last time.Time
	now := time.Now()
	for range req.WithDelay(req.RandomDelayer{7 * time.Second, 10 * time.Second}) {
		last, now = now, time.Now()
		logger.Debugf("获取微博 (+%s)", now.Sub(last))
		r, err := GetMymlog(ctx, options.Target, jar)
		if err != nil {
			entry.WithError(err).Error("获取微博失败")
			continue
		}
		for _, mblog := range r.Data.List {
			blog := mblog.ToBlog()
			// 当前博文未保存则写入数据库，会比较编辑次数是否有差异，如果有差异会重新写入
			result := db.Scopes(blog.Match).Limit(1).Find(&model.Blog{})
			if result.Error != nil {
				entry.WithError(result.Error).Error("查询微博失败")
				continue
			}
			// 已经保存过则跳过
			if result.RowsAffected != 0 {
				continue
			}
			// 否则补充博主信息
			SetProfileInfo(ctx, blog, jar)
			logger.Info(blog)
			// 异步通知
			go func(send model.Blog) {
				p := &send
				if send.Type == "like" {
					wrapper := &model.Blog{
						UID:       strconv.Itoa(options.Target),
						Avatar:    send.Avatar,
						URL:       send.URL,
						Time:      send.Time,
						Plaintext: send.Title,
						Extra:     model.Extra{},
					}
					SetProfileInfo(ctx, wrapper, jar)
					wrapper.Reply, p = p, wrapper
				}
				// 发送卡片失败则退避为发送链接
				err := SendActionCard(ctx, options.DingTalk, p)
				if err != nil {
					entry.WithError(err).Error("发送卡片失败")
					err = SendLink(ctx, options.DingTalk, p)
					if err != nil {
						entry.WithError(err).Error("发送链接失败")
					}
				}
			}(*blog)
			// 写入数据库
			err := db.Create(blog).Error
			if err != nil {
				entry.WithError(err).Error("保存微博失败")
			}
		}
	}
	return nil
}
