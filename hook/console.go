package hook

import (
	"os"

	"github.com/sirupsen/logrus"
)

// ConsoleHook 控制台钩子
type ConsoleHook []logrus.Level

func (c ConsoleHook) Levels() []logrus.Level {
	return []logrus.Level(c)
}

func (ConsoleHook) Fire(entry *logrus.Entry) error {
	b, err := entry.Bytes()
	if err != nil {
		return err
	}
	_, err = os.Stderr.Write(b)
	return err
}
