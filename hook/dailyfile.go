package hook

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// IsSameDay 判断两个时间是否为同一天
func IsSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.In(time.Local).Date()
	y2, m2, d2 := t2.In(time.Local).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// DailyFileHook 以本地时区的日期为单位将日志写入文件的钩子
type DailyFileHook struct {
	Layout string         // 日志文件路径模板，会利用日志事件的时间进行格式化处理，参考值 "logs/2006-01-02.log"
	mu     sync.Mutex     // 日志锁
	file   *os.File       // 日志文件
	date   time.Time      // 日志文件的创建时间
	levels []logrus.Level // 日志等级，为空时视为全部等级
}

func (d *DailyFileHook) Levels() []logrus.Level {
	if len(d.levels) != 0 {
		return d.levels
	}
	return logrus.AllLevels
}

func (d *DailyFileHook) Fire(entry *logrus.Entry) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// 将日志事件时间与当前日志文件的创建时间比较
	if !IsSameDay(entry.Time, d.date) {
		// 格式化新日志文件路径
		filePath := entry.Time.Format(d.Layout)
		// 创建前置文件夹
		err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
		if err != nil {
			return err
		}
		// 关闭当前日志
		if d.file != nil {
			_ = d.file.Close()
		}
		// 打开新日志
		d.file, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
		if err != nil {
			return err
		}
		// 更新日志创建时间
		d.date = entry.Time.In(time.Local)
	}
	// 写入文件
	b, err := entry.Bytes()
	if err != nil {
		return err
	}
	if d.file != nil {
		_, err = d.file.Write(b)
	}
	return err
}

var _ logrus.Hook = (*DailyFileHook)(nil)

// NewDailyFileHook 创建写入文件钩子，日志等级为空时视为全部等级
func NewDailyFileHook(layout string, levels ...logrus.Level) *DailyFileHook {
	return &DailyFileHook{Layout: layout, levels: levels}
}
