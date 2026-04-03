package log

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	case FATAL:
		return "fatal"
	default:
		return "info"
	}
}

func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return INFO
	}
}

type RotateLogger struct {
	mu         sync.Mutex
	file       *os.File
	logPath    string
	maxSize    int64
	maxBackups int
	minLevel   Level
}

func NewRotateLogger(logPath string, maxSize int64, maxBackups int, minLevel Level) (*RotateLogger, error) {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	rl := &RotateLogger{
		file:       f,
		logPath:    logPath,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		minLevel:   minLevel,
	}

	return rl, nil
}

func (rl *RotateLogger) Write(p []byte) (n int, err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.file == nil {
		return 0, io.EOF
	}

	if !rl.shouldLog(p) {
		return len(p), nil
	}

	info, err := rl.file.Stat()
	if err != nil {
		return 0, err
	}

	if info.Size() >= rl.maxSize {
		if err := rl.rotate(); err != nil {
			return 0, err
		}
	}

	return rl.file.Write(p)
}

func (rl *RotateLogger) shouldLog(p []byte) bool {
	if len(p) < 6 {
		return true
	}

	for i := 0; i < len(p); i++ {
		if p[i] == '[' {
			end := i + 7
			if end <= len(p) && p[end-1] == ']' {
				levelStr := strings.ToLower(string(p[i+1 : end-1]))
				switch levelStr {
				case "debug":
					return rl.minLevel <= DEBUG
				case "info":
					return rl.minLevel <= INFO
				case "warn":
					return rl.minLevel <= WARN
				case "error":
					return rl.minLevel <= ERROR
				case "fatal":
					return rl.minLevel <= FATAL
				}
			}
		}
		if p[i] == ' ' || p[i] == '\t' {
			continue
		}
		break
	}

	return true
}

func (rl *RotateLogger) SetLevel(level Level) {
	rl.minLevel = level
}

func (rl *RotateLogger) rotate() error {
	rl.file.Close()

	if err := os.Rename(rl.logPath, rl.logPath+".1"); err != nil {
		return err
	}

	f, err := os.OpenFile(rl.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	rl.file = f

	go rl.cleanup()

	return nil
}

func (rl *RotateLogger) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	pattern := rl.logPath + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	type fileInfo struct {
		path  string
		info  os.FileInfo
		index int
	}

	var files []fileInfo
	for _, match := range matches {
		if filepath.Ext(match) == ".gz" {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			files = append(files, fileInfo{path: match, info: info, index: -1})
			continue
		}
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		var idx int
		if _, err := fmt.Sscanf(filepath.Ext(match), ".%d", &idx); err == nil {
			files = append(files, fileInfo{path: match, info: info, index: idx})
		}
	}

	for i := len(files) - rl.maxBackups - 1; i >= 0; i-- {
		if i < len(files) {
			os.Remove(files[i].path)
		}
	}
}

func (rl *RotateLogger) Close() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.file != nil {
		return rl.file.Close()
	}
	return nil
}

func (rl *RotateLogger) compressOldLog(src string) error {
	dst := src + ".gz"
	gzFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer gzFile.Close()

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	gzWriter := gzip.NewWriter(gzFile)
	defer gzWriter.Close()

	_, err = io.Copy(gzWriter, srcFile)
	if err != nil {
		return err
	}

	return os.Remove(src)
}

func (rl *RotateLogger) GetLogPath() string {
	return rl.logPath
}

func (rl *RotateLogger) SetMaxSize(maxSize int64) {
	rl.maxSize = maxSize
}

func (rl *RotateLogger) SetMaxBackups(maxBackups int) {
	rl.maxBackups = maxBackups
}

func (rl *RotateLogger) Flush() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.file != nil {
		return rl.file.Sync()
	}
	return nil
}

func (rl *RotateLogger) Rotate() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.rotate()
}

type Logger interface {
	Write(p []byte) (n int, err error)
	Close() error
	Flush() error
	Rotate() error
}

func NewLogger(logPath string, minLevel Level) (*RotateLogger, error) {
	return NewRotateLogger(logPath, 100*1024*1024, 10, minLevel)
}
