package hook

import (
	"io"
	"runtime"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

func New(level logrus.Level, hooks ...logrus.Hook) *logrus.Logger {
	logger := &logrus.Logger{
		Out:   io.Discard,
		Hooks: make(logrus.LevelHooks),
		Formatter: &nested.Formatter{
			TimestampFormat:       "2006-01-02 15:04:05",
			NoColors:              true,
			ShowFullLevel:         true,
			CustomCallerFormatter: func(*runtime.Frame) string { return "" },
		},
		ReportCaller: true,
		Level:        logrus.TraceLevel,
	}
	logger.AddHook(ConsoleHook(logrus.AllLevels[:level+1]))
	for _, hook := range hooks {
		logger.AddHook(hook)
	}
	return logger
}
