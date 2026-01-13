package bilibili

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Drelf2018/req/cookie"
)

// Credential 凭据
type Credential struct {
	// 登录 Token
	SESSDATA string `cookie:"SESSDATA"`

	// CSRF Token
	BiliJct string `cookie:"bili_jct"`

	// 设备信息
	Buvid3 string `cookie:"buvid3"`

	// 数字型用户 UID
	DedeUserID string `cookie:"DedeUserID"`

	// 字符型用户 UID
	DedeUserIDckMd5 string `cookie:"DedeUserID__ckMd5"`
}

func (c *Credential) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if c != nil {
		cookie.Set(c, cookies)
	}
}

func (c *Credential) Cookies(u *url.URL) (cookies []*http.Cookie) {
	if c != nil {
		cookies, _ = cookie.Get(c)
	}
	return
}

var _ http.CookieJar = (*Credential)(nil)

// Refresher 用保存在浏览器本地储存 ac_time_value 中的口令实现持久化刷新
type Refresher struct {
	RefreshToken string
}

func (*Refresher) IsValid(ctx context.Context, jar http.CookieJar) (bool, error) {
	r, err := GetNav(ctx, jar)
	if err != nil || r.Code != 0 {
		return false, err
	}
	info, err := GetCookieInfo(ctx, jar)
	return !info.Data.Refresh, err
}

func (r *Refresher) Refresh(ctx context.Context, jar http.CookieJar) (err error) {
	r.RefreshToken, err = PostConfirmRefresh(ctx, r.RefreshToken, jar)
	if err == nil {
		jar.SetCookies(Session.BaseURL, []*http.Cookie{{Name: "refresh_token", Value: r.RefreshToken}})
	}
	return
}

var _ cookie.Refresher = (*Refresher)(nil)
