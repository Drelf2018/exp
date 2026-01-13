package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/method"
)

var Session, _ = req.NewSession(
	req.SessionURL("https://weibo.com/"),
	req.SessionHeaders{
		"Referer":          "https://weibo.com/",
		"User-Agent":       req.UserAgent,
		"X-Requested-With": "XMLHttpRequest",
	},
)

// CSRF 为请求头添加 X-Xsrf-Token
type CSRF struct{}

func (CSRF) XSRF() (string, string) {
	return "XSRF-TOKEN", "X-Xsrf-Token"
}

var _ method.APIXSRF = CSRF{}

// ProfileInfo 获取博主信息
type ProfileInfo struct {
	CSRF
	req.Get
	http.CookieJar

	// 博主标识符
	UID string `req:"query"`
}

func (ProfileInfo) RawURL() string {
	return "/ajax/profile/info"
}

var _ req.API = ProfileInfo{}

type ProfileInfoResponse struct {
	Ok   int `json:"ok"`
	Data struct {
		User struct {
			// 	ID                 int64  `json:"id"`
			// 	Idstr              string `json:"idstr"`
			// 	PcNew              int    `json:"pc_new"`
			// 	ScreenName         string `json:"screen_name"`
			// 	ProfileImageURL    string `json:"profile_image_url"`
			// 	ProfileURL         string `json:"profile_url"`
			// 	Verified           bool   `json:"verified"`
			// 	VerifiedType       int    `json:"verified_type"`
			// 	Domain             string `json:"domain"`
			// 	Weihao             string `json:"weihao"`
			// 	VerifiedTypeExt    int    `json:"verified_type_ext"`
			// 	StatusTotalCounter struct {
			// 		TotalCntFormat any    `json:"total_cnt_format"`
			// 		CommentCnt     string `json:"comment_cnt"`
			// 		RepostCnt      string `json:"repost_cnt"`
			// 		LikeCnt        string `json:"like_cnt"`
			// 		TotalCnt       string `json:"total_cnt"`
			// 	} `json:"status_total_counter"`
			// 	Remark            string `json:"remark"`
			// 	AvatarLarge       string `json:"avatar_large"`
			// 	AvatarHd          string `json:"avatar_hd"`
			// 	FollowMe          bool   `json:"follow_me"`
			// 	Following         bool   `json:"following"`
			// 	Mbrank            int    `json:"mbrank"`
			// 	Mbtype            int    `json:"mbtype"`
			// 	VPlus             int    `json:"v_plus"`
			// 	UserAbility       int    `json:"user_ability"`
			// 	PlanetVideo       bool   `json:"planet_video"`
			// 	VerifiedReason    string `json:"verified_reason"`
			Description string `json:"description"`
			// 	Location          string `json:"location"`
			// 	Gender            string `json:"gender"`
			// 	FollowersCount    int    `json:"followers_count"`
			FollowersCountStr string `json:"followers_count_str"`
			FriendsCount      int    `json:"friends_count"`
			// 	StatusesCount     int    `json:"statuses_count"`
			// 	URL               string `json:"url"`
			// 	Svip              int    `json:"svip"`
			// 	Vvip              int    `json:"vvip"`
			CoverImagePhone string `json:"cover_image_phone"`
			// 	IconList          []struct {
			// 		Type string `json:"type"`
			// 		Data struct {
			// 			Mbrank int `json:"mbrank"`
			// 			Mbtype int `json:"mbtype"`
			// 			Svip   int `json:"svip"`
			// 			Vvip   int `json:"vvip"`
			// 		} `json:"data"`
			// 	} `json:"icon_list"`
			// 	TopUser       int    `json:"top_user"`
			// 	UserType      int    `json:"user_type"`
			// 	IsStar        string `json:"is_star"`
			// 	IsMuteuser    bool   `json:"is_muteuser"`
			// 	SpecialFollow bool   `json:"special_follow"`
		} `json:"user"`
		// TabList []struct {
		// 	Name    string `json:"name"`
		// 	TabName string `json:"tabName"`
		// } `json:"tabList"`
		// BlockText string `json:"blockText"`
	} `json:"data"`
}

func (r ProfileInfoResponse) Unwrap() error {
	if r.Ok != 1 {
		return fmt.Errorf("failed to get profile info: %d", r.Ok)
	}
	return nil
}

var _ req.Unwrap = (*ProfileInfoResponse)(nil)

func GetProfileInfo(ctx context.Context, uid string, jar http.CookieJar) (r ProfileInfoResponse, err error) {
	err = Session.ResultWithContext(ctx, ProfileInfo{UID: uid, CookieJar: jar}, &r)
	return
}

// Mymlog 获取博文
type Mymlog struct {
	CSRF
	req.Get
	http.CookieJar

	// 博主标识符
	UID int `req:"query"`

	// 查询页数，默认第 1 页
	Page int `req:"query" default:"1"`

	// 未知参数
	Feature int `req:"query" default:"0"`
}

func (Mymlog) RawURL() string {
	return "/ajax/statuses/mymblog"
}

var _ req.API = Mymlog{}

type PicInfo struct {
	URL     string `json:"url"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	CutType int    `json:"cut_type"`
	Type    string `json:"type"`
}

type Mblog struct {
	// Visible struct {
	// 	Type   int `json:"type"`
	// 	ListID int `json:"list_id"`
	// } `json:"visible"`
	CreatedAt string `json:"created_at"`
	// ID        int64  `json:"id"`
	// Idstr     string `json:"idstr"`
	Mid     string `json:"mid"`
	Mblogid string `json:"mblogid"`
	User    struct {
		// 	ID                 int64  `json:"id"`
		Idstr string `json:"idstr"`
		// 	PcNew              int    `json:"pc_new"`
		ScreenName string `json:"screen_name"`
		// 	ProfileImageURL    string `json:"profile_image_url"`
		// 	ProfileURL         string `json:"profile_url"`
		// 	Verified           bool   `json:"verified"`
		// 	VerifiedType       int    `json:"verified_type"`
		// 	Domain             string `json:"domain"`
		// 	Weihao             string `json:"weihao"`
		// 	VerifiedTypeExt    int    `json:"verified_type_ext"`
		// 	StatusTotalCounter struct {
		// 		TotalCntFormat any    `json:"total_cnt_format"`
		// 		CommentCnt     string `json:"comment_cnt"`
		// 		RepostCnt      string `json:"repost_cnt"`
		// 		LikeCnt        string `json:"like_cnt"`
		// 		TotalCnt       string `json:"total_cnt"`
		// 	} `json:"status_total_counter"`
		// 	Remark      string `json:"remark"`
		// 	AvatarLarge string `json:"avatar_large"`
		AvatarHd string `json:"avatar_hd"`
		// 	FollowMe    bool   `json:"follow_me"`
		// 	Following   bool   `json:"following"`
		// 	Mbrank      int    `json:"mbrank"`
		// 	Mbtype      int    `json:"mbtype"`
		// 	VPlus       int    `json:"v_plus"`
		// 	UserAbility int    `json:"user_ability"`
		// 	PlanetVideo bool   `json:"planet_video"`
		// 	IconList    []struct {
		// 		Type string `json:"type"`
		// 		Data struct {
		// 			Mbrank int `json:"mbrank"`
		// 			Mbtype int `json:"mbtype"`
		// 			Svip   int `json:"svip"`
		// 			Vvip   int `json:"vvip"`
		// 		} `json:"data"`
		// 	} `json:"icon_list"`
	} `json:"user"`
	// CanEdit     bool `json:"can_edit"`
	EditCount int `json:"edit_count"`
	// TextLength  int  `json:"textLength,omitempty"`
	// Annotations []struct {
	// 	PhotoSubType  string `json:"photo_sub_type,omitempty"`
	// 	ClientMblogid string `json:"client_mblogid,omitempty"`
	// 	SourceText    string `json:"source_text,omitempty"`
	// 	PhoneID       string `json:"phone_id,omitempty"`
	// 	MapiRequest   bool   `json:"mapi_request,omitempty"`
	// } `json:"annotations"`
	Source string `json:"source"`
	// Favorited     bool     `json:"favorited"`
	// Rid           string   `json:"rid"`
	// Cardid        string   `json:"cardid"`
	PicIds []string `json:"pic_ids"`
	// PicFocusPoint []struct {
	// 	FocusPoint struct {
	// 		Left   float64 `json:"left"`
	// 		Top    float64 `json:"top"`
	// 		Width  float64 `json:"width"`
	// 		Height float64 `json:"height"`
	// 	} `json:"focus_point"`
	// 	PicID string `json:"pic_id"`
	// } `json:"pic_focus_point,omitempty"`
	// PicNum   int `json:"pic_num"`
	PicInfos map[string]struct {
		// 	Thumbnail  PicInfo `json:"thumbnail"`
		// 	Bmiddle    PicInfo `json:"bmiddle"`
		// 	Large      PicInfo `json:"large"`
		// 	Original   PicInfo `json:"original"`
		Largest PicInfo `json:"largest"`
		// 	Mw2000     PicInfo `json:"mw2000"`
		// 	Largecover PicInfo `json:"largecover"`
		// 	FocusPoint struct {
		// 		Left   float64 `json:"left"`
		// 		Top    float64 `json:"top"`
		// 		Width  float64 `json:"width"`
		// 		Height float64 `json:"height"`
		// 	} `json:"focus_point"`
		// 	ObjectID  string `json:"object_id"`
		// 	PicID     string `json:"pic_id"`
		// 	PhotoTag  int    `json:"photo_tag"`
		// 	Type      string `json:"type"`
		// 	PicStatus int    `json:"pic_status"`
	} `json:"pic_infos,omitempty"`
	// IsPaid                bool   `json:"is_paid"`
	// PicBgNew              string `json:"pic_bg_new"`
	// MblogVipType          int    `json:"mblog_vip_type"`
	// NumberDisplayStrategy struct {
	// 	ApplyScenarioFlag    int    `json:"apply_scenario_flag"`
	// 	DisplayTextMinNumber int    `json:"display_text_min_number"`
	// 	DisplayText          string `json:"display_text"`
	// } `json:"number_display_strategy"`
	// RepostsCount      int  `json:"reposts_count"`
	// CommentsCount     int  `json:"comments_count"`
	// AttitudesCount    int  `json:"attitudes_count"`
	// AttitudesStatus   int  `json:"attitudes_status"`
	// IsLongText        bool `json:"isLongText"`
	// Mlevel            int  `json:"mlevel"`
	// ContentAuth       int  `json:"content_auth"`
	// IsShowBulletin    int  `json:"is_show_bulletin"`
	// CommentManageInfo struct {
	// 	CommentPermissionType int `json:"comment_permission_type"`
	// 	ApprovalCommentType   int `json:"approval_comment_type"`
	// 	CommentSortType       int `json:"comment_sort_type"`
	// } `json:"comment_manage_info"`
	// ShareRepostType          int    `json:"share_repost_type"`
	IsTop int `json:"isTop,omitempty"`
	// Mblogtype                int    `json:"mblogtype"`
	// ShowFeedRepost           bool   `json:"showFeedRepost"`
	// ShowFeedComment          bool   `json:"showFeedComment"`
	// PictureViewerSign        bool   `json:"pictureViewerSign"`
	// ShowPictureViewer        bool   `json:"showPictureViewer"`
	// RcList                   []any  `json:"rcList"`
	// CanRemark                bool   `json:"can_remark,omitempty"`
	// AnalysisExtra            string `json:"analysis_extra"`
	// Readtimetype             string `json:"readtimetype"`
	// MixedCount               int    `json:"mixed_count"`
	// IsShowMixed              bool   `json:"is_show_mixed"`
	// MblogFeedBackMenusFormat []any  `json:"mblog_feed_back_menus_format"`
	// IsAd                     bool   `json:"isAd"`
	// IsSinglePayAudio         bool   `json:"isSinglePayAudio"`
	Text       string `json:"text"`
	TextRaw    string `json:"text_raw"`
	RegionName string `json:"region_name"`
	// RepostType               int    `json:"repost_type,omitempty"`
	RetweetedStatus *Mblog `json:"retweeted_status,omitempty"`
	// TopicStruct              []struct {
	// 	Title string `json:"title"`
	// 	TopicURL   string `json:"topic_url"`
	// 	TopicTitle string `json:"topic_title"`
	// 	Actionlog  struct {
	// 		ActType int    `json:"act_type"`
	// 		ActCode int    `json:"act_code"`
	// 		Oid     string `json:"oid"`
	// 		UUID    int64  `json:"uuid"`
	// 		Cardid  string `json:"cardid"`
	// 		Lcardid string `json:"lcardid"`
	// 		Uicode  string `json:"uicode"`
	// 		Luicode string `json:"luicode"`
	// 		Fid     string `json:"fid"`
	// 		Lfid    string `json:"lfid"`
	// 		Ext     string `json:"ext"`
	// 	} `json:"actionlog"`
	// } `json:"topic_struct,omitempty"`
	// URLStruct []struct {
	// 	URLTitle   string `json:"url_title"`
	// 	URLTypePic string `json:"url_type_pic"`
	// 	OriURL     string `json:"ori_url"`
	// 	PageID     string `json:"page_id"`
	// 	ShortURL   string `json:"short_url"`
	// 	LongURL    string `json:"long_url"`
	// 	URLType    any    `json:"url_type"`
	// 	Result     bool   `json:"result"`
	// 	Actionlog  struct {
	// 		ActType int    `json:"act_type"`
	// 		ActCode int    `json:"act_code"`
	// 		Oid     string `json:"oid"`
	// 		UUID    any    `json:"uuid"`
	// 		Cardid  string `json:"cardid"`
	// 		Lcardid string `json:"lcardid"`
	// 		Uicode  string `json:"uicode"`
	// 		Luicode string `json:"luicode"`
	// 		Fid     string `json:"fid"`
	// 		Lfid    string `json:"lfid"`
	// 		Ext     string `json:"ext"`
	// 	} `json:"actionlog"`
	// 	StorageType string `json:"storage_type"`
	// 	Hide        int    `json:"hide"`
	// 	ObjectType  string `json:"object_type"`
	// 	H5TargetURL string `json:"h5_target_url"`
	// 	NeedSaveObj int    `json:"need_save_obj"`
	// 	Log         string `json:"log"`
	// } `json:"url_struct,omitempty"`
	PageInfo struct {
		// 	Type       any    `json:"type"`
		// 	PageID     string `json:"page_id"`
		// 	ObjectType string `json:"object_type"`
		// 	Tips       string `json:"tips"`
		// 	PageDesc   string `json:"page_desc"`
		// 	PageTitle  string `json:"page_title"`
		// 	PagePic    string `json:"page_pic"`
		// 	TypeIcon   string `json:"type_icon"`
		// 	PageURL    string `json:"page_url"`
		// 	ObjectID   string `json:"object_id"`
		// 	ActStatus  int    `json:"act_status"`
		// 	Actionlog  struct {
		// 		ActType int    `json:"act_type"`
		// 		ActCode int    `json:"act_code"`
		// 		Oid     string `json:"oid"`
		// 		UUID    int64  `json:"uuid"`
		// 		Cardid  string `json:"cardid"`
		// 		Lcardid string `json:"lcardid"`
		// 		Uicode  string `json:"uicode"`
		// 		Luicode string `json:"luicode"`
		// 		Fid     string `json:"fid"`
		// 		Lfid    string `json:"lfid"`
		// 		Ext     string `json:"ext"`
		// 	} `json:"actionlog"`
		// 	Content1  string `json:"content1"`
		// 	Content2  string `json:"content2"`
		MediaInfo struct {
			// 		Name               string `json:"name"`
			// 		StreamURL          string `json:"stream_url"`
			// 		StreamURLHd        string `json:"stream_url_hd"`
			// 		Format             string `json:"format"`
			// 		H5URL              string `json:"h5_url"`
			// 		Mp4SdURL           string `json:"mp4_sd_url"`
			// 		Mp4HdURL           string `json:"mp4_hd_url"`
			// 		H265Mp4Hd          string `json:"h265_mp4_hd"`
			// 		H265Mp4Ld          string `json:"h265_mp4_ld"`
			// 		Inch4Mp4Hd         string `json:"inch_4_mp4_hd"`
			// 		Inch5Mp4Hd         string `json:"inch_5_mp4_hd"`
			// 		Inch55Mp4Hd        string `json:"inch_5_5_mp4_hd"`
			Mp4720PMp4 string `json:"mp4_720p_mp4"`
			// 		HevcMp4720P        string `json:"hevc_mp4_720p"`
			// 		PrefetchType       int    `json:"prefetch_type"`
			// 		PrefetchSize       int    `json:"prefetch_size"`
			// 		ActStatus          int    `json:"act_status"`
			// 		Protocol           string `json:"protocol"`
			// 		MediaID            string `json:"media_id"`
			// 		OriginTotalBitrate int    `json:"origin_total_bitrate"`
			// 		VideoOrientation   string `json:"video_orientation"`
			// 		Duration           int    `json:"duration"`
			// 		ForwardStrategy    int    `json:"forward_strategy"`
			// 		SearchScheme       string `json:"search_scheme"`
			// 		IsShortVideo       int    `json:"is_short_video"`
			// 		VoteIsShow         int    `json:"vote_is_show"`
			// 		BelongCollection   int    `json:"belong_collection"`
			// 		TitlesDisplayTime  string `json:"titles_display_time"`
			// 		ShowProgressBar    int    `json:"show_progress_bar"`
			// 		ShowMuteButton     bool   `json:"show_mute_button"`
			// 		ExtInfo            struct {
			// 			VideoOrientation string `json:"video_orientation"`
			// 		} `json:"ext_info"`
			// 		NextTitle             string `json:"next_title"`
			// 		KolTitle              string `json:"kol_title"`
			// 		PlayCompletionActions []struct {
			// 			Type         string `json:"type"`
			// 			Icon         string `json:"icon"`
			// 			Text         string `json:"text"`
			// 			Link         string `json:"link"`
			// 			BtnCode      int    `json:"btn_code"`
			// 			ShowPosition int    `json:"show_position"`
			// 			Actionlog    struct {
			// 				Oid     string `json:"oid"`
			// 				ActCode int    `json:"act_code"`
			// 				ActType int    `json:"act_type"`
			// 				Source  string `json:"source"`
			// 			} `json:"actionlog"`
			// 		} `json:"play_completion_actions"`
			// 		VideoPublishTime int    `json:"video_publish_time"`
			// 		PlayLoopType     int    `json:"play_loop_type"`
			// 		AuthorMid        string `json:"author_mid"`
			// 		AuthorName       string `json:"author_name"`
			// 		ExtraInfo        struct {
			// 			Sceneid string `json:"sceneid"`
			// 		} `json:"extra_info"`
			// 		VideoDownloadStrategy struct {
			// 			AbandonDownload int `json:"abandon_download"`
			// 		} `json:"video_download_strategy"`
			// 		JumpTo     int `json:"jump_to"`
			// 		BigPicInfo struct {
			// 			PicBig struct {
			// 				Height int    `json:"height"`
			// 				URL    string `json:"url"`
			// 				Width  int    `json:"width"`
			// 			} `json:"pic_big"`
			// 			PicSmall struct {
			// 				Height int    `json:"height"`
			// 				URL    string `json:"url"`
			// 				Width  int    `json:"width"`
			// 			} `json:"pic_small"`
			// 			PicMiddle struct {
			// 				Height int    `json:"height"`
			// 				URL    string `json:"url"`
			// 				Width  int    `json:"width"`
			// 			} `json:"pic_middle"`
			// 		} `json:"big_pic_info"`
			// 		OnlineUsers        string `json:"online_users"`
			// 		OnlineUsersNumber  int    `json:"online_users_number"`
			// 		TTL                int    `json:"ttl"`
			// 		StorageType        string `json:"storage_type"`
			// 		IsKeepCurrentMblog int    `json:"is_keep_current_mblog"`
			// 		AuthorInfo         struct {
			// 			ID                 int64  `json:"id"`
			// 			Idstr              string `json:"idstr"`
			// 			PcNew              int    `json:"pc_new"`
			// 			ScreenName         string `json:"screen_name"`
			// 			ProfileImageURL    string `json:"profile_image_url"`
			// 			ProfileURL         string `json:"profile_url"`
			// 			Verified           bool   `json:"verified"`
			// 			VerifiedType       int    `json:"verified_type"`
			// 			Domain             string `json:"domain"`
			// 			Weihao             string `json:"weihao"`
			// 			VerifiedTypeExt    int    `json:"verified_type_ext"`
			// 			StatusTotalCounter struct {
			// 				TotalCntFormat any    `json:"total_cnt_format"`
			// 				CommentCnt     string `json:"comment_cnt"`
			// 				RepostCnt      string `json:"repost_cnt"`
			// 				LikeCnt        string `json:"like_cnt"`
			// 				TotalCnt       string `json:"total_cnt"`
			// 			} `json:"status_total_counter"`
			// 			Remark            string `json:"remark"`
			// 			AvatarLarge       string `json:"avatar_large"`
			// 			AvatarHd          string `json:"avatar_hd"`
			// 			FollowMe          bool   `json:"follow_me"`
			// 			Following         bool   `json:"following"`
			// 			Mbrank            int    `json:"mbrank"`
			// 			Mbtype            int    `json:"mbtype"`
			// 			VPlus             int    `json:"v_plus"`
			// 			UserAbility       int    `json:"user_ability"`
			// 			PlanetVideo       bool   `json:"planet_video"`
			// 			VerifiedReason    string `json:"verified_reason"`
			// 			Description       string `json:"description"`
			// 			Location          string `json:"location"`
			// 			Gender            string `json:"gender"`
			// 			FollowersCount    int    `json:"followers_count"`
			// 			FollowersCountStr string `json:"followers_count_str"`
			// 			FriendsCount      int    `json:"friends_count"`
			// 			StatusesCount     int    `json:"statuses_count"`
			// 			URL               string `json:"url"`
			// 			Svip              int    `json:"svip"`
			// 			Vvip              int    `json:"vvip"`
			// 			CoverImagePhone   string `json:"cover_image_phone"`
			// 		} `json:"author_info"`
			// 		PlaybackList []struct {
			// 			Meta struct {
			// 				Label        string `json:"label"`
			// 				QualityIndex int    `json:"quality_index"`
			// 				QualityDesc  string `json:"quality_desc"`
			// 				QualityLabel string `json:"quality_label"`
			// 				QualityClass string `json:"quality_class"`
			// 				Type         int    `json:"type"`
			// 				QualityGroup int    `json:"quality_group"`
			// 				IsHidden     bool   `json:"is_hidden"`
			// 			} `json:"meta"`
			// 			PlayInfo struct {
			// 				Type               int     `json:"type"`
			// 				Mime               string  `json:"mime"`
			// 				Protocol           string  `json:"protocol"`
			// 				Label              string  `json:"label"`
			// 				URL                string  `json:"url"`
			// 				Bitrate            int     `json:"bitrate"`
			// 				PrefetchRange      string  `json:"prefetch_range"`
			// 				VideoCodecs        string  `json:"video_codecs"`
			// 				Fps                int     `json:"fps"`
			// 				Width              int     `json:"width"`
			// 				Height             int     `json:"height"`
			// 				Size               int     `json:"size"`
			// 				Duration           float64 `json:"duration"`
			// 				Sar                string  `json:"sar"`
			// 				AudioCodecs        string  `json:"audio_codecs"`
			// 				AudioSampleRate    int     `json:"audio_sample_rate"`
			// 				QualityLabel       string  `json:"quality_label"`
			// 				QualityClass       string  `json:"quality_class"`
			// 				QualityDesc        string  `json:"quality_desc"`
			// 				AudioChannels      int     `json:"audio_channels"`
			// 				AudioSampleFmt     string  `json:"audio_sample_fmt"`
			// 				AudioBitsPerSample int     `json:"audio_bits_per_sample"`
			// 				Watermark          string  `json:"watermark"`
			// 				Extension          struct {
			// 					TranscodeInfo struct {
			// 						PcdnRuleID    int    `json:"pcdn_rule_id"`
			// 						PcdnJank      int    `json:"pcdn_jank"`
			// 						OriginVideoDr string `json:"origin_video_dr"`
			// 						AbStrategies  string `json:"ab_strategies"`
			// 					} `json:"transcode_info"`
			// 				} `json:"extension"`
			// 				VideoDecoder     string `json:"video_decoder"`
			// 				PrefetchEnabled  bool   `json:"prefetch_enabled"`
			// 				TCPReceiveBuffer int    `json:"tcp_receive_buffer"`
			// 				DolbyAtmos       bool   `json:"dolby_atmos"`
			// 				ColorTransfer    string `json:"color_transfer"`
			// 				StereoVideo      int    `json:"stereo_video"`
			// 				FirstPktEndPos   int    `json:"first_pkt_end_pos"`
			// 			} `json:"play_info"`
			// 		} `json:"playback_list"`
		} `json:"media_info"`
		// PicInfo struct {
		// 		PicBig struct {
		// 			Height string `json:"height"`
		// 			URL    string `json:"url"`
		// 			Width  string `json:"width"`
		// 		} `json:"pic_big"`
		// 		PicSmall struct {
		// 			Height string `json:"height"`
		// 			URL    string `json:"url"`
		// 			Width  string `json:"width"`
		// 		} `json:"pic_small"`
		// 		PicMiddle struct {
		// 			URL    string `json:"url"`
		// 			Height string `json:"height"`
		// 			Width  string `json:"width"`
		// 		} `json:"pic_middle"`
		// } `json:"pic_info"`
		// 	Oid      string `json:"oid"`
		// 	AuthorID string `json:"author_id"`
		// 	Authorid string `json:"authorid"`
		// 	Warn     string `json:"warn"`
		// 	ShortURL string `json:"short_url"`
	} `json:"page_info,omitempty"`
	Title struct {
		Text string `json:"text"`
		// 	BaseColor int    `json:"base_color"`
		// 	IconURL   string `json:"icon_url"`
	} `json:"title,omitempty"`
}

type MymlogResponse struct {
	Ok      int    `json:"ok"`
	Message string `json:"message"`
	Data    struct {
		SinceID           any     `json:"since_id"`
		List              []Mblog `json:"list"`
		StatusVisible     int     `json:"status_visible"`
		BottomTipsVisible bool    `json:"bottom_tips_visible"`
		BottomTipsText    string  `json:"bottom_tips_text"`
		TopicList         []any   `json:"topicList"`
		Total             int     `json:"total"`
	} `json:"data"`
}

func (r MymlogResponse) Unwrap() error {
	if r.Ok != 1 {
		return fmt.Errorf("failed to get mymlog: %s (%d)", r.Message, r.Ok)
	}
	return nil
}

var _ req.Unwrap = (*MymlogResponse)(nil)

func GetMymlog(ctx context.Context, uid int, jar http.CookieJar) (r MymlogResponse, err error) {
	err = Session.ResultWithContext(ctx, Mymlog{CookieJar: jar, UID: uid}, &r)
	return
}

func GetMymlogIter(ctx context.Context, uid int, jar http.CookieJar) func(yield func(Mblog) bool) {
	return func(yield func(Mblog) bool) {
		r, err := GetMymlog(ctx, uid, jar)
		if err != nil {
			logger.Errorf("failed to get mymlog by %d: %s", uid, err)
		}
		for _, mblog := range r.Data.List {
			if !yield(mblog) {
				break
			}
		}
	}
}
