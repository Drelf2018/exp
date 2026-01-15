package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/cookie"
	"github.com/playwright-community/playwright-go"
)

var browser playwright.Browser

// Refresher 使用 playwright 访问微博主页进行刷新
var Refresher cookie.Refresher = cookie.ForcedRefresher(func(ctx context.Context, jar http.CookieJar) error {
	// 创建浏览器上下文
	logger.Debug("创建浏览器上下文")
	browserContext, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("could not create context: %w", err)
	}
	defer browserContext.Close()
	// 添加 Cookie
	logger.Debug("添加 Cookie")
	cookies := make([]playwright.OptionalCookie, 0)
	for _, cookie := range jar.Cookies(session.BaseURL) {
		cookies = append(cookies, playwright.OptionalCookie{
			Name:   cookie.Name,
			Value:  cookie.Value,
			Domain: playwright.String("." + session.BaseURL.Host),
			Path:   playwright.String("/"),
		})
	}
	err = browserContext.AddCookies(cookies)
	if err != nil {
		return fmt.Errorf("could not add cookies: %w", err)
	}
	// 新建页面
	logger.Debug("新建页面")
	page, err := browserContext.NewPage()
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
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for cookie timeout: %w", ctx.Err())
		case <-ticker.C:
			logger.Debug("开始获取 Cookie")
			pwCookies, err := browserContext.Cookies("https://weibo.com/u/7198559139")
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
			jar.SetCookies(session.BaseURL, cookies)
			_, err = GetMymlog(ctx, 7198559139, jar)
			if err != nil {
				return fmt.Errorf("invalid cookie: %w", err)
			}
			img, err := page.Screenshot()
			if err != nil {
				saki.Errorln("cannot screenshot:", err)
				return nil
			}
			api := Upload{File: bytes.NewReader(img)}
			uuid, err := req.JSONWithContext(ctx, api)
			if err != nil {
				saki.Errorln("cannot upload screenshot:", err)
				return nil
			}
			saki.Infof("![刷新成功](%s/%s)", api.RawURL(), uuid)
			return nil
		}
	}
})

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
	for _, cookie := range c.CookieJar.Cookies(session.BaseURL) {
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
	jar.SetCookies(session.BaseURL, cookies)
	return k, nil
}
