package bilibili

import "fmt"

type Base struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TTL     int    `json:"ttl"`
}

func (b Base) Unwrap() error {
	if b.Code == 0 {
		return nil
	}
	return fmt.Errorf("bilibili: %s (%d)", b.Message, b.Code)
}

type RoomInfoResponse struct {
	Base
	Msg  string `json:"msg"` // "ok"
	Data struct {
		UID              int      `json:"uid"`         // 434334701
		RoomID           int      `json:"room_id"`     // 21452505
		ShortID          int      `json:"short_id"`    // 0
		Attention        int      `json:"attention"`   // 1057100
		Online           int      `json:"online"`      // 0
		IsPortrait       bool     `json:"is_portrait"` // false
		Description      string   `json:"description"`
		LiveStatus       int      `json:"live_status"`        // 0
		AreaID           int      `json:"area_id"`            // 745
		ParentAreaID     int      `json:"parent_area_id"`     // 9
		ParentAreaName   string   `json:"parent_area_name"`   // "虚拟主播"
		OldAreaID        int      `json:"old_area_id"`        // 6
		Background       string   `json:"background"`         // "https://i0.hdslb.com/bfs/live/636d66a97d5f55099a9d8d6813558d6d4c95fd61.jpg"
		Title            string   `json:"title"`              // "看看大家的24年最值购物"
		UserCover        string   `json:"user_cover"`         // "https://i0.hdslb.com/bfs/live/new_room_cover/ef9375fd23aefb03e3c2fd48934fa51a30caacbc.jpg"
		Keyframe         string   `json:"keyframe"`           // ""
		IsStrictRoom     bool     `json:"is_strict_room"`     // false
		LiveTime         string   `json:"live_time"`          // "0000-00-00 00:00:00"
		Tags             string   `json:"tags"`               // "七海,海子姐,VirtuaReal"
		IsAnchor         int      `json:"is_anchor"`          // 0
		RoomSilentType   string   `json:"room_silent_type"`   // ""
		RoomSilentLevel  int      `json:"room_silent_level"`  // 0
		RoomSilentSecond int      `json:"room_silent_second"` // 0
		AreaName         string   `json:"area_name"`          // "虚拟Gamer"
		Pendants         string   `json:"pendants"`           // ""
		AreaPendants     string   `json:"area_pendants"`      // ""
		HotWords         []string `json:"hot_words"`
		HotWordsStatus   int      `json:"hot_words_status"` // 0
		Verify           string   `json:"verify"`           // ""
		NewPendants      struct {
			Frame struct {
				Name       string `json:"name"`         // "长红计划-Topstar"
				Value      string `json:"value"`        // "https://i0.hdslb.com/bfs/live/62e62f657d379aaaec2bbd4a6a16a938bcba76e6.png"
				Position   int    `json:"position"`     // 0
				Desc       string `json:"desc"`         // ""
				Area       int    `json:"area"`         // 0
				AreaOld    int    `json:"area_old"`     // 0
				BgColor    string `json:"bg_color"`     // ""
				BgPic      string `json:"bg_pic"`       // ""
				UseOldArea bool   `json:"use_old_area"` // false
			} `json:"frame"`
			Badge struct {
				Name     string `json:"name"`     // "v_person"
				Position int    `json:"position"` // 3
				Value    string `json:"value"`    // ""
				Desc     string `json:"desc"`     // "虚拟UP主、bilibili直播高能主播"
			} `json:"badge"`
			MobileFrame struct {
				Name       string `json:"name"`         // "长红计划-Topstar"
				Value      string `json:"value"`        // "https://i0.hdslb.com/bfs/live/62e62f657d379aaaec2bbd4a6a16a938bcba76e6.png"
				Position   int    `json:"position"`     // 0
				Desc       string `json:"desc"`         // ""
				Area       int    `json:"area"`         // 0
				AreaOld    int    `json:"area_old"`     // 0
				BgColor    string `json:"bg_color"`     // ""
				BgPic      string `json:"bg_pic"`       // ""
				UseOldArea bool   `json:"use_old_area"` // false
			} `json:"mobile_frame"`
			MobileBadge any `json:"mobile_badge"`
		} `json:"new_pendants"`
		UpSession            string `json:"up_session"`              // ""
		PkStatus             int    `json:"pk_status"`               // 0
		PkID                 int    `json:"pk_id"`                   // 0
		BattleID             int    `json:"battle_id"`               // 0
		AllowChangeAreaTime  int    `json:"allow_change_area_time"`  // 0
		AllowUploadCoverTime int    `json:"allow_upload_cover_time"` // 0
		StudioInfo           struct {
			Status     int   `json:"status"` // 0
			MasterList []any `json:"master_list"`
		} `json:"studio_info"`
	} `json:"data"`
}

type CreateOrderResponse struct {
	Base
	Data struct {
		Bp        int    `json:"bp"`
		ErrorInfo any    `json:"error_info"`
		Gold      int    `json:"gold"`
		OrderID   string `json:"order_id"`
		Status    int    `json:"status"`
	} `json:"data"`
}

type NavResponse struct {
	Base
	Data struct {
		IsLogin bool `json:"isLogin"` // false
		WbiImg  struct {
			ImgURL string `json:"img_url"` // https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png
			SubURL string `json:"sub_url"` // https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png
		} `json:"wbi_img"`
	} `json:"data"`
}

type CookieInfoResponse struct {
	Base
	Data struct {
		Refresh   bool  `json:"refresh"`   // true
		Timestamp int64 `json:"timestamp"` // 1734963138171
	} `json:"data"`
}

type CookieRefreshResponse struct {
	Base
	Data struct {
		Status       int    `json:"status"`        // 0
		Message      string `json:"message"`       // ""
		RefreshToken string `json:"refresh_token"` // "xxx"
	} `json:"data"`
}

type ConfirmRefreshResponse struct {
	Base
}
