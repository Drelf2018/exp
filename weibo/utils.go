package main

import (
	"strings"
	"time"

	"github.com/Drelf2018/exp/model"
)

// NextTimeDuration 返回距离下次指定时间的间隔
func NextTimeDuration(hour, min, sec int) time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())
	switch {
	case now.Hour() > hour,
		now.Hour() == hour && now.Minute() > min,
		now.Hour() == hour && now.Minute() == min && now.Second() > sec:
		next = next.AddDate(0, 0, 1)
	}
	return time.Until(next)
}

// RenderMarkdown 将通用博文模型格式化成 Markdown
func RenderMarkdown(b *strings.Builder, blog *model.Blog, depth int) {
	prefix := strings.Repeat(">", depth) + " "
	if blog.Banner != "" {
		if depth != 0 {
			b.WriteString(prefix)
		}
		b.WriteString("![](")
		b.WriteString(blog.Banner)
		b.WriteString(")\n")
		if depth == 0 {
			b.WriteByte('\n')
		}
	}
	if depth != 0 {
		b.WriteString(prefix)
	}
	b.WriteString("### ")
	b.WriteString(blog.Name)
	if blog.Title != "" {
		b.WriteString(" ")
		b.WriteString(blog.Title)
	}
	b.WriteByte('\n')
	if depth == 0 {
		b.WriteByte('\n')
	} else {
		b.WriteString(prefix)
	}
	b.WriteString(strings.TrimSpace(blog.Content))
	b.WriteByte('\n')
	if depth == 0 {
		b.WriteByte('\n')
	}
	for _, a := range blog.Assets {
		if !strings.HasPrefix(a.MIME, "image") {
			continue
		}
		if depth != 0 {
			b.WriteString(prefix)
		}
		b.WriteString("![](")
		b.WriteString(a.URL)
		b.WriteByte(')')
		b.WriteByte('\n')
		if depth == 0 {
			b.WriteByte('\n')
		}
	}
	if blog.Reply != nil {
		blog.Reply.Banner = ""
		RenderMarkdown(b, blog.Reply, depth+1)
	}
}
