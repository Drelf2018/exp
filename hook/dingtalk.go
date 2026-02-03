package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/filler"
	"github.com/sirupsen/logrus"
	stripmd "github.com/writeas/go-strip-markdown"
)

// DingTalk 钉钉键
const DingTalk string = "dingtalk"

// Sync 同步发送钉钉消息键
const Sync string = "sync"

// DingTalkLayout 钉钉消息布局
type DingTalkLayout struct {
	Banner string
	Header string
	Main   string
	Footer string
}

func (d DingTalkLayout) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Banner + d.Header + d.Main + d.Footer)
}

var _ json.Marshaler = DingTalkLayout{}

// DingTalkLayout 用于钉钉消息的日志卡片
type DingTalkCard struct {
	Title       DingTalkLayout `json:"title"`
	Text        DingTalkLayout `json:"text"`
	SingleTitle string         `json:"singleTitle,omitempty"`
	SingleURL   string         `json:"singleURL,omitempty"`
}

func (DingTalkCard) Type() dingtalk.MsgType {
	return dingtalk.MsgActionCard
}

// Header 生成通知日志的头部
func Header(entry *logrus.Entry) string {
	if value, ok := entry.Data["header"]; ok {
		return fmt.Sprint(value)
	}
	b := &strings.Builder{}
	b.WriteByte('[')
	b.WriteString(strings.ToUpper(entry.Level.String()))
	b.WriteByte(']')
	if value, ok := entry.Data["title"]; ok {
		b.WriteByte(' ')
		fmt.Fprint(b, value)
	} else if entry.Caller != nil {
		b.WriteByte(' ')
		b.WriteString(filepath.Base(entry.Caller.File))
	}
	return b.String()
}

// Prefix 为每行字符串添加前缀
func Prefix(s, prefix string) string {
	parts := strings.Split(s, "\n")
	newParts := make([]string, 0, len(parts))
	for _, p := range parts {
		newParts = append(newParts, prefix+p)
	}
	return strings.Join(newParts, "\n")
}

// TimeFormat 格式化时间
func TimeFormat(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// 默认消息模板
var MsgTemplate = template.New("").Funcs(template.FuncMap{
	"header": Header,
	"strip":  stripmd.Strip,
	"timef":  TimeFormat,
	"prefix": Prefix,
})

var dingtalkCard dingtalk.Msg = DingTalkCard{
	Title: DingTalkLayout{
		Header: " {{header .}}\n",
		Main:   "{{if strip .Message}}{{strip .Message}}\n{{end}}",
		Footer: "{{timef .Time}}",
	},
	Text: DingTalkLayout{
		Banner: "{{if .Data.banner}}![]({{.Data.banner}})\n{{end}}",
		Header: "### {{header .}}\n\n",
		Main:   "{{prefix .Message \"#### \"}}\n\n",
		Footer: "###### {{timef .Time}}",
	},
	SingleTitle: "{{if .Data.button}}{{.Data.button}}{{end}}",
	SingleURL:   "{{if .Data.url}}{{.Data.url}}{{end}}",
}

func init() {
	err := filler.Parse(MsgTemplate, dingtalkCard)
	if err != nil {
		panic(err)
	}
}

// DingTalkHook 钉钉机器人钩子
type DingTalkHook struct {
	// 钉钉机器人
	*dingtalk.Bot

	// 自定义模板，用于将日志自动填充进钉钉 actionCard 类型消息
	Template *template.Template

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
func (d *DingTalkHook) Fire(entry *logrus.Entry) (err error) {
	if entry.Data[DingTalk] == d.Bot.Name {
		card := &DingTalkCard{}
		if d.Template != nil {
			err = filler.Fill(d.Template, entry, card)
		} else {
			err = filler.Fill(MsgTemplate, entry, card)
		}
		if err != nil {
			return
		}
		if entry.Data[Sync] == true {
			return d.Bot.Send(card)
		}
		go d.Send(entry.Logger, card)
	}
	return
}

var _ logrus.Hook = (*DingTalkHook)(nil)

// Bind 将当前机器人绑定在日志上
func (d *DingTalkHook) Bind(logger *logrus.Logger) *logrus.Entry {
	return logger.WithField(DingTalk, d.Bot.Name)
}

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	return &DingTalkHook{Bot: bot, levels: levels}
}
