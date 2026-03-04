package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/exp/qiniu"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

type Logger struct {
	Level    string `json:"level" yaml:"level" toml:"level" long:"level"`
	Filename string `json:"filename" yaml:"filename" toml:"filename" long:"filename"`
}

type Options struct {
	Weibo    int                      `json:"weibo" yaml:"weibo" toml:"weibo" long:"weibo" description:"微博 UID"`
	State    string                   `json:"state" yaml:"state" toml:"state" long:"state" description:"浏览器用户数据文件名"`
	Crontab  string                   `json:"crontab" yaml:"crontab" toml:"crontab" long:"crontab" description:"刷新 Cookie 任务"`
	Database model.Database           `json:"database" yaml:"database" toml:"database" long:"database" description:"数据库文件路径"`
	Logger   Logger                   `json:"Logger" yaml:"Logger" toml:"Logger" group:"Logger" description:"日志等级和文件名格式"`
	DingTalk *dingtalk.Bot            `json:"DingTalk" yaml:"DingTalk" toml:"DingTalk" group:"DingTalk" description:"钉钉机器人"`
	Qiniu    *qiniu.TemporaryUploader `json:"Qiniu" yaml:"Qiniu" toml:"Qiniu" group:"Qiniu" description:"七牛云凭证"`
}

func Default() *Options {
	return &Options{
		State:    "state.json",
		Crontab:  "0 4/6 * * *",
		Database: "blogs.db",
		Logger: Logger{
			Level:    "INFO",
			Filename: "logs/2006-01-02.log",
		},
		DingTalk: &dingtalk.Bot{
			Name: "Bot",
		},
		Qiniu: &qiniu.TemporaryUploader{
			DeleteAfterDays: 3,
		},
	}
}

func Write(path string, opts *Options) error {
	switch ext := filepath.Ext(path); strings.ToLower(ext) {
	case ".yaml", ".yml":
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return err
		}
		encoder := yaml.NewEncoder(f)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(opts)
	case ".json":
		data, err := json.MarshalIndent(opts, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, os.ModePerm)
	case ".toml":
		data, err := toml.Marshal(opts)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, os.ModePerm)
	default:
		return fmt.Errorf("不支持的配置文件格式: %q", ext)
	}
}
