package order

import (
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID         int64           `json:"order_id"`
	UserID     int64           `json:"user_id"`
	Market     string          `json:"market"`
	Side       Side            `json:"side"`
	Price      decimal.Decimal `json:"price"`
	Amount     decimal.Decimal `json:"amount"`
	Deal       decimal.Decimal `json:"deal"`
	Fee        decimal.Decimal `json:"fee"`
	Status     OrderStatus     `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

type OrderStatus int

const (
	OrderStatusPending   OrderStatus = 0
	OrderStatusPartial   OrderStatus = 1
	OrderStatusFilled    OrderStatus = 2
	OrderStatusCancelled OrderStatus = 3
)

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusPending:
		return "pending"
	case OrderStatusPartial:
		return "partial"
	case OrderStatusFilled:
		return "filled"
	case OrderStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}
