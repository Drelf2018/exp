package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Drelf2018/dingtalk"
	"github.com/Drelf2018/exp/hook"
	"github.com/Drelf2018/exp/model"
	"github.com/Drelf2018/req/cookie"
	"github.com/gofiber/fiber/v2"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
)

var logger = hook.New(logrus.InfoLevel, hook.NewDailyFileHook("logs/2006-01-02.log"), hook.NewDingTalkHook(hook.Saki))
var saki = logger.WithField(hook.DingTalk, hook.Saki.Name)
var port = 8654

func ToBlog(ctx context.Context, jar http.CookieJar, mblog *Mblog) *model.Blog {
	if mblog == nil {
		return nil
	}
	blog := &model.Blog{
		Edited:    uint64(mblog.EditCount),
		Platform:  "weibo.com",
		Type:      "blog",
		UID:       mblog.User.Idstr,
		Name:      mblog.User.ScreenName,
		Avatar:    mblog.User.AvatarHd,
		MID:       mblog.Mid,
		URL:       fmt.Sprintf("https://weibo.com/%s/%s", mblog.User.Idstr, mblog.Mblogid),
		Title:     mblog.Title.Text,
		Source:    mblog.RegionName,
		Content:   mblog.Text,
		Plaintext: mblog.TextRaw,
		Extra: map[string]any{
			"device": mblog.Source,
			"is_top": mblog.IsTop == 1,
		},
	}
	// 解析博主
	var r ProfileInfoResponse
	r, blog.Extra["profile_info_error"] = GetProfileInfo(ctx, blog.UID, jar)
	blog.Banner, _, _ = strings.Cut(r.Data.User.CoverImagePhone, ";")
	blog.Follower = r.Data.User.FollowersCountStr
	blog.Following = strconv.Itoa(r.Data.User.FriendsCount)
	blog.Description = r.Data.User.Description
	// 解析时间
	blog.Time, blog.Extra["time_parse_error"] = time.Parse(time.RubyDate, mblog.CreatedAt)
	// 解析配图
	for _, picID := range mblog.PicIds {
		if pic, ok := mblog.PicInfos[picID]; ok {
			asset := model.Asset{URL: pic.Largest.URL}
			ext := filepath.Ext(pic.Largest.URL)
			if ext != "" {
				asset.MIME = "image/" + strings.ToLower(ext[1:])
			}
			blog.Assets = append(blog.Assets, asset)
		}
	}
	// 解析视频
	if mblog.PageInfo.MediaInfo.Mp4720PMp4 != "" {
		blog.Assets = append(blog.Assets, model.Asset{URL: mblog.PageInfo.MediaInfo.Mp4720PMp4, MIME: "video/mp4"})
	}
	// 解析被回复博文
	if mblog.RetweetedStatus != nil {
		blog.Reply = ToBlog(ctx, jar, mblog.RetweetedStatus)
	}
	return blog
}

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

func fetch(ctx context.Context, jar http.CookieJar, target int) {
	blogs := make(map[string]Mblog)
	cleanTicker := time.NewTicker(24 * time.Hour)
	defer cleanTicker.Stop()
	fetchTicker := time.NewTicker(10 * time.Second)
	defer fetchTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanTicker.C:
			for k, v := range blogs {
				created, err := time.Parse(time.RubyDate, v.CreatedAt)
				if err != nil || time.Since(created) > 14*24*time.Hour {
					delete(blogs, k)
					logger.Infoln("Blog", k, "has been deleted")
				}
			}
		case <-fetchTicker.C:
			logger.Debugf("Getting blogs (%d blogs have been saved)", len(blogs))
			for mblog := range GetMymlogIter(ctx, target, jar) {
				// 当前博文未保存则写入文件，并且保存在映射中，会比较编辑次数是否有差异，如果有差异则写入新文件
				if v, ok := blogs[mblog.Mid]; ok && v.EditCount == mblog.EditCount {
					continue
				}
				blogs[mblog.Mid] = mblog
				filename := "./blogs/" + mblog.Mid
				if mblog.EditCount != 0 {
					filename += "." + strconv.Itoa(mblog.EditCount)
				}
				// 已经保存过文件
				if _, err := os.Stat(filename + ".json"); err == nil {
					continue
				}
				// 推送
				logger.Infoln("Saving blog", mblog.Mid)
				blog := ToBlog(ctx, jar, &mblog)
				go func(blog *model.Blog) {
					var b strings.Builder
					RenderMarkdown(&b, blog, 0)
					err := hook.Saki.Send(dingtalk.ActionCard{
						SingleURL:   blog.URL,
						SingleTitle: "阅读全文",
						Text:        b.String(),
						Title:       fmt.Sprintln(blog.Name, blog.Plaintext),
					})
					if err != nil {
						saki.Error(err)
						err = hook.Saki.SendLink(blog.Name, blog.Plaintext, blog.Avatar, blog.URL)
						if err != nil {
							saki.Error(err)
						}
					}
				}(blog)
				// 保存至文件
				b, err := json.MarshalIndent(blog, "", "\t")
				if err != nil {
					saki.Error(err)
					continue
				}
				err = os.WriteFile(filename+".json", b, os.ModePerm)
				if err != nil {
					saki.Error(err)
				}
			}
		}
	}
}

type Options struct {
	Me     int `short:"m" long:"me" description:"你的微博 UID"`
	Target int `short:"t" long:"target" description:"监控目标 UID"`
}

var ErrNoTarget = errors.New("no target")

func main() {
	// 解析命令行参数
	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		logger.Panic(err)
	}
	// 未提供命令行参数，读取配置文件
	if opts.Target == 0 {
		err = flags.IniParse("config.ini", &opts)
		if err != nil {
			logger.Panic(err)
		}
	}
	if opts.Target == 0 {
		logger.Panic(ErrNoTarget)
	}
	// 获取 Cookie
	c, err := NewCookieJar(opts.Me)
	if err != nil {
		logger.Panic(err)
	}
	// 保存博文
	err = os.MkdirAll("./blogs", os.ModePerm)
	if err != nil {
		logger.Panic(err)
	}
	// 开启 Cookie 保活
	ctx, cancel := context.WithCancel(context.Background())
	cookie.KeepaliveWithContext(ctx, c, Refresher, 6*time.Hour)
	// 轮询获取博文
	go fetch(ctx, c, opts.Target)
	// 创建后端
	app := fiber.New()
	app.Static("/", "./blogs", fiber.Static{Browse: true})
	// 监听程序关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// 阻塞等待信号
		<-sigCh
		cancel()
		close(sigCh)
		app.Shutdown()
	}()
	// 开启后端
	app.Listen(fmt.Sprintf("0.0.0.0:%d", port))
}
