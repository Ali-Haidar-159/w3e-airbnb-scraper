package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Logger wraps standard log with level-based output
type Logger struct {
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
	debug *log.Logger
}

// NewLogger creates a new structured logger
func NewLogger() *Logger {
	flags := log.Lmsgprefix
	return &Logger{
		info:  log.New(os.Stdout, "[INFO]  ", flags),
		warn:  log.New(os.Stdout, "[WARN]  ", flags),
		error: log.New(os.Stderr, "[ERROR] ", flags),
		debug: log.New(os.Stdout, "[DEBUG] ", flags),
	}
}

func (l *Logger) prefix() string {
	return fmt.Sprintf(" %s ", time.Now().Format("15:04:05"))
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.info.Printf(l.prefix()+msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.warn.Printf(l.prefix()+msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.error.Printf(l.prefix()+msg, args...)
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.debug.Printf(l.prefix()+msg, args...)
}