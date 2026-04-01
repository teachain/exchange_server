package model

import "github.com/gorilla/websocket"

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
