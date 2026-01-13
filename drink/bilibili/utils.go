package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/Drelf2018/req"
)

var Session, _ = req.NewSession(
	req.SessionURL("https://api.live.bilibili.com/"),
	req.SessionHeaders{
		"User-Agent": req.UserAgent,
	},
)

// GetRoomInfo 获取直播间信息
func GetRoomInfo(roomid int) (r RoomInfoResponse, err error) {
	err = Session.Result(RoomInfo{RoomID: roomid}, &r)
	return
}

// 用户房间号与 UID 映射表 map[int]int
var uidCache sync.Map

// GetUIDByRoomID 通过房间号获取用户 UID
func GetUIDByRoomID(roomid int) (int, error) {
	value, ok := uidCache.Load(roomid)
	if ok {
		return value.(int), nil
	}
	info, err := GetRoomInfo(roomid)
	if err != nil {
		return 0, err
	}
	uidCache.Store(roomid, info.Data.UID)
	return info.Data.UID, nil
}

// PostSuperchat 发送醒目留言
func PostSuperchat(roomid int, battery int, msg string, jar http.CookieJar) (r CreateOrderResponse, err error) {
	if battery%10 != 0 {
		err = fmt.Errorf("bilibili: battery count is not divisible by ten: %d", battery)
		return
	}
	if battery < 300 {
		err = fmt.Errorf("bilibili: minimum payment: 300 batteries, got: %d", battery)
		return
	}
	uid, err := GetUIDByRoomID(roomid)
	if err != nil {
		err = fmt.Errorf("bilibili: get uid error: %w", err)
		return
	}
	api := &CreateOrder{CookieJar: jar, ContextID: roomid, Ruid: uid, PayGold: battery * 100}
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
	err = Session.Result(api, &r)
	return
}

// GetNav 获取导航栏用户信息
func GetNav(ctx context.Context, jar http.CookieJar) (r NavResponse, err error) {
	err = Session.ResultWithContext(ctx, Nav{CookieJar: jar}, &r)
	return
}
