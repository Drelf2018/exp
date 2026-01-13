package hook

import (
	"io"
	"os"
	"runtime"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

func New(level logrus.Level, hooks ...logrus.Hook) *logrus.Logger {
	logger := &logrus.Logger{
		Out: io.Discard,
		Formatter: &nested.Formatter{
			TimestampFormat:       "2006-01-02 15:04:05",
			NoColors:              true,
			ShowFullLevel:         true,
			CustomCallerFormatter: func(*runtime.Frame) string { return "" },
		},
		Hooks:        make(logrus.LevelHooks),
		Level:        logrus.TraceLevel,
		ExitFunc:     os.Exit,
		ReportCaller: true,
	}
	var levels ConsoleHook
	for _, l := range logrus.AllLevels {
		if l <= level {
			levels = append(levels, l)
		}
	}
	logger.AddHook(levels)
	for _, hook := range hooks {
		logger.AddHook(hook)
	}
	return logger
}
