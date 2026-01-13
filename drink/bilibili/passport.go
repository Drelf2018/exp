package bilibili

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Drelf2018/req"

	_ "embed"
)

var Passport, _ = req.NewSession(
	req.SessionURL("https://passport.bilibili.com/"),
	req.SessionHeaders{
		"User-Agent": req.UserAgent,
	},
)

// CookieInfo 检查是否需要刷新
type CookieInfo struct {
	req.Get
	http.CookieJar
}

func (CookieInfo) RawURL() string {
	return "/x/passport-login/web/cookie/info"
}

// Correspond 获取 refresh_csrf
type Correspond struct {
	req.Get
	http.CookieJar

	// 通过 GetCorrespondPath 获取
	CorrespondPath string
}

func (c Correspond) RawURL() string {
	return fmt.Sprintf("https://www.bilibili.com/correspond/1/%s", c.CorrespondPath)
}

// CookieRefresh 刷新 Cookie
type CookieRefresh struct {
	PostCSRF
	http.CookieJar

	// 实时刷新口令
	// 通过 GetRefreshCSRF 获得
	RefreshCSRF string `req:"body:refresh_csrf"`

	// 访问来源 默认值 main_web
	Source string `req:"body" default:"main_web"`

	// 持久化刷新口令
	RefreshToken string `req:"body"`
}

func (CookieRefresh) RawURL() string {
	return "/x/passport-login/web/cookie/refresh"
}

// ConfirmRefresh 确认更新
type ConfirmRefresh struct {
	PostCSRF
	http.CookieJar

	// 旧的持久化刷新口令
	RefreshToken string `req:"body"`
}

func (ConfirmRefresh) RawURL() string {
	return "/x/passport-login/web/confirm/refresh"
}

// GetCookieInfo 检查是否需要刷新
func GetCookieInfo(ctx context.Context, jar http.CookieJar) (r CookieInfoResponse, err error) {
	err = Passport.ResultWithContext(ctx, CookieInfo{CookieJar: jar}, &r)
	return
}

//go:embed pubkey.pem
var PublicKeyPEM []byte

// GetCorrespondPath 生成 CorrespondPath
//
// ts 为当前毫秒级时间戳
//
// 代码由 https://socialsisteryi.github.io/bilibili-API-collect/docs/login/cookie_refresh.html 提供
func GetCorrespondPath(ts int64) (string, error) {
	pubKeyBlock, _ := pem.Decode(PublicKeyPEM)

	pubInterface, parseErr := x509.ParsePKIXPublicKey(pubKeyBlock.Bytes)
	if parseErr != nil {
		return "", parseErr
	}
	pub := pubInterface.(*rsa.PublicKey)

	encryptedData, encryptErr := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, []byte(fmt.Sprintf("refresh_%d", ts)), nil)
	if encryptErr != nil {
		return "", encryptErr
	}
	return hex.EncodeToString(encryptedData), nil
}

var refreshCSRFPattern = regexp.MustCompile(`<div id="1-name">(.*?)</div>`)

var ErrRefreshCSRFNotExist = errors.New("bilibili: refresh_csrf does not exist")

// GetRefreshCSRF 获取 refresh_csrf
func GetRefreshCSRF(ctx context.Context, jar http.CookieJar) (string, error) {
	path, err := GetCorrespondPath(time.Now().UnixMilli())
	if err != nil {
		return "", err
	}
	s, err := Passport.TextWithContext(ctx, Correspond{CookieJar: jar, CorrespondPath: path})
	if err != nil {
		return "", fmt.Errorf("bilibili.GetRefreshCSRF: %w", err)
	}
	r := refreshCSRFPattern.FindStringSubmatch(s)
	if len(r) < 2 {
		return "", ErrRefreshCSRFNotExist
	}
	return r[1], nil
}

// PostCookieRefresh 刷新 Cookie
func PostCookieRefresh(ctx context.Context, refreshToken string, jar http.CookieJar) (r CookieRefreshResponse, err error) {
	key, err := GetRefreshCSRF(ctx, jar)
	if err != nil {
		return
	}
	err = Passport.ResultWithContext(ctx, CookieRefresh{
		CookieJar:    jar,
		RefreshCSRF:  key,
		RefreshToken: refreshToken,
	}, &r)
	if err != nil {
		err = fmt.Errorf("bilibili.PostCookieRefresh: %w", err)
	}
	return
}

// PostConfirmRefresh 确认更新
func PostConfirmRefresh(ctx context.Context, refreshToken string, jar http.CookieJar) (string, error) {
	r, err := PostCookieRefresh(ctx, refreshToken, jar)
	if err != nil {
		return "", err
	}
	err = Passport.ResultWithContext(ctx, ConfirmRefresh{
		CookieJar:    jar,
		RefreshToken: refreshToken,
	}, &ConfirmRefreshResponse{})
	if err != nil {
		return "", fmt.Errorf("bilibili.PostConfirmRefresh: %w", err)
	}
	return r.Data.RefreshToken, nil
}

// 申请二维码
type Generate struct {
	req.Get
}

func (Generate) RawURL() string {
	return "/x/passport-login/web/qrcode/generate"
}

type GenerateResponse struct {
	Base
	Data struct {
		URL       string `json:"url"`        // https://account.bilibili.com/h5/account-h5/auth/scan-web?navhide=1&callback=close&qrcode_key=52360fad71935c52ca33f4f24fd18e07&from=
		QRCodeKey string `json:"qrcode_key"` // 52360fad71935c52ca33f4f24fd18e07
	} `json:"data"`
}

// 申请二维码
func GetGenerate() (result GenerateResponse, err error) {
	err = Passport.Result(Generate{}, &result)
	return
}

// 扫码登录
//
// 登录成功后会自动将 Cookie 值写入该结构体的 http.CookieJar 字段中
type Poll struct {
	req.Get
	http.CookieJar

	// 先前生成的二维码密钥
	QRCodeKey string `req:"query:qrcode_key"`
}

func (Poll) RawURL() string {
	return "/x/passport-login/web/qrcode/poll"
}

type PollResponse struct {
	Base
	Data struct {
		Base
		URL          string `json:"url"`
		RefreshToken string `json:"refresh_token"`
		Timestamp    int    `json:"timestamp"`
	} `json:"data"`
}

func (r PollResponse) Unwrap() error {
	err := r.Base.Unwrap()
	if err != nil {
		return err
	}
	return r.Data.Base.Unwrap()
}

// 登录成功后会自动将 Cookie 值写入 credential 变量中
func GetPoll(qrcodeKey string, jar http.CookieJar) (result PollResponse, err error) {
	err = Passport.Result(Poll{QRCodeKey: qrcodeKey, CookieJar: jar}, &result)
	return
}
