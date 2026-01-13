package logger

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint" // Nice colored output
)

const (
	LevelTrace  = slog.Level(-8)
	LevelDev    = slog.Level(-6)
	LevelSilent = slog.Level(100)
	LevelUser   = slog.Level(101)
)

var LevelNames = map[slog.Leveler]string{
	LevelTrace:  "TRACE",
	LevelDev:    "DEV",
	LevelSilent: "SILENT",
	LevelUser:   "",
}

type Logger struct {
	*slog.Logger
}

func (l *Logger) Trace(msg string, args ...any) {
	l.Log(context.TODO(), LevelTrace, msg, args...)
}
func (l *Logger) Dev(msg string, args ...any) {
	l.Log(context.TODO(), LevelDev, msg, args...)
}
func (l *Logger) Print(msg string, args ...any) {
	l.Log(context.TODO(), LevelUser, msg, args...)
}

var instance *Logger

func Initialize() {
	opts := &tint.Options{
		Level:      LevelSilent,
		TimeFormat: time.Kitchen,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				if name, ok := LevelNames[level]; ok {
					a.Value = slog.StringValue(name)
				}
			}
			return a
		},
	}

	instance = &Logger{
		Logger: slog.New(tint.NewHandler(os.Stdout, opts)),
	}
	slog.SetDefault(instance.Logger)
}

func Get() *Logger {
	if instance == nil {
		panic("logger not initialized - call Initialize() first")
	}
	return instance
}
