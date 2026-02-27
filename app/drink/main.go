package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/hook"
	bilibili "github.com/Drelf2018/go-bilibili-api"
	"github.com/Drelf2018/go-bilibili-api/cookie"
	"github.com/Drelf2018/req"
	"github.com/glebarez/sqlite"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Options struct {
	Sleep    int           `long:"sleep" description:"休眠小时"`
	Drinks   string        `long:"drinks" description:"饮料文件路径"`
	Logger   string        `long:"logger" description:"日志文件路径"`
	QRCode   string        `long:"qrcode" description:"扫码登录文件路径"`
	Database string        `long:"database" description:"数据库文件路径"`
	DingTalk *dingtalk.Bot `group:"DingTalk" description:"钉钉机器人"`
}

type Drink struct {
	Name      string `gorm:"primarykey"`
	Value     string
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

var (
	options Options
	logger  *logrus.Logger
	bot     *logrus.Entry
	jar     http.CookieJar
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
	// 初始化日志
	ding := hook.NewDingTalkHook(options.DingTalk)
	logger = hook.New(logrus.InfoLevel, hook.NewDailyFileHook(options.Logger), ding)
	bot = ding.Bind(logger)
}

// 初始化数据库
func init() {
	var err error
	logger.Info("初始化数据库")
	db, err = gorm.Open(sqlite.Open(options.Database))
	if err != nil {
		logger.Panicln("创建数据库失败:", err)
	}
	err = db.AutoMigrate(&cookie.Refresher{}, &cookie.Cookie{}, &Drink{})
	if err != nil {
		logger.Panicln("自动迁移数据库失败:", err)
	}
	// 打开数据库，写入饮品
	if options.Drinks == "" {
		return
	}
	logger.Info("初始化饮料")
	data, err := os.ReadFile(options.Drinks)
	if err != nil {
		logger.Panicln("读取饮料失败:", err)
	}
	var drinksSlice []string
	err = json.Unmarshal(data, &drinksSlice)
	if err != nil {
		logger.Panicln("反序列化饮料失败:", err)
	}
	drinks := make([]*Drink, 0, len(drinksSlice))
	for _, d := range drinksSlice {
		name, _, _ := strings.Cut(strings.TrimPrefix(d, "我今天喝了"), "，")
		drinks = append(drinks, &Drink{Name: name, Value: d})
	}
	err = db.Clauses(clause.OnConflict{DoNothing: true}).Create(drinks).Error
	if err != nil {
		logger.Panicln("写入饮料失败:", err)
	}
}

// 扫码登录
func init() {
	if options.QRCode != "" {
		logger.Info("扫码登录")
		generate, err := bilibili.GetGenerate(context.Background())
		if err != nil {
			logger.Panicln("申请二维码失败:", err)
		}
		logger.Info(generate.Data.URL)
		err = qrcode.WriteFile(generate.Data.URL, qrcode.Low, 200, options.QRCode)
		if err != nil {
			logger.Panicln("导出二维码失败:", err)
		}
		var cred *bilibili.Credential
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cred, err = bilibili.GetCredential(context.Background(), generate.Data.QRCodeKey)
			if err == nil {
				break
			}
			logger.Errorln("扫码登录失败:", err)
		}
		err = db.Save(&cookie.Refresher{Token: cred.RefreshToken, Jar: cred}).Error
		if err != nil {
			logger.Panicln("凭据写入数据库失败:", err)
		}
	}
}

// 初始化 Cookie
func init() {
	logger.Info("初始化 Cookie")
	r := &cookie.Refresher{}
	err := db.Preload("Cookies").Order("id DESC").First(r).Error
	if err != nil {
		logger.Panicln("从数据库读取凭据失败:", err)
	}
	jar = r.Jar
}

type BizExtra struct {
	// 留言内容
	Msg string `json:"msg"`

	// 留言级别
	Level int `json:"level"`

	// 礼物序号，与级别有对应关系 (1, 8) / (2, 14) / (3, 3) / (4, 12) / (5, 5) / (6, 6)
	BizID int `json:"biz_id"`

	TransKey  string `json:"trans_key"`
	ScImageID int    `json:"sc_image_id"`
}

// CreateOrder 创建订单
type CreateOrder struct {
	bilibili.PostCSRF
	http.CookieJar

	// 房间号
	ContextID int `req:"body"`

	// 主播 UID
	Ruid int `req:"body"`

	// 100 倍电池
	PayGold int `req:"body"`

	BizExtra    BizExtra `req:"body"`
	ContextType int      `req:"body" default:"1"`
	GoodsID     int      `req:"body" default:"12"`
	GoodsNum    int      `req:"body" default:"1"`
	Platform    string   `req:"body" default:"pc"`
}

func (CreateOrder) RawURL() string {
	return "https://api.live.bilibili.com/xlive/revenue/v1/order/createOrder"
}

type CreateOrderResponse struct {
	bilibili.Error
	Data struct {
		Bp        int    `json:"bp"`
		ErrorInfo any    `json:"error_info"`
		Gold      int    `json:"gold"`
		OrderID   string `json:"order_id"`
		Status    int    `json:"status"`
	} `json:"data"`
}

// PostSuperchat 发送醒目留言
func PostSuperchat(ctx context.Context, jar http.CookieJar, roomid int, battery int, msg string) (CreateOrderResponse, error) {
	if battery%10 != 0 {
		return CreateOrderResponse{}, fmt.Errorf("bilibili: battery count is not divisible by ten: %d", battery)
	}
	if battery < 300 {
		return CreateOrderResponse{}, fmt.Errorf("bilibili: minimum payment: 300 batteries, got: %d", battery)
	}
	info, err := bilibili.GetRoomInfo(ctx, roomid)
	if err != nil {
		return CreateOrderResponse{}, fmt.Errorf("bilibili: get uid error: %w", err)
	}
	api := &CreateOrder{CookieJar: jar, ContextID: roomid, Ruid: info.Data.UID, PayGold: battery * 100}
	if battery >= 20000 {
		api.BizExtra = BizExtra{Msg: msg, Level: 6, BizID: 6}
	} else if battery >= 10000 {
		api.BizExtra = BizExtra{Msg: msg, Level: 5, BizID: 5}
	} else if battery >= 5000 {
		api.BizExtra = BizExtra{Msg: msg, Level: 4, BizID: 12}
	} else if battery >= 1000 {
		api.BizExtra = BizExtra{Msg: msg, Level: 3, BizID: 3}
	} else if battery >= 500 {
		api.BizExtra = BizExtra{Msg: msg, Level: 2, BizID: 14}
	} else {
		api.BizExtra = BizExtra{Msg: msg, Level: 1, BizID: 8}
	}
	return bilibili.Do[CreateOrderResponse](ctx, api)
}

type MyGoldWallet struct {
	req.Get
	http.CookieJar
	NeedBp           int    `req:"query" default:"1"`
	NeedMetal        int    `req:"query" default:"1"`
	Platform         string `req:"query" default:"pc"`
	BpWithDecimal    int    `req:"query" default:"0"`
	IosBpAffordParty int    `req:"query" default:"0"`
}

func (MyGoldWallet) RawURL() string {
	return "https://api.live.bilibili.com/xlive/revenue/v1/wallet/myGoldWallet"
}

type MyGoldWalletResponse struct {
	bilibili.Error
	Data struct {
		Gold int `json:"gold"`
	} `json:"data"`
}

// GetMyGoldWallet 获取我的钱包
func GetMyGoldWallet(ctx context.Context, jar http.CookieJar) (MyGoldWalletResponse, error) {
	return bilibili.Do[MyGoldWalletResponse](ctx, MyGoldWallet{CookieJar: jar})
}

func main() {
	logger.Info("开始轮询")
	// 随机轮询计时器
	ticker := req.NewTicker(req.RandomTicker{4 * time.Minute, 6 * time.Minute})
	defer ticker.Stop()
	for {
		for range ticker.C {
			r, err := bilibili.GetRoomInfo(context.Background(), 21452505)
			if err != nil {
				bot.WithField("title", "获取直播失败").Error(err)
				continue
			}
			if r.Data.LiveTime == "0000-00-00 00:00:00" {
				logger.Debug("未开播")
				continue
			}
			var drink Drink
			err = db.Order("RANDOM()").First(&drink).Error
			if err != nil {
				bot.WithField("title", "读取饮料失败").Error(err)
				continue
			}
			_, err = PostSuperchat(context.Background(), jar, 21452505, 300, drink.Value)
			if err != nil {
				bot.WithField("title", "发送留言失败").Error(err)
				continue
			}
			wallet, err := GetMyGoldWallet(context.Background(), jar)
			if err == nil {
				drink.Value = fmt.Sprintf("%s\n（还可发送 %d 条）", drink.Value, wallet.Data.Gold/30000)
			}
			bot.WithField("title", "发送留言成功").Info(drink.Value)
			err = db.Delete(&Drink{}, "name = ?", drink.Name).Error
			if err != nil {
				bot.WithField("title", "移除饮料失败").Error(err)
			}
			break
		}
		time.Sleep(time.Duration(options.Sleep) * time.Hour)
	}
}
