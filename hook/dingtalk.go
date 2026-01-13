package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Drelf2018/dingtalk"
	"github.com/sirupsen/logrus"
)

var Saki = &dingtalk.Bot{}

func init() {
	saki, err := os.ReadFile("saki.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(saki, Saki)
	if err != nil {
		panic(err)
	}
}

const DingTalk string = "dingtalk"

var dingtalkTmpl, _ = template.New("dingtalk").
	Funcs(template.FuncMap{
		"upper": strings.ToUpper,
		"base":  filepath.Base,
	}).
	Parse(`### [{{upper .Level.String}}] {{base .Caller.File}}

{{.Time.Format "2006-01-02 15:04:05"}}

{{.Message}}

{{if .Data}}| Key | Value |
|-----|-------|
{{range $k, $v := .Data}}| {{$k}} | {{$v}} |
{{end}}{{end}}`)

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
			go func(logger *logrus.Logger, msg, text string) {
				err := d.SendMarkdown(text[4:strings.Index(text, "\n")+1]+msg, text)
				if err != nil {
					logger.Error(err)
				}
			}(entry.Logger, entry.Message, b.String())
		}
	}
	return nil
}

var _ logrus.Hook = (*DingTalkHook)(nil)

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	return &DingTalkHook{Bot: bot, levels: levels}
}
