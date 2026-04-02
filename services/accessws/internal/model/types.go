package model

import (
	"github.com/gorilla/websocket"
	"sync"
)

type ClientSession struct {
	ID      uint64
	Conn    *websocket.Conn
	Auth    bool
	UserID  uint32
	Source  string
	Markets map[string]bool
	Assets  map[string]bool
	Dead    bool
}

type JSONRPCRequest struct {
	ID     interface{}   `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

type JSONRPCResponse struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *JSONError  `json:"error,omitempty"`
}

type JSONError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewErrorResponse(id interface{}, code int, msg string) *JSONRPCResponse {
	return &JSONRPCResponse{
		ID: id,
		Error: &JSONError{
			Code:    code,
			Message: msg,
		},
	}
}

func NewSuccessResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		ID:     id,
		Result: result,
	}
}

const (
	ERR_INVALID_PARAMS      = 1
	ERR_INTERNAL_ERROR      = 2
	ERR_SERVICE_UNAVAILABLE = 3
	ERR_METHOD_NOT_FOUND    = 4
	ERR_TIMEOUT             = 5
	ERR_AUTH_REQUIRED       = 6
	ERR_AUTH_FAILED         = 11
)

type OrderEvent struct {
	Event int        `json:"event"`
	Order *OrderInfo `json:"order"`
	Stock string     `json:"stock"`
	Money string     `json:"money"`
}

type OrderInfo struct {
	ID     uint64 `json:"id"`
	Market string `json:"market"`
	Type   int    `json:"type"`
	Side   int    `json:"side"`
	Price  string `json:"price"`
	Amount string `json:"amount"`
	UserID uint32 `json:"user_id"`
	Status int    `json:"status"`
	CTime  int64  `json:"ctime"`
	MTime  int64  `json:"mtime"`
}

type BalanceInfo struct {
	Asset   string `json:"asset"`
	Balance string `json:"balance"`
	Lock    string `json:"lock"`
}

type DepthInfo struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

type KlineInfo [][]interface{}

type DealInfo struct {
	ID     uint64 `json:"id"`
	Time   int64  `json:"time"`
	Price  string `json:"price"`
	Amount string `json:"amount"`
	Side   int    `json:"side"`
}

type StateInfo struct {
	High   string `json:"high"`
	Low    string `json:"low"`
	Open   string `json:"open"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
	Last   string `json:"last"`
}

type TodayInfo struct {
	High   string `json:"high"`
	Low    string `json:"low"`
	Open   string `json:"open"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
	Last   string `json:"last"`
}

type DepthSnapshot struct {
	Bids       map[string]string
	Asks       map[string]string
	LastUpdate int64
}

type DealRecord struct {
	ID     int64   `json:"id"`
	Time   float64 `json:"time"`
	Type   string  `json:"type"`
	Amount string  `json:"amount"`
	Price  string  `json:"price"`
}

type DealsBuffer struct {
	records []DealRecord
	lastID  int64
	maxSize int
	mu      sync.Mutex
}

func NewDealsBuffer(maxSize int) *DealsBuffer {
	return &DealsBuffer{
		records: make([]DealRecord, 0, maxSize),
		lastID:  0,
		maxSize: maxSize,
	}
}

func (b *DealsBuffer) Add(deal DealRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if deal.ID <= b.lastID {
		return
	}
	b.records = append(b.records, deal)
	b.lastID = deal.ID
	if len(b.records) > b.maxSize {
		b.records = b.records[len(b.records)-b.maxSize:]
	}
}

func (b *DealsBuffer) GetAll() []DealRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]DealRecord, len(b.records))
	copy(result, b.records)
	return result
}

func (b *DealsBuffer) GetSince(sinceID int64) []DealRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []DealRecord
	for _, d := range b.records {
		if d.ID > sinceID {
			result = append(result, d)
		}
	}
	return result
}

func (b *DealsBuffer) GetLastID() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastID
}
