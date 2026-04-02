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

type OperLogHandler interface {
	HandleOrderCreate(data []byte) error
	HandleOrderDeal(data []byte) error
	HandleOrderCancel(data []byte) error
	HandleBalanceChange(data []byte) error
}

func (w *OperLogWriter) Replay(fromID int64, handler OperLogHandler) error {
	query := "SELECT id, type, data, created_at FROM operlog WHERE id > ? ORDER BY id ASC LIMIT 10000"
	rows, err := w.db.Queryx(query, fromID)
	if err != nil {
		return fmt.Errorf("query operlog failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var log OperLog
		if err := rows.StructScan(&log); err != nil {
			return fmt.Errorf("scan operlog failed: %w", err)
		}

		if err := w.processLog(&log, handler); err != nil {
			stdlog.Printf("replay operlog %d failed: %v", log.ID, err)
			continue
		}
	}

	return nil
}

func (w *OperLogWriter) processLog(log *OperLog, handler OperLogHandler) error {
	data := []byte(log.Data)

	switch log.Type {
	case OperLogTypeOrderCreate:
		return handler.HandleOrderCreate(data)
	case OperLogTypeOrderDeal:
		return handler.HandleOrderDeal(data)
	case OperLogTypeOrderCancel:
		return handler.HandleOrderCancel(data)
	case OperLogTypeBalanceChange:
		return handler.HandleBalanceChange(data)
	default:
		return fmt.Errorf("unknown operlog type: %d", log.Type)
	}
}

func (w *OperLogWriter) GetLastLogID() (int64, error) {
	var id int64
	err := w.db.Get(&id, "SELECT MAX(id) FROM operlog")
	if err != nil {
		return 0, err
	}
	return id, nil
}
