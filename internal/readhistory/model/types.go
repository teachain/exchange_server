package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderHistory struct {
	ID         int64           `db:"id"`
	CreateTime float64         `db:"create_time"`
	FinishTime float64         `db:"finish_time"`
	UserID     int64           `db:"user_id"`
	Market     string          `db:"market"`
	Source     string          `db:"source"`
	Type       int             `db:"t"`
	Side       int             `db:"side"`
	Price      decimal.Decimal `db:"price"`
	Amount     decimal.Decimal `db:"amount"`
	TakerFee   decimal.Decimal `db:"taker_fee"`
	MakerFee   decimal.Decimal `db:"maker_fee"`
	DealStock  decimal.Decimal `db:"deal_stock"`
	DealMoney  decimal.Decimal `db:"deal_money"`
	DealFee    decimal.Decimal `db:"deal_fee"`
	Status     int             `db:"status"`
	CreatedAt  time.Time       `db:"created_at"`
	FinishedAt *time.Time      `db:"finished_at"`
}

type DealHistory struct {
	ID            int64           `db:"id"`
	Time          float64         `db:"time"`
	UserID        int64           `db:"user_id"`
	DealID        int64           `db:"deal_id"`
	OrderID       int64           `db:"order_id"`
	Side          int             `db:"side"`
	Role          int             `db:"role"`
	Price         decimal.Decimal `db:"price"`
	Amount        decimal.Decimal `db:"amount"`
	Deal          decimal.Decimal `db:"deal"`
	Fee           decimal.Decimal `db:"fee"`
	DealOrderID   int64           `db:"deal_order_id"`
	Market        string          `db:"market"`
	CounterUserID int64           `db:"counter_user_id"`
	CreatedAt     time.Time       `db:"created_at"`
}

type BalanceHistory struct {
	ID        int64           `db:"id"`
	Time      float64         `db:"time"`
	UserID    int64           `db:"user_id"`
	Asset     string          `db:"asset"`
	Business  string          `db:"business"`
	Change    decimal.Decimal `db:"change"`
	Balance   decimal.Decimal `db:"balance"`
	Detail    string          `db:"detail"`
	CreatedAt time.Time       `db:"created_at"`
}
