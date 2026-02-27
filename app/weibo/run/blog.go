package run

import (
	"context"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/model"
	"github.com/google/uuid"
)

const (
	CardTmpl = "{{if .Banner}}![]({{.Banner}})\n\n{{end}}{{template \"blog\" .}}\n\n###### {{.Time.Format \"2006-01-02 15:04:05\"}}"
	BlogTmpl = `### {{.Name}}{{if (and .Title (ne .Type "like"))}} {{.Title}}{{end}}

{{prefix .Plaintext "#### "}}{{range .Assets}}{{if or (suffix . ".jpg") (suffix . ".jpeg") (suffix . ".png")}}

![]({{.}}){{end}}{{end}}{{if .Reply}}

{{template "blog" .Reply}}{{end}}`
)

var tmpl *template.Template

// 初始化博文模板
func init() {
	funcMap := template.FuncMap{"suffix": strings.HasSuffix, "prefix": hook.Prefix}
	tmpl = template.Must(template.New("").Funcs(funcMap).Parse(CardTmpl))
	template.Must(tmpl.New("blog").Parse(BlogTmpl))
}

// SendLink 发送链接
func SendLink(ctx context.Context, bot *dingtalk.Bot, blog *model.Blog) error {
	err := bot.SendLinkWithContext(ctx, blog.Name, blog.Plaintext, blog.URL, blog.Avatar)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			err = urlErr.Unwrap()
		}
	}
	return err
}

// SendActionCard 发送卡片通知
func SendActionCard(ctx context.Context, bot *dingtalk.Bot, blog *model.Blog) error {
	var b strings.Builder
	err := tmpl.Execute(&b, blog)
	if err != nil {
		return err
	}
	// 重试三次，如果一直系统繁忙则切换发送方式
	msgUUID := dingtalk.UUID(uuid.NewString())
	for i := range 3 {
		if i != 0 {
			time.Sleep((1 << i) * time.Second)
		}
		// 发送成功，直接返回
		err = bot.SendActionCard(" "+blog.String(), b.String(), "阅读全文", blog.URL, msgUUID)
		if err == nil {
			return nil
		}
		// 服务器系统繁忙，等待后重试
		if respErr, ok := err.(dingtalk.SendError); ok && respErr.ErrCode == -1 {
			continue
		}
		// 其他错误，不再重试
		if urlErr, ok := err.(*url.Error); ok {
			err = urlErr.Unwrap()
		}
		break
	}
	return err
}
