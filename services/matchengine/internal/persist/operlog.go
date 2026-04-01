package persist

import (
	"encoding/json"
	"fmt"
	stdlog "log"
	"time"

	"github.com/jmoiron/sqlx"
)

type OperLogType int

const (
	OperLogTypeOrderCreate   OperLogType = 1
	OperLogTypeOrderDeal     OperLogType = 2
	OperLogTypeOrderCancel   OperLogType = 3
	OperLogTypeBalanceChange OperLogType = 4
)

type OperLog struct {
	ID        int64       `db:"id" json:"id"`
	Type      OperLogType `db:"type" json:"type"`
	Data      string      `db:"data" json:"data"`
	CreatedAt time.Time   `db:"created_at" json:"created_at"`
}

func NewOperLog(operType OperLogType, data interface{}) (*OperLog, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &OperLog{
		Type:      operType,
		Data:      string(jsonData),
		CreatedAt: time.Now(),
	}, nil
}

type OperLogWriter struct {
	logs          chan *OperLog
	db            *sqlx.DB
	batchSize     int
	flushInterval time.Duration
}

func NewOperLogWriter(db *sqlx.DB) *OperLogWriter {
	w := &OperLogWriter{
		logs:          make(chan *OperLog, 10000),
		db:            db,
		batchSize:     100,
		flushInterval: time.Second,
	}

	go w.flushLoop()
	return w
}

func (w *OperLogWriter) Write(log *OperLog) {
	w.logs <- log
}

func (w *OperLogWriter) flushLoop() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]*OperLog, 0, w.batchSize)

	for {
		select {
		case l := <-w.logs:
			batch = append(batch, l)
			if len(batch) >= w.batchSize {
				if err := w.flush(batch); err != nil {
					stdlog.Printf("operlog flush failed: %v", err)
				}
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				if err := w.flush(batch); err != nil {
					stdlog.Printf("operlog flush failed: %v", err)
				}
				batch = batch[:0]
			}
		}
	}
}

func (w *OperLogWriter) flush(batch []*OperLog) error {
	tx, err := w.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}

	for _, l := range batch {
		_, err := tx.Exec("INSERT INTO operlog (type, data) VALUES (?, ?)", l.Type, l.Data)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("insert operlog failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}
