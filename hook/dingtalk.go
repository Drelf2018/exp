package hook

import (
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Drelf2018/dingtalk"
	"github.com/sirupsen/logrus"
)

const DingTalk string = "dingtalk"

var dingtalkTmpl, _ = template.New("dingtalk").
	Funcs(template.FuncMap{
		"upper": strings.ToUpper,
		"base":  filepath.Base,
	}).
	Parse(`### [{{upper .Level.String}}] {{if .Data.title}}{{.Data.title}}{{else}}{{base .Caller.File}}{{end}}

{{.Message}}

###### {{.Time.Format "2006-01-02 15:04:05"}}`)

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

func (d *DingTalkHook) Fire(entry *logrus.Entry) error {
	if data, ok := entry.Data[DingTalk]; ok {
		if data, ok := data.(string); ok && data == d.Name {
			delete(entry.Data, DingTalk)
			var b strings.Builder
			err := dingtalkTmpl.Execute(&b, entry)
			if err != nil {
				return err
			}
			go func(logger *logrus.Logger, text string) {
				title := strings.ReplaceAll(text, "\n\n", "\n")
				title = strings.ReplaceAll(title, "\n", " ")
				title = strings.ReplaceAll(title, "#", "")
				err := d.SendMarkdown(title, text)
				if err != nil {
					logger.Error(err)
				}
			}(entry.Logger, b.String())
		}
	}
	return nil
}

var _ logrus.Hook = (*DingTalkHook)(nil)

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	return &DingTalkHook{Bot: bot, levels: levels}
}
