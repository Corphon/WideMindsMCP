package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// Field 表示结构化日志的键值对。
type Field struct {
	Key   string
	Value interface{}
}

// KV 是创建 Field 的便捷方法。
func KV(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

var (
	loggerOnce    sync.Once
	defaultLogger *slog.Logger
)

// Logger 返回全局结构化日志对象（JSON 输出）。
func Logger() *slog.Logger {
	loggerOnce.Do(func() {
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
		})
		defaultLogger = slog.New(handler)
	})
	return defaultLogger
}

// SetLogger 允许替换全局日志对象，便于测试。
func SetLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	loggerOnce.Do(func() {})
	defaultLogger = logger
}

// With 生成带默认字段的派生日志器。
func With(fields ...Field) *slog.Logger {
	args := make([]any, 0, len(fields))
	for _, field := range fields {
		if field.Key == "" {
			continue
		}
		args = append(args, slog.Any(field.Key, field.Value))
	}
	return Logger().With(args...)
}

// Debug 输出调试级别日志。
func Debug(msg string, fields ...Field) {
	logWithLevel(slog.LevelDebug, msg, fields...)
}

// Info 输出信息级别日志。
func Info(msg string, fields ...Field) {
	logWithLevel(slog.LevelInfo, msg, fields...)
}

// Warn 输出警告级别日志。
func Warn(msg string, fields ...Field) {
	logWithLevel(slog.LevelWarn, msg, fields...)
}

// Error 输出错误级别日志。
func Error(msg string, fields ...Field) {
	logWithLevel(slog.LevelError, msg, fields...)
}

// Infof 兼容旧接口，建议改用 Info。
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Warnf 兼容旧接口，建议改用 Warn。
func Warnf(format string, args ...interface{}) {
	Warn(fmt.Sprintf(format, args...))
}

// Errorf 兼容旧接口，建议改用 Error。
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

func logWithLevel(level slog.Level, msg string, fields ...Field) {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, field := range fields {
		if field.Key == "" {
			continue
		}
		attrs = append(attrs, slog.Any(field.Key, field.Value))
	}
	Logger().LogAttrs(context.Background(), level, msg, attrs...)
}
