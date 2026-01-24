package hook

import (
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/sirupsen/logrus"
	stripmd "github.com/writeas/go-strip-markdown"
)

// DingTalk 钉钉键
const DingTalk string = "dingtalk"

// LoggerMsg 钉钉日志模板消息
type LoggerMsg dingtalk.ActionCard

func (LoggerMsg) Type() dingtalk.MsgType {
	return dingtalk.MsgActionCard
}

// DingTalkHook 钉钉机器人钩子
type DingTalkHook struct {
	*dingtalk.Bot
	levels []logrus.Level // 日志等级，为空时视为全部等级
}

func (d *DingTalkHook) Levels() []logrus.Level {
	if len(d.levels) != 0 {
		return d.levels
	}
	return logrus.AllLevels
}

// Fire 发送钉钉消息，发送失败时会将错误写入日志
func (d *DingTalkHook) Fire(entry *logrus.Entry) error {
	if data, ok := entry.Data[DingTalk]; ok {
		if data, ok := data.(string); ok && data == d.Bot.Name {
			log := &LoggerMsg{}
			if _, err := d.Bot.Fill(entry, log); err != nil {
				return err
			}
			go func(logger *logrus.Logger, msg *LoggerMsg) {
				if err := d.Bot.Send(msg); err != nil {
					logger.Error(err)
				}
			}(entry.Logger, log)
		}
	}
	return nil
}

var _ logrus.Hook = (*DingTalkHook)(nil)

// Bind 将当前机器人绑定在日志上
func (d *DingTalkHook) Bind(logger *logrus.Logger) *logrus.Entry {
	return logger.WithField(DingTalk, d.Bot.Name)
}

// FirstLine 生成通知日志的首行
func FirstLine(entry *logrus.Entry) string {
	if value, ok := entry.Data["header"]; ok {
		if header, ok := value.(string); ok {
			return header
		}
	}
	b := &strings.Builder{}
	b.WriteByte('[')
	b.WriteString(strings.ToUpper(entry.Level.String()))
	b.WriteByte(']')
	if value, ok := entry.Data["title"]; ok {
		if title, ok := value.(string); ok {
			b.WriteByte(' ')
			b.WriteString(title)
		}
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

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	bot.Funcs(template.FuncMap{"titlef": FirstLine, "prefix": Prefix, "stripmd": stripmd.Strip, "timef": TimeFormat}).Parse(LoggerMsg{
		Title:       " {{titlef .}}\n{{if stripmd .Message}}{{stripmd .Message}}\n{{end}}{{timef .Time}}",
		Text:        "{{if .Data.banner}}{{.Data.banner}}\n{{end}}### {{titlef .}}\n\n{{prefix .Message \"#### \"}}\n\n###### {{timef .Time}}",
		SingleTitle: "{{if .Data.button}}{{.Data.button}}{{end}}",
		SingleURL:   "{{if .Data.url}}{{.Data.url}}{{end}}",
	})
	return &DingTalkHook{Bot: bot, levels: levels}
}
