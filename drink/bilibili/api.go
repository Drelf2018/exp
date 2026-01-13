package bilibili

import (
	"net/http"

	"github.com/Drelf2018/req"
)

// RoomInfo 获取直播间信息
type RoomInfo struct {
	req.Get

	// 直播间号	可以为短号
	RoomID int `req:"query"`
}

func (RoomInfo) RawURL() string {
	return "/room/v1/Room/get_info"
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
	PostCSRF
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
	return "/xlive/revenue/v1/order/createOrder"
}

// Nav 导航栏用户信息
type Nav struct {
	req.Get
	http.CookieJar
}

func (Nav) RawURL() string {
	return "https://api.bilibili.com/x/web-interface/nav"
}
