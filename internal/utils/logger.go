package utils

import (
	"log"
	"os"
	"sync"
)

var (
	loggerOnce    sync.Once
	defaultLogger *log.Logger
)

// Logger returns a singleton logger instance writing to stdout.
func Logger() *log.Logger {
	loggerOnce.Do(func() {
		defaultLogger = log.New(os.Stdout, "[WideMinds] ", log.LstdFlags|log.Lshortfile)
	})
	return defaultLogger
}

// SetLogger allows replacing the global logger instance.
func SetLogger(logger *log.Logger) {
	if logger == nil {
		return
	}
	loggerOnce.Do(func() {})
	defaultLogger = logger
}

// Infof logs an informational message.
func Infof(format string, args ...interface{}) {
	Logger().Printf("INFO: "+format, args...)
}

// Warnf logs a warning message.
func Warnf(format string, args ...interface{}) {
	Logger().Printf("WARN: "+format, args...)
}

// Errorf logs an error message.
func Errorf(format string, args ...interface{}) {
	Logger().Printf("ERROR: "+format, args...)
}
