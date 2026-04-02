package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	mu          sync.Mutex
	file        *os.File
	filename    string
	maxSize     int64
	maxFiles    int
	currentSize int64
}

type LoggerConfig struct {
	Filename string
	MaxSize  int64
	MaxFiles int
}

func NewLogger(config LoggerConfig) (*Logger, error) {
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024
	}
	if config.MaxFiles == 0 {
		config.MaxFiles = 10
	}

	logger := &Logger{
		filename: config.Filename,
		maxSize:  config.MaxSize,
		maxFiles: config.MaxFiles,
	}

	if err := logger.openFile(); err != nil {
		return nil, err
	}

	return logger, nil
}

func (l *Logger) openFile() error {
	dir := filepath.Dir(l.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.file = file
	if stat, err := file.Stat(); err == nil {
		l.currentSize = stat.Size()
	} else {
		l.currentSize = 0
	}

	return nil
}

func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentSize >= l.maxSize {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	if err != nil {
		return 0, err
	}
	l.currentSize += int64(n)

	return n, nil
}

func (l *Logger) rotate() error {
	l.file.Close()

	archivePath := fmt.Sprintf("%s.%s", l.filename, time.Now().Format("20060102150405"))
	if err := os.Rename(l.filename, archivePath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	l.cleanupOldFiles()

	newFile, err := os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}
	l.file = newFile
	l.currentSize = 0

	return nil
}

func (l *Logger) cleanupOldFiles() {
	dir := filepath.Dir(l.filename)
	basename := filepath.Base(l.filename)
	prefix := basename + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	if len(files) <= l.maxFiles {
		return
	}

	sort.Strings(files)
	deleteCount := len(files) - l.maxFiles
	for i := 0; i < deleteCount; i++ {
		os.Remove(files[i])
	}
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) GetWriter() io.Writer {
	return l
}
