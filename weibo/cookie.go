package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Drelf2018/req/cookie"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

var browser playwright.Browser

func init() {
	logrus.Debug("安装 playwright")
	// 安装 playwright
	err := playwright.Install()
	if err != nil {
		logrus.Panicln("could not install playwright:", err)
	}
	// 启动 playwright
	logrus.Debug("启动 playwright")
	pw, err := playwright.Run()
	if err != nil {
		logrus.Panicln("could not start playwright:", err)
	}
	// 启动 Chromium 浏览器
	logrus.Debug("启动 Chromium 浏览器")
	browser, err = pw.Chromium.Launch()
	if err != nil {
		logrus.Panicln("could not launch browser:", err)
	}
}

func Refresh(ctx context.Context, jar http.CookieJar) error {
	// 创建浏览器上下文
	logger.Debug("创建浏览器上下文")
	context, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("could not create context: %w", err)
	}
	defer context.Close()
	// 添加 Cookie
	logger.Debug("添加 Cookie")
	cookies := make([]playwright.OptionalCookie, 0)
	for _, cookie := range jar.Cookies(Session.BaseURL) {
		cookies = append(cookies, playwright.OptionalCookie{
			Name:   cookie.Name,
			Value:  cookie.Value,
			Domain: playwright.String("." + Session.BaseURL.Host),
			Path:   playwright.String("/"),
		})
	}
	err = context.AddCookies(cookies)
	if err != nil {
		return fmt.Errorf("could not add cookies: %w", err)
	}
	// 新建页面
	logger.Debug("新建页面")
	page, err := context.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}
	defer page.Close()
	// 访问微博主页实现 Cookie 刷新
	logger.Debug("访问微博主页实现 Cookie 刷新")
	_, err = page.Goto("https://weibo.com/u/7198559139")
	if err != nil {
		return fmt.Errorf("could not goto: %w", err)
	}
	// 轮询等待页面加载后获取 Cookie
	logger.Debug("轮询等待页面加载后获取 Cookie")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-time.After(30 * time.Second):
			return fmt.Errorf("waiting for cookie timeout: %v", 30*time.Second)
		case <-ticker.C:
			logger.Debug("开始获取 Cookie")
			pwCookies, err := context.Cookies("https://weibo.com/u/7198559139")
			if err != nil {
				return fmt.Errorf("cookie not found: %w", err)
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
			logger.Debug("设置 Cookie")
			jar.SetCookies(Session.BaseURL, cookies)
			img, err := page.Screenshot()
			if err != nil {
				saki.Errorln("cannot screenshot:", err)
				return nil
			}
			err = os.MkdirAll("./blogs/screenshots", os.ModePerm)
			if err != nil {
				saki.Errorln("cannot mkdir:", err)
				return nil
			}
			filename := time.Now().Format("2006-01-02-15-04-05")
			err = os.WriteFile(fmt.Sprintf("./blogs/screenshots/%s.jpg", filename), img, os.ModePerm)
			if err != nil {
				saki.Errorln("cannot write file:", err)
			}
			// 刷新成功
			saki.Infof("![%s 刷新成功](http://api.nana7mi.link:%d/screenshots/%s.jpg)", filename, port, filename)
			return nil
		}
	}
}

// Refresher 使用 playwright 访问微博主页进行刷新
var Refresher cookie.Refresher = cookie.ForcedRefresher(Refresh)

// CookieJar 每次设置 Cookie 时将其保存在本地
type CookieJar struct {
	http.CookieJar
	UID int
}

func (c *CookieJar) OnError(err error) {
	if err != nil {
		saki.Error(err)
	}
}

var _ interface{ OnError(error) } = (*CookieJar)(nil)

func (c *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	// 更新底层 http.CookieJar
	c.CookieJar.SetCookies(u, cookies)
	// 更新本地 Cookie
	req := &http.Request{Header: make(http.Header)}
	for _, cookie := range c.CookieJar.Cookies(Session.BaseURL) {
		req.AddCookie(cookie)
	}
	c.OnError(os.WriteFile(strconv.Itoa(c.UID)+".cookie", []byte(req.Header.Get("Cookie")), os.ModePerm))
}

func NewCookieJar(uid int) (*CookieJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	k := &CookieJar{CookieJar: jar, UID: uid}
	b, err := os.ReadFile(strconv.Itoa(uid) + ".cookie")
	if err != nil {
		return nil, err
	}
	cookies, err := http.ParseCookie(string(b))
	if err != nil {
		return nil, err
	}
	jar.SetCookies(Session.BaseURL, cookies)
	return k, nil
}
