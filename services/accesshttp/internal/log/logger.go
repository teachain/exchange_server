package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type Logger struct {
	mu        sync.Mutex
	dir       string
	prefix    string
	maxSize   int64
	maxFiles  int
	level     Level
	current   *os.File
	currentSz int64
}

func NewLogger(dir, prefix string, maxSize int64, maxFiles int) *Logger {
	return &Logger{
		dir:      dir,
		prefix:   prefix,
		maxSize:  maxSize,
		maxFiles: maxFiles,
		level:    LevelInfo,
	}
}

func (l *Logger) Init() error {
	if err := os.MkdirAll(l.dir, 0755); err != nil {
		return err
	}
	return l.rotate()
}

func (l *Logger) rotate() error {
	if l.current != nil {
		l.current.Close()
	}

	now := time.Now()
	filename := filepath.Join(l.dir, fmt.Sprintf("%s.%s.log",
		l.prefix, now.Format("20060102.150405")))

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.current = f
	l.currentSz = 0

	l.cleanOld()

	return nil
}

func (l *Logger) cleanOld() {
	pattern := filepath.Join(l.dir, l.prefix+".*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	if len(matches) > l.maxFiles {
		sort.Strings(matches)
		oldest := matches[:len(matches)-l.maxFiles]
		for _, f := range oldest {
			os.Remove(f)
		}
	}
}

func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentSz >= l.maxSize {
		l.rotate()
	}

	n, err = l.current.Write(p)
	l.currentSz += int64(n)

	return n, err
}

func (l *Logger) WriteString(s string) error {
	_, err := l.Write([]byte(s))
	return err
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current != nil {
		return l.current.Close()
	}
	return nil
}

func (l *Logger) SetLevel(level Level) {
	l.level = level
}

func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

func (l *Logger) levelString(level Level) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func (l *Logger) Log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	msg := fmt.Sprintf("[%s] %s: %s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		l.levelString(level),
		fmt.Sprintf(format, args...))
	l.WriteString(msg)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(LevelDebug, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(LevelInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.Log(LevelWarn, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(LevelError, format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.Log(LevelFatal, format, args...)
}
