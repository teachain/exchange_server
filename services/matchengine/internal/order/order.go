package order

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderType uint8

const (
	OrderTypeLimit     OrderType = 1
	OrderTypeMarket    OrderType = 2
	OrderTypeStopLoss  OrderType = 3
	OrderTypeStopLimit OrderType = 4
)

type OrderSide uint8

const (
	OrderSideAsk OrderSide = 1
	OrderSideBid OrderSide = 2
)

type OrderStatus uint8

const (
	OrderStatusPending  OrderStatus = 1
	OrderStatusPartial  OrderStatus = 2
	OrderStatusFinished OrderStatus = 3
	OrderStatusCanceled OrderStatus = 4
)

type Order struct {
	ID         uint64          `json:"order_id"`
	Type       OrderType       `json:"type"`
	Side       Side            `json:"side"`
	Status     OrderStatus     `json:"status"`
	UserID     uint32          `json:"user_id"`
	Market     string          `json:"market"`
	Price      decimal.Decimal `json:"price"`
	Amount     decimal.Decimal `json:"amount"`
	Left       decimal.Decimal `json:"left"`
	Freeze     decimal.Decimal `json:"freeze"`
	TakerFee   decimal.Decimal `json:"taker_fee"`
	MakerFee   decimal.Decimal `json:"maker_fee"`
	DealStock  decimal.Decimal `json:"deal_stock"`
	DealMoney  decimal.Decimal `json:"deal_money"`
	DealFee    decimal.Decimal `json:"deal_fee"`
	Source     string          `json:"source"`
	CreateTime time.Time       `json:"create_time"`
	UpdateTime time.Time       `json:"update_time"`
}

func (o *Order) IsBuy() bool {
	return o.Side == SideBid
}

func (o *Order) IsSell() bool {
	return o.Side == SideAsk
}

func (o *Order) IsFinished() bool {
	return o.Status == OrderStatusFinished
}

func (o *Order) IsCanceled() bool {
	return o.Status == OrderStatusCanceled
}

func (o *Order) IsActive() bool {
	return o.Status == OrderStatusPending || o.Status == OrderStatusPartial
}

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusPending:
		return "pending"
	case OrderStatusPartial:
		return "partial"
	case OrderStatusFinished:
		return "finished"
	case OrderStatusCanceled:
		return "cancelled"
	default:
		return "unknown"
	}
}
