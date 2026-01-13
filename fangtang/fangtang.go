package fangtang

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Drelf2018/req"
)

// Channel 推送通道，已内置十种类型
//
//	Test          // 测试号
//	WeChatWorkBot // 企业微信群机器人
//	DingTalkBot   // 钉钉群机器人
//	LarkBot       // 飞书群机器人
//	BarkiOS       // Bark iOS
//	WeChat        // 方糖服务号
//	PushDeer      // PushDeer
//	WeChatWork    // 企业微信应用消息
//	Webhook       // 自定义 Webhook
//	Android       // 官方Android版·β
type Channel string

const (
	Test          Channel = "0"  // 测试号
	WeChatWorkBot Channel = "1"  // 企业微信群机器人
	DingTalkBot   Channel = "2"  // 钉钉群机器人
	LarkBot       Channel = "3"  // 飞书群机器人
	Bark          Channel = "8"  // Bark iOS
	WeChat        Channel = "9"  // 方糖服务号
	PushDeer      Channel = "18" // PushDeer
	WeChatWork    Channel = "66" // 企业微信应用消息
	Webhook       Channel = "88" // 自定义 Webhook
	Android       Channel = "98" // 官方Android版·β
)

// Send 方糖推送
type Send struct {
	req.PostJSON

	// 发送密钥
	SendKey string

	// 消息标题，最大长度为 32
	Title string `req:"body"`

	// 消息内容，支持 Markdown 语法
	Desp string `req:"body"`

	// 标签，用 | 分隔
	Tags string `req:"body,omitempty"`

	// 消息卡片内容，最大长度64，如果不指定，将自动从desp中截取生成
	Short string `req:"body,omitempty"`

	// 是否隐藏调用IP，如果不指定，则显示，为 1 则隐藏
	Noip int `req:"body,omitempty"`

	// 动态指定本次推送使用的消息通道
	Channel Channel `req:"body,omitempty"`

	// 消息抄送的 openid
	Openid string `req:"body,omitempty"`
}

func (s Send) RawURL() string {
	if len(s.SendKey) >= 4 && s.SendKey[:4] == "sctp" {
		return fmt.Sprintf("https://%s.push.ft07.com/send", s.SendKey)
	}
	return fmt.Sprintf("https://sctapi.ftqq.com/%s.send", s.SendKey)
}

var _ req.API = Send{}

// Push 查询推送状态
type Push struct {
	PushID  string `json:"pushid" req:"query:id"`
	ReadKey string `json:"readkey" req:"query:readkey"`
}

func (Push) Method() string {
	return http.MethodGet
}

func (p Push) RawURL() string {
	return "https://sctapi.ftqq.com/push"
}

var _ req.API = Push{}

// PushResponse 查询推送状态的响应体
type PushResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID        int             `json:"id"`
		UID       string          `json:"uid"`
		Title     string          `json:"title"`
		Desp      string          `json:"desp"`
		Encoded   any             `json:"encoded"`
		ReadKey   string          `json:"readkey"`
		WXStatus  json.RawMessage `json:"wxstatus"`
		IP        string          `json:"ip"`
		CreatedAt time.Time       `json:"created_at"`
		UpdatedAt time.Time       `json:"updated_at"`
	} `json:"data"`
}

type PushStatus struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (r PushResponse) Status() (s PushStatus, err error) {
	var str string
	err = json.Unmarshal(r.Data.WXStatus, &str)
	if err != nil {
		return s, fmt.Errorf("fangtang: failed to unmarshal raw wxstatus: %w", err)
	}
	err = json.Unmarshal([]byte(str), &s)
	if err != nil {
		return s, fmt.Errorf("fangtang: failed to unmarshal wxstatus: %w", err)
	}
	return s, nil
}

func (r PushResponse) Unwrap() error {
	if r.Code == 0 {
		return nil
	}
	return fmt.Errorf("fangtang: failed to push: %s (%d)", r.Message, r.Code)
}

var _ req.Unwrap = PushResponse{}

// SendResponse 方糖推送的响应体
type SendResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Errno int    `json:"errno"`
		Error string `json:"error"`
		Push
	} `json:"data"`
}

func (r SendResponse) Unwrap() error {
	if r.Code == 0 {
		return nil
	}
	return fmt.Errorf("fangtang: failed to send: %s (%d)", r.Message, r.Code)
}

var _ req.Unwrap = SendResponse{}

// PostSend 推送消息
func PostSend(ctx context.Context, sendkey, title, desp string, channel ...Channel) (SendResponse, error) {
	api := &Send{SendKey: sendkey, Title: title, Desp: desp}
	if len(channel) != 0 {
		parts := make([]string, 0, len(channel))
		for _, ch := range channel {
			parts = append(parts, string(ch))
		}
		api.Channel = Channel(strings.Join(parts, "|"))
	}
	return req.ResultWithContext[SendResponse](ctx, api)
}

// FangTang 方糖推送
type FangTang string

// SendWithContext 携带上下文推送消息，会等待异步推送结果再返回
func (f FangTang) SendWithContext(ctx context.Context, title string, desp string, channel ...Channel) error {
	r, err := PostSend(ctx, string(f), title, desp, channel...)
	if err != nil {
		return err
	}
	if r.Data.Errno != 0 {
		return fmt.Errorf("fangtang: failed to get push: %s (%d)", r.Data.Error, r.Data.Errno)
	}
	var pr PushResponse
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return err
		case <-timeout:
			return err
		case <-ticker.C:
			pr, err = req.ResultWithContext[PushResponse](ctx, r.Data.Push)
			if err == nil {
				status, err := pr.Status()
				if err == nil && status.ErrCode == 0 {
					return nil
				}
			}
		}
	}
}

// Send 推送消息，会等待异步推送结果再返回
func (f FangTang) Send(title string, desp string, channel ...Channel) error {
	return f.SendWithContext(context.Background(), title, desp, channel...)
}
