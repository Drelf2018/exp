package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/fangtang"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/qiniu"
	"github.com/go-viper/mapstructure/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Logger struct {
	Level    string `json:"level" yaml:"level" toml:"level" long:"level"`
	Filename string `json:"filename" yaml:"filename" toml:"filename" long:"filename"`
}

type Options struct {
	CSV      string                   `toml:"csv" description:"csv 文件路径"`
	FangTang fangtang.FangTang        `toml:"fangtang" description:"方糖密钥"`
	Logger   Logger                   `toml:"Logger" description:"日志等级和文件名格式"`
	DingTalk *dingtalk.Bot            `toml:"DingTalk" description:"钉钉机器人"`
	Error    *dingtalk.Bot            `toml:"Error" description:"错误消息钉钉机器人"`
	Qiniu    *qiniu.TemporaryUploader `toml:"Qiniu" description:"七牛云凭证"`
}

var (
	options Options
	logger  *logrus.Logger
	csv     *os.File
	bot     *logrus.Entry
	errBot  *logrus.Entry
)

// 获取运行参数
func init() {
	v := viper.New()
	v.SetConfigFile("config.toml")
	if err := v.ReadInConfig(); err != nil {
		logrus.Panic(err)
	}
	// 解析默认配置文件
	err := v.Unmarshal(&options, viper.DecoderConfigOption(func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "toml"
	}))
	if err != nil {
		logrus.Panic(err)
	}
	// 初始化日志
	ding := hook.NewDingTalkHook(options.DingTalk)
	erro := hook.NewDingTalkHook(options.Error)
	level, err := logrus.ParseLevel(options.Logger.Level)
	if err != nil {
		logrus.Panic(err)
	}
	logger = hook.New(level, hook.NewDailyFileHook(options.Logger.Filename), ding, erro)
	bot = ding.Bind(logger)
	errBot = erro.Bind(logger)
}

// 初始化数据库
func init() {
	if _, err := os.Stat(options.CSV); err != nil {
		csv, err = os.OpenFile(options.CSV, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			logger.Panicln("打开文件失败:", err)
		}
		_, err = csv.Write([]byte{0xEF, 0xBB, 0xBF})
		if err != nil {
			logger.Panicln("写入文件头失败:", err)
		}
		_, err = csv.WriteString("时间,类型,品牌,价格,型号\n")
		if err != nil {
			logger.Panicln("写入表头失败:", err)
		}
	} else {
		csv, err = os.OpenFile(options.CSV, os.O_APPEND, os.ModePerm)
		if err != nil {
			logger.Panicln("追加打开文件失败:", err)
		}
	}
}

// AppendCSV 追加写入 csv 文件
func AppendCSV(cmd string, product *Product) {
	_, err := fmt.Fprintf(csv, "%s,%s,%s,\"%s\",\"%s\"\n",
		time.Now().Format("2006-01-02 15:04:05"),
		cmd, product.Name, product.Price, product.VariantString(" ", ","))
	if err != nil {
		errBot.WithError(err).Error("写入文件失败")
	}
}

// SendDingTalk 钉钉推送商品
func SendDingTalk(cmd string, product *Product, image string) {
	fields := logrus.Fields{
		"header": cmd + " " + product.Name + " " + product.Price,
		"url":    product.URL(),
		"button": "查看详情",
	}
	if image != "" {
		filepath := fmt.Sprintf("acrnm%s_%s.jpg", product.Href, time.Now().Format("2006_01_02_15_04_05"))
		err := options.Qiniu.UploadURL(context.Background(), image, filepath, qiniu.JPEG)
		if err != nil {
			errBot.WithError(err).Error("上传图片失败")
		} else {
			fields["banner"] = "https://yun.nana7mi.link/" + filepath
		}
	}
	bot.WithFields(fields).Info(product.VariantString(" ", "\n"))
}

// SendFangTang 方糖推送商品
func SendFangTang(cmd string, product *Product, image string) {
	title := cmd + " " + product.Name + " " + product.Price
	desp := product.VariantString(" ", "\n") + "\n\n" + time.Now().Format("2006-01-02 15:04:05")
	if image != "" {
		desp += fmt.Sprintf("\n\n![%s](%s)", product.Href, image)
	}
	_, err := options.FangTang.Send(title, desp, fangtang.WeChat)
	if err != nil {
		errBot.WithError(err).Error("方糖推送失败")
	}
}

func main() {
	logger.Info("运行中")
	acrnm := &Acrnm{
		OnNew: func(products []*Product) {
			for _, p := range products {
				AppendCSV("上新", p)
				image, err := GetProductImage(p.URL())
				if err != nil {
					logger.Errorln("获取图片失败:", err)
				}
				SendDingTalk("上新", p, image)
				SendFangTang("上新", p, image)
			}
		},
		OnUpdate: func(products []*Product) {
			for _, p := range products {
				AppendCSV("变更", p)
				image, err := GetProductImage(p.URL())
				if err != nil {
					logger.Errorln("获取图片失败:", err)
				}
				SendDingTalk("变更", p, image)
				SendFangTang("变更", p, image)
			}
		},
		OnRemoval: func(products []*Product) {
			for _, p := range products {
				AppendCSV("下架", p)
			}
		},
		OnError: func(err error) {
			errBot.WithError(err).Error("请求发生错误")
		},
	}
	err := acrnm.Run()
	if err != nil {
		logger.Panic(err)
	}
}
