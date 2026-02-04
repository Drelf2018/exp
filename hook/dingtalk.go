package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Drelf2018/dingtalk"
	"github.com/sirupsen/logrus"
	stripmd "github.com/writeas/go-strip-markdown"
)

const (
	// DingTalkKey 用来指定发送消息机器人名称的键
	DingTalkKey string = "dingtalk"

	// SyncKey 用来同步发送日志消息的键，值为真时启用。在发送消息完成前会阻塞其他日志的打印，谨慎使用
	SyncKey string = "sync"

	// BannerKey 设置日志消息头图的键，值为头图链接
	BannerKey string = "banner"

	// HeaderKey 设置日志消息头部的键
	HeaderKey string = "header"

	// TitleKey 设置日志消息标题的键
	TitleKey string = "title"

	// ButtonKey 设置日志消息跳转按钮文本的键
	ButtonKey string = "button"

	// URLKey 设置日志消息跳转按钮的键，值为跳转链接
	URLKey string = "url"
)

// Sync 用来同步发送日志消息
var Sync = logrus.Fields{SyncKey: true}

// Banner 设置日志消息头图
func Banner(url string) logrus.Fields {
	return logrus.Fields{BannerKey: url}
}

// Button 设置日志消息跳转按钮
func Button(text, url string) logrus.Fields {
	return logrus.Fields{ButtonKey: text, URLKey: url}
}

// CardLayout 日志卡片布局
type CardLayout struct {
	Banner  string // 头图
	Header  string // 首行
	Content string // 内容
	Footer  string // 页脚
}

func (d CardLayout) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Banner + d.Header + d.Content + d.Footer)
}

var _ json.Marshaler = CardLayout{}

// LoggerCard 日志类型消息
type LoggerCard struct {
	Title       CardLayout `json:"title"`
	Text        CardLayout `json:"text"`
	SingleTitle string     `json:"singleTitle,omitempty"`
	SingleURL   string     `json:"singleURL,omitempty"`
}

func (LoggerCard) Type() dingtalk.MsgType {
	return dingtalk.MsgActionCard
}

var _ dingtalk.Msg = LoggerCard{}

// Prefix 为每行字符串添加前缀
func Prefix(s, prefix string) string {
	parts := strings.Split(s, "\n")
	newParts := make([]string, 0, len(parts))
	for _, p := range parts {
		newParts = append(newParts, prefix+p)
	}
	return strings.Join(newParts, "\n")
}

// NewLoggerCard 新建日志类型消息
func NewLoggerCard(entry *logrus.Entry) *LoggerCard {
	message := entry.Message
	header, exists := entry.Data[HeaderKey]
	if !exists {
		level := "[" + strings.ToUpper(entry.Level.String()) + "]"
		if value, ok := entry.Data[logrus.ErrorKey]; ok {
			header = level + " " + entry.Message
			message = fmt.Sprint(value)
		} else if value, ok := entry.Data[TitleKey]; ok {
			header = level + " " + fmt.Sprint(value)
		} else if entry.Caller != nil {
			header = level + " " + filepath.Base(entry.Caller.File)
		} else {
			header = level
		}
	}
	card := &LoggerCard{
		Title: CardLayout{
			Header: fmt.Sprintf(" %s\n", header),
			Footer: entry.Time.Format("2006-01-02 15:04:05"),
		},
		Text: CardLayout{
			Header: fmt.Sprintf("### %s\n\n", header),
		},
	}
	card.Text.Footer = "###### " + card.Title.Footer
	if content := stripmd.Strip(message); content != "" {
		card.Title.Content = content + "\n"
	}
	if content := Prefix(message, "#### "); content != "" {
		card.Text.Content = content + "\n\n"
	}
	if banner, exists := entry.Data[BannerKey]; exists {
		card.Text.Banner = fmt.Sprintf("![](%s)\n", banner)
	}
	if button, exists := entry.Data[ButtonKey]; exists {
		card.SingleTitle = fmt.Sprint(button)
	}
	if url, exists := entry.Data[URLKey]; exists {
		card.SingleURL = fmt.Sprint(url)
	}
	return card
}

// DingTalkHook 钉钉机器人钩子
type DingTalkHook struct {
	// 钉钉机器人
	*dingtalk.Bot

	// 根据日志事件生成钉钉消息，值为空时使用内置日志类型消息生成函数
	New func(*logrus.Entry) (dingtalk.Msg, error)

	// 日志等级，为空时视为全部等级
	levels []logrus.Level
}

func (d *DingTalkHook) Levels() []logrus.Level {
	if len(d.levels) != 0 {
		return d.levels
	}
	return logrus.AllLevels
}

// Send 发送钉钉消息，发送失败时会将错误写入日志
func (d *DingTalkHook) Send(logger *logrus.Logger, msg dingtalk.Msg) {
	if err := d.Bot.Send(msg); err != nil {
		logger.Error(err)
	}
}

// Fire 触发日志转发，当日志事件中指定的钉钉机器人与自身名称相同时转发
func (d *DingTalkHook) Fire(entry *logrus.Entry) error {
	if entry.Data[DingTalkKey] == d.Bot.Name {
		var msg dingtalk.Msg
		if d.New != nil {
			var err error
			msg, err = d.New(entry)
			if err != nil {
				return err
			}
		} else {
			msg = NewLoggerCard(entry)
		}
		if entry.Data[SyncKey] == true {
			return d.Bot.Send(msg)
		}
		go d.Send(entry.Logger, msg)
	}
	return nil
}

var _ logrus.Hook = (*DingTalkHook)(nil)

// Bind 将当前机器人绑定在日志上
func (d *DingTalkHook) Bind(logger *logrus.Logger) *logrus.Entry {
	return logger.WithField(DingTalkKey, d.Bot.Name)
}

// WithError 用于打印错误
func (d *DingTalkHook) WithError(logger *logrus.Logger) func(title string, err error, fields ...logrus.Fields) {
	return func(title string, err error, fields ...logrus.Fields) {
		entry := logger.WithFields(logrus.Fields{DingTalkKey: d.Bot.Name, logrus.ErrorKey: err})
		for _, field := range fields {
			entry = entry.WithFields(field)
		}
		entry.Error(title)
	}
}

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	return &DingTalkHook{Bot: bot, levels: levels}
}
