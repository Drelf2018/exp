package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/method"
)

var session, _ = req.NewSession(
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
			Description       string `json:"description"`
			FollowersCountStr string `json:"followers_count_str"`
			FriendsCount      int    `json:"friends_count"`
			CoverImagePhone   string `json:"cover_image_phone"`
		} `json:"user"`
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
	err = session.ResultWithContext(ctx, ProfileInfo{UID: uid, CookieJar: jar}, &r)
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
	CreatedAt string `json:"created_at"`
	Mid       string `json:"mid"`
	Mblogid   string `json:"mblogid"`
	User      struct {
		Idstr      string `json:"idstr"`
		ScreenName string `json:"screen_name"`
		AvatarHd   string `json:"avatar_hd"`
	} `json:"user"`
	EditCount int      `json:"edit_count"`
	Source    string   `json:"source"`
	PicIds    []string `json:"pic_ids"`
	PicInfos  map[string]struct {
		Largest PicInfo `json:"largest"`
	} `json:"pic_infos,omitempty"`
	IsTop           int    `json:"isTop,omitempty"`
	Text            string `json:"text"`
	TextRaw         string `json:"text_raw"`
	RegionName      string `json:"region_name"`
	RetweetedStatus *Mblog `json:"retweeted_status,omitempty"`
	PageInfo        struct {
		MediaInfo struct {
			Mp4720PMp4 string `json:"mp4_720p_mp4"`
		} `json:"media_info"`
	} `json:"page_info,omitempty"`
	Title struct {
		Text string `json:"text"`
	} `json:"title,omitempty"`
}

// ToBlog 将微博转换成通用博文模型
func (mblog *Mblog) ToBlog(ctx context.Context, jar http.CookieJar) *model.Blog {
	blog := &model.Blog{
		Edited:    uint64(mblog.EditCount),
		Platform:  "weibo.com",
		Type:      "blog",
		UID:       mblog.User.Idstr,
		Name:      mblog.User.ScreenName,
		Avatar:    mblog.User.AvatarHd,
		MID:       mblog.Mid,
		URL:       fmt.Sprintf("https://weibo.com/%s/%s", mblog.User.Idstr, mblog.Mblogid),
		Title:     mblog.Title.Text,
		Source:    mblog.RegionName,
		Content:   mblog.Text,
		Plaintext: mblog.TextRaw,
		Extra: map[string]any{
			"device": mblog.Source,
			"is_top": mblog.IsTop == 1,
		},
	}
	// 解析博主
	var r ProfileInfoResponse
	r, blog.Extra["profile_info_error"] = GetProfileInfo(ctx, blog.UID, jar)
	blog.Banner, _, _ = strings.Cut(r.Data.User.CoverImagePhone, ";")
	blog.Follower = r.Data.User.FollowersCountStr
	blog.Following = strconv.Itoa(r.Data.User.FriendsCount)
	blog.Description = r.Data.User.Description
	// 解析时间
	blog.Time, blog.Extra["time_parse_error"] = time.Parse(time.RubyDate, mblog.CreatedAt)
	// 解析配图
	for _, picID := range mblog.PicIds {
		if pic, ok := mblog.PicInfos[picID]; ok {
			asset := model.Asset{URL: pic.Largest.URL}
			ext := filepath.Ext(pic.Largest.URL)
			if ext != "" {
				asset.MIME = "image/" + strings.ToLower(ext[1:])
			}
			blog.Assets = append(blog.Assets, asset)
		}
	}
	// 解析视频
	if mblog.PageInfo.MediaInfo.Mp4720PMp4 != "" {
		blog.Assets = append(blog.Assets, model.Asset{URL: mblog.PageInfo.MediaInfo.Mp4720PMp4, MIME: "video/mp4"})
	}
	// 解析被回复博文
	if mblog.RetweetedStatus != nil {
		blog.Reply = mblog.RetweetedStatus.ToBlog(ctx, jar)
	}
	return blog
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
	err = session.ResultWithContext(ctx, Mymlog{CookieJar: jar, UID: uid}, &r)
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

type Upload struct {
	req.PostMultipartForm
	File io.Reader `req:"body"`
}

func (Upload) RawURL() string {
	return "http://serverless.nana7mi.link/api/file"
}
