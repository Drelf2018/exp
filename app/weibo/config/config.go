package config

import (
	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/exp/qiniu"
)

type Logger struct {
	Level    string `json:"level" yaml:"level" toml:"level" long:"level" default:"info"`
	Filename string `json:"filename" yaml:"filename" toml:"filename" long:"filename"`
}

type Options struct {
	Target   int                      `long:"target" description:"监控目标 UID"`
	Crontab  string                   `long:"crontab" description:"刷新 Cookie 任务"`
	State    string                   `long:"state" description:"浏览器用户数据文件名" default:"state.json"`
	Database model.Database           `long:"database" description:"数据库文件路径"`
	Logger   Logger                   `group:"Logger" description:"日志等级和文件名格式"`
	DingTalk *dingtalk.Bot            `group:"DingTalk" description:"钉钉机器人"`
	Qiniu    *qiniu.TemporaryUploader `group:"Qiniu" description:"七牛云凭证"`
}
