package main

import (
	"flag"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Drelf2018/exp/drink/bilibili"
	"github.com/Drelf2018/exp/drink/db"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/cookie"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
)

var logger = hook.New(logrus.InfoLevel, hook.NewDailyFileHook("logs/2006-01-02.log"), hook.NewDingTalkHook(hook.Saki))
var saki = logger.WithField(hook.DingTalk, hook.Saki.Name)

// bilibili Cookie 保活
type CookieJar struct {
	http.CookieJar

	// 所属用户 UID
	UID int
}

func (*CookieJar) OnError(err error) {
	if err != nil {
		saki.Error(err)
	}
}

func (c *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.CookieJar.SetCookies(u, cookies)
	err := db.UpdateUserCookies(c.UID, cookies)
	if err != nil {
		logger.Error(err)
	}
}

var (
	uid     int
	drinks  string
	cookies string
)

func init() {
	flag.IntVar(&uid, "uid", 0, "uid")
	flag.StringVar(&drinks, "drinks", "drinks.json", "drinks")
	flag.StringVar(&cookies, "cookies", "cookies.db", "cookies")
	flag.Parse()
}

func main() {
	// 打开数据库，写入饮品
	err := db.Open(cookies)
	if err != nil {
		logger.Panic(err)
	}
	if drinks != "" {
		err = db.CreateDrinks(drinks)
		if err != nil {
			logger.Error(err)
		}
	}
	// 初始化 Cookie
	refresher := &bilibili.Refresher{}
	jar := &CookieJar{CookieJar: &bilibili.Credential{}, UID: uid}
	// 如果 UID 未给入则扫码获取
	if uid == 0 {
		generate, err := bilibili.GetGenerate()
		if err != nil {
			logger.Panic(err)
		}
		logger.Info(generate.Data.URL)
		err = qrcode.WriteFile(generate.Data.URL, qrcode.Low, 200, "qrcode.jpg")
		if err != nil {
			logger.Panic(err)
		}
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			// 先写入内部
			result, err := bilibili.GetPoll(generate.Data.QRCodeKey, jar.CookieJar)
			if err != nil {
				logger.Error(err)
				continue
			}
			// 再提取出 UID
			cookies := jar.Cookies(bilibili.Session.BaseURL)
			for _, cookie := range cookies {
				if cookie.Name == "DedeUserID" {
					jar.UID, err = strconv.Atoi(cookie.Value)
					if err != nil {
						logger.Panic(err)
					}
					break
				}
			}
			// 再从外部重新设置一次，触发数据库写入
			refresher.RefreshToken = result.Data.RefreshToken
			jar.SetCookies(bilibili.Session.BaseURL, append(cookies, &http.Cookie{Name: "refresh_token", Value: refresher.RefreshToken}))
			break
		}
		ticker.Stop()
	} else {
		cookies, err := db.GetUserCookies(uid)
		if err != nil {
			logger.Panic(err)
		}
		for _, cookie := range cookies {
			if cookie.Name == "refresh_token" {
				refresher.RefreshToken = cookie.Value
				cookie.Name = ""
				break
			}
		}
		jar.CookieJar.SetCookies(bilibili.Session.BaseURL, cookies)
	}

	// 每天保活一次
	cookie.Keepalive(jar, refresher, 24*time.Hour)
	// 随机轮询计时器
	ticker := req.NewTicker(req.RandomTicker{4 * time.Minute, 6 * time.Minute})
	defer ticker.Stop()
	// 开始轮询
	for {
		for range ticker.C {
			r, err := bilibili.GetRoomInfo(21452505)
			if err != nil {
				logger.Error(err)
				continue
			}
			if r.Data.LiveTime == "0000-00-00 00:00:00" {
				logger.Debug("未开播")
				continue
			}
			drink, err := db.GetRandomDrink()
			if err != nil {
				logger.Error(err)
				continue
			}
			_, err = bilibili.PostSuperchat(21452505, 300, drink.Value, jar)
			if err != nil {
				logger.Error(err)
				continue
			}
			saki.Info(drink.Value)
			err = db.DrinkUp(drink.Name)
			if err != nil {
				logger.Error(err)
			}
			break
		}
		time.Sleep(47 * time.Hour)
	}
}
