package model

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	schema.RegisterSerializer("url", URLSerializer)
}

// Cookie 是用于 GORM 存取的 Cookie 模型
type Cookie struct {
	Name        string
	Value       string
	Quoted      bool
	CreatedAt   time.Time
	CookieJarID uint64
}

// From 将 *http.Cookie 转换成 GORM 模型
func (c *Cookie) From(cookie *http.Cookie) {
	c.Name = cookie.Name
	c.Value = cookie.Value
	c.Quoted = cookie.Quoted
}

// To 将 GORM 模型转换成 *http.Cookie
func (c *Cookie) To() *http.Cookie {
	return &http.Cookie{Name: c.Name, Value: c.Value, Quoted: c.Quoted}
}

// CookieJar 是用于 GORM 存取 http.CookieJar 的模型
type CookieJar struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	URL       *url.URL       `gorm:"serializer:url"`
	Jar       http.CookieJar `gorm:"-"`
	Cookies   []Cookie
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// ToHTTPCookies 将 GORM 模型切片转换成 *http.Cookie 切片
func ToHTTPCookies(models []Cookie) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(models))
	for _, m := range models {
		cookies = append(cookies, m.To())
	}
	return cookies
}

// WriteTo 将刷新器中的 Cookies 写入 http.CookieJar
func (c *CookieJar) WriteTo(jar http.CookieJar) {
	if jar != nil {
		jar.SetCookies(c.URL, ToHTTPCookies(c.Cookies))
	}
}

// AfterFind 在查询后将数据写入 http.CookieJar
func (c *CookieJar) AfterFind(*gorm.DB) error {
	if c.Jar == nil {
		var err error
		c.Jar, err = cookiejar.New(nil)
		if err != nil {
			return err
		}
	}
	c.WriteTo(c.Jar)
	return nil
}

// FromHTTPCookies 将 *http.Cookie 切片转换成 GORM 模型切片
func FromHTTPCookies(cookies []*http.Cookie) []Cookie {
	models := make([]Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" {
			models = append(models, Cookie{Name: cookie.Name, Value: cookie.Value, Quoted: cookie.Quoted})
		}
	}
	return models
}

// ReadFrom 从 http.CookieJar 中读取 Cookies 到刷新器
func (c *CookieJar) ReadFrom(jar http.CookieJar) {
	if jar != nil {
		for _, cookie := range jar.Cookies(c.URL) {
			if cookie.Name != "" {
				c.Cookies = append(c.Cookies, Cookie{Name: cookie.Name, Value: cookie.Value, Quoted: cookie.Quoted})
			}
		}
	}
}

// BeforeSave 在保存前读取 http.CookieJar 中数据
func (c *CookieJar) BeforeSave(*gorm.DB) error {
	if c.Jar != nil {
		c.ReadFrom(c.Jar)
	}
	return nil
}
