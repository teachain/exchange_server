package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RotateLogger struct {
	mu         sync.Mutex
	file       *os.File
	filename   string
	maxSize    int64
	maxBackups int
	maxAge     int
}

func NewRotateLogger(filename string, maxSize int64, maxBackups int, maxAge int) (*RotateLogger, error) {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	rl := &RotateLogger{
		filename:   filename,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		maxAge:     maxAge,
	}

	if err := rl.open(); err != nil {
		return nil, err
	}

	return rl, nil
}

func (rl *RotateLogger) open() error {
	f, err := os.OpenFile(rl.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	rl.file = f
	return nil
}

func (rl *RotateLogger) Write(p []byte) (n int, err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	info, err := rl.file.Stat()
	if err != nil {
		return 0, err
	}

	if info.Size()+int64(len(p)) >= rl.maxSize {
		if err := rl.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = rl.file.Write(p)
	if err != nil {
		return n, err
	}

	return n, nil
}

func (rl *RotateLogger) rotate() error {
	rl.file.Close()

	backupName := fmt.Sprintf("%s.%s", rl.filename, time.Now().Format("20060102150405"))
	if err := os.Rename(rl.filename, backupName); err != nil {
		return err
	}

	if err := rl.open(); err != nil {
		return err
	}

	rl.cleanup()

	return nil
}

func (rl *RotateLogger) cleanup() {
	if rl.maxBackups <= 0 && rl.maxAge <= 0 {
		return
	}

	pattern := rl.filename + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	now := time.Now()
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		age := now.Sub(info.ModTime())
		if rl.maxAge > 0 && age > time.Duration(rl.maxAge)*24*time.Hour {
			os.Remove(match)
			continue
		}

		if rl.maxBackups > 0 {
			if len(matches) > rl.maxBackups {
				os.Remove(match)
			}
		}
	}
}

func (rl *RotateLogger) Close() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.file.Close()
}

func InitLogger(logFile string, maxSize int64, maxBackups int, maxAge int) {
	logger, err := NewRotateLogger(logFile, maxSize, maxBackups, maxAge)
	if err != nil {
		log.Fatalf("Failed to create rotate logger: %v", err)
	}

	multi := io.MultiWriter(os.Stdout, logger)
	log.SetOutput(multi)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type Scheduler struct {
	tasks  map[string]*time.Timer
	mu     sync.Mutex
	stopCh chan struct{}
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:  make(map[string]*time.Timer),
		stopCh: make(chan struct{}),
	}
}

func (s *Scheduler) Schedule(interval time.Duration, name string, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tasks[name] != nil {
		s.tasks[name].Stop()
	}

	s.tasks[name] = time.AfterFunc(interval, func() {
		fn()
		s.Schedule(interval, name, fn)
	})
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.stopCh)
	for _, t := range s.tasks {
		t.Stop()
	}
}
