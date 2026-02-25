# filler

使用预设模板自动填充结构体字段

### 使用

```go
type DingTalkCard struct {
	Title       string
	Text        string
	SingleTitle string
	SingleURL   string
}

var dingtalkCard = DingTalkCard{
	Title: " {{.Message}}",
	Text: `### {{.Message}}

###### {{.Time.Format "2006-01-02 15:04:05"}}`,
	SingleTitle: "{{if .Data.button}}{{.Data.button}}{{end}}",
	SingleURL:   "{{if .Data.url}}{{.Data.url}}{{end}}",
}

var tmpl = filler.Must(dingtalkCard)

func TestParse(t *testing.T) {
	var data = map[string]any{
		"Message": "你好！",
		"Time":    time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
		"Data": map[string]string{
			"button": "查看原文",
			"url":    "https://httpbin.org",
		},
	}
	var card DingTalkCard
	err := filler.Fill(tmpl, data, &card)
	if err != nil {
		t.Fatal(err)
	}
	assert := func(s1, s2 string) {
		if s1 != s2 {
			t.Fatal(s1, s2)
		}
	}
	assert(card.Title, " 你好！")
	assert(card.Text, "### 你好！\n\n###### 2006-01-02 15:04:05")
	assert(card.SingleTitle, "查看原文")
	assert(card.SingleURL, "https://httpbin.org")
}
```
