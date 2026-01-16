package hook

import (
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Drelf2018/dingtalk"
	"github.com/sirupsen/logrus"
)

// DingTalk 钉钉
const DingTalk string = "dingtalk"

// DingTalkMarkdown 钉钉 markdown 消息模板
const DingTalkMarkdown string = `### [{{upper .Level.String}}] {{if .Data.title}}{{.Data.title}}{{else}}{{base .Caller.File}}{{end}}

#### {{.Message}}

###### {{.Time.Format "2006-01-02 15:04:05"}}`

// DingTalkTemplateFunc 钉钉 markdown 消息模板中使用到的函数
var DingTalkTemplateFunc = template.FuncMap{"upper": strings.ToUpper, "base": filepath.Base}

// DingTalkTemplate 钉钉模板
var DingTalkTemplate = template.Must(template.New(DingTalk).Funcs(DingTalkTemplateFunc).Parse(DingTalkMarkdown))

// MarkdownReplacer 替换 markdown 部分符号
var MarkdownReplacer = strings.NewReplacer("# ", "", "#", "", "\n\n", "\n")

// DingTalkHook 钉钉机器人钩子
type DingTalkHook struct {
	*dingtalk.Bot
	levels []logrus.Level // 日志等级，为空时视为全部等级
}

// SendMarkdown 发送 markdown 消息，使用替换部分符号的文本作为标题，发送失败时会记录错误
func (d *DingTalkHook) SendMarkdown(logger *logrus.Logger, text string) {
	err := d.Bot.SendMarkdown(" "+MarkdownReplacer.Replace(text), text)
	if err != nil {
		logger.Error(err)
	}
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
			var b strings.Builder
			err := DingTalkTemplate.Execute(&b, entry)
			if err != nil {
				return err
			}
			go d.SendMarkdown(entry.Logger, b.String())
		}
	}
	return nil
}

var _ logrus.Hook = (*DingTalkHook)(nil)

// NewDingTalkHook 创建钉钉机器人钩子，日志等级为空时视为全部等级
func NewDingTalkHook(bot *dingtalk.Bot, levels ...logrus.Level) *DingTalkHook {
	return &DingTalkHook{Bot: bot, levels: levels}
}
