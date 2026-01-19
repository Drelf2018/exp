package hook

import (
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Drelf2018/dingtalk"
	"github.com/sirupsen/logrus"
)

// DingTalk 钉钉键
const DingTalk string = "dingtalk"

// LoggerMsg 钉钉日志模板消息
type LoggerMsg dingtalk.Markdown

func (LoggerMsg) Type() dingtalk.MsgType {
	return dingtalk.MsgMarkdown
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

// Prefix 为每行字符串添加前缀
func Prefix(s, prefix string) string {
	parts := strings.Split(s, "\n")
	newParts := make([]string, 0, len(parts))
	for _, p := range parts {
		newParts = append(newParts, prefix+p)
	}
	return strings.Join(newParts, "\n")
}

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	bot.Funcs(template.FuncMap{"upper": strings.ToUpper, "base": filepath.Base, "prefix": Prefix}).Parse(LoggerMsg{
		Title: " {{template \"title\" .}}\n{{.Message}}\n{{.Time.Format \"2006-01-02 15:04:05\"}}",
		Text:  "### {{template \"title\" .}}\n\n{{prefix .Message \"#### \"}}\n\n###### {{.Time.Format \"2006-01-02 15:04:05\"}}",
	})
	bot.NewTemplate("title", `[{{upper .Level.String}}] {{if .Data.title}}{{.Data.title}}{{else}}{{base .Caller.File}}{{end}}`)
	return &DingTalkHook{Bot: bot, levels: levels}
}
