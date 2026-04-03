package history

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"github.com/teachain/exchange_server/internal/matchengine/order"
)

const (
	HistoryTypeUserBalance = iota
	HistoryTypeUserOrder
	HistoryTypeUserDeal
	HistoryTypeOrderDetail
	HistoryTypeOrderDeal
)

const HistoryHashNum = 16

const MaxHistoryPendingSize = 5000

type HistoryRecord struct {
	Type int
	Hash uint32
	SQL  string
}

type HistoryWriter struct {
	db          *sqlx.DB
	pending     map[string]*HistoryRecord
	mu          sync.Mutex
	flushTimer  *time.Ticker
	stopCh      chan struct{}
	tablePrefix string
	blocked     bool
}

func NewHistoryWriter(db *sqlx.DB, tablePrefix string) *HistoryWriter {
	return &HistoryWriter{
		db:          db,
		pending:     make(map[string]*HistoryRecord),
		flushTimer:  time.NewTicker(100 * time.Millisecond),
		stopCh:      make(chan struct{}),
		tablePrefix: tablePrefix,
		blocked:     false,
	}
}

func (w *HistoryWriter) Start() {
	go w.flushLoop()
}

func (w *HistoryWriter) Stop() {
	close(w.stopCh)
	w.flushPending()
}

func (w *HistoryWriter) IsBlocked() bool {
	return w.blocked
}

func (w *HistoryWriter) PendingCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pending)
}

func (w *HistoryWriter) flushLoop() {
	for {
		select {
		case <-w.flushTimer.C:
			w.flushPending()
		case <-w.stopCh:
			return
		}
	}
}

func (w *HistoryWriter) getKey(histType int, hash uint32) string {
	return fmt.Sprintf("%d_%d", histType, hash)
}

func (w *HistoryWriter) AppendSQL(histType int, hash uint32, sql string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) >= MaxHistoryPendingSize {
		w.blocked = true
		log.Printf("history pending queue is full, blocking writes")
		return
	}
	w.blocked = false

	key := w.getKey(histType, hash)
	rec, exists := w.pending[key]
	if !exists {
		w.pending[key] = &HistoryRecord{
			Type: histType,
			Hash: hash,
			SQL:  sql,
		}
	} else {
		rec.SQL = rec.SQL + ", " + sql
	}
}

func (w *HistoryWriter) flushPending() {
	w.mu.Lock()
	if len(w.pending) == 0 {
		w.mu.Unlock()
		return
	}

	pending := w.pending
	w.pending = make(map[string]*HistoryRecord)
	w.mu.Unlock()

	for _, rec := range pending {
		if rec.SQL == "" {
			continue
		}
		sql := rec.SQL
		log.Printf("exec history sql: %s", sql)
		if _, err := w.db.Exec(sql); err != nil {
			if !strings.Contains(err.Error(), "Duplicate") {
				log.Printf("exec history sql failed: %v", err)
			}
		}
	}
}

func (w *HistoryWriter) AppendUserOrder(o *order.Order) {
	hash := uint32(o.UserID % HistoryHashNum)

	sql := fmt.Sprintf(
		"INSERT INTO `order_history_%d` (`id`, `create_time`, `finish_time`, `user_id`, `market`, `source`, `t`, `side`, `price`, `amount`, `taker_fee`, `maker_fee`, `deal_stock`, `deal_money`, `deal_fee`) VALUES (%d, %f, %f, %d, '%s', '%s', %d, %d, '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
		hash,
		o.ID,
		float64(o.CreateTime.Unix()),
		float64(o.UpdateTime.Unix()),
		o.UserID,
		o.Market,
		o.Source,
		o.Type,
		o.Side,
		o.Price.String(),
		o.Amount.String(),
		o.TakerFee.String(),
		o.MakerFee.String(),
		o.Left.Sub(o.Amount).String(),
		o.Price.Mul(o.Left.Sub(o.Amount)).String(),
		o.DealFee.String(),
	)

	w.AppendSQL(HistoryTypeUserOrder, hash, sql)
}

func (w *HistoryWriter) AppendOrderDetail(o *order.Order) {
	hash := uint32(o.ID % HistoryHashNum)

	sql := fmt.Sprintf(
		"INSERT INTO `order_detail_%d` (`id`, `create_time`, `finish_time`, `user_id`, `market`, `source`, `t`, `side`, `price`, `amount`, `taker_fee`, `maker_fee`, `deal_stock`, `deal_money`, `deal_fee`) VALUES (%d, %f, %f, %d, '%s', '%s', %d, %d, '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
		hash,
		o.ID,
		float64(o.CreateTime.Unix()),
		float64(o.UpdateTime.Unix()),
		o.UserID,
		o.Market,
		o.Source,
		o.Type,
		o.Side,
		o.Price.String(),
		o.Amount.String(),
		o.TakerFee.String(),
		o.MakerFee.String(),
		o.Left.Sub(o.Amount).String(),
		o.Price.Mul(o.Left.Sub(o.Amount)).String(),
		o.DealFee.String(),
	)

	w.AppendSQL(HistoryTypeOrderDetail, hash, sql)
}

type DealRecord struct {
	ID        uint64
	Time      float64
	UserID    uint32
	Market    string
	DealID    uint64
	OrderID   uint64
	DealOrder uint64
	Side      int
	Role      int
	Price     decimal.Decimal
	Amount    decimal.Decimal
	Deal      decimal.Decimal
	Fee       decimal.Decimal
	DealFee   decimal.Decimal
}

func (w *HistoryWriter) AppendOrderDeal(deal DealRecord, hash uint32) {
	sql := fmt.Sprintf(
		"INSERT INTO `deal_history_%d` (`id`, `time`, `user_id`, `deal_id`, `order_id`, `deal_order_id`, `role`, `price`, `amount`, `deal`, `fee`, `deal_fee`) VALUES (NULL, %f, %d, %d, %d, %d, %d, '%s', '%s', '%s', '%s', '%s')",
		hash,
		deal.Time,
		deal.UserID,
		deal.DealID,
		deal.OrderID,
		deal.DealOrder,
		deal.Role,
		deal.Price.String(),
		deal.Amount.String(),
		deal.Deal.String(),
		deal.Fee.String(),
		deal.DealFee.String(),
	)

	w.AppendSQL(HistoryTypeOrderDeal, hash, sql)
}

func (w *HistoryWriter) AppendUserDeal(deal DealRecord, hash uint32) {
	sql := fmt.Sprintf(
		"INSERT INTO `user_deal_history_%d` (`id`, `time`, `user_id`, `market`, `deal_id`, `order_id`, `deal_order_id`, `side`, `role`, `price`, `amount`, `deal`, `fee`, `deal_fee`) VALUES (NULL, %f, %d, '%s', %d, %d, %d, %d, %d, '%s', '%s', '%s', '%s', '%s')",
		hash,
		deal.Time,
		deal.UserID,
		deal.Market,
		deal.DealID,
		deal.OrderID,
		deal.DealOrder,
		deal.Side,
		deal.Role,
		deal.Price.String(),
		deal.Amount.String(),
		deal.Deal.String(),
		deal.Fee.String(),
		deal.DealFee.String(),
	)

	w.AppendSQL(HistoryTypeUserDeal, hash, sql)
}

type BalanceChange struct {
	ID       uint64
	Time     float64
	UserID   uint32
	Asset    string
	Business string
	Change   decimal.Decimal
	Balance  decimal.Decimal
	Detail   string
}

func (w *HistoryWriter) AppendBalanceChange(change BalanceChange) {
	hash := change.UserID % HistoryHashNum

	sql := fmt.Sprintf(
		"INSERT INTO `balance_history_%d` (`id`, `time`, `user_id`, `asset`, `business`, `change`, `balance`, `detail`) VALUES (NULL, %f, %d, '%s', '%s', '%s', '%s', '%s')",
		hash,
		change.Time,
		change.UserID,
		change.Asset,
		change.Business,
		change.Change.String(),
		change.Balance.String(),
		change.Detail,
	)

	w.AppendSQL(HistoryTypeUserBalance, hash, sql)
}

func (w *HistoryWriter) AppendOrderHistory(o *order.Order) {
	w.AppendUserOrder(o)
	w.AppendOrderDetail(o)
}

func (w *HistoryWriter) AppendOrderDealHistory(dealID uint64, t time.Time, ask, bid *order.Order, askRole, bidRole int, price, amount, deal, askFee, bidFee decimal.Decimal) {
	askHash := uint32(ask.ID % HistoryHashNum)
	bidHash := uint32(bid.ID % HistoryHashNum)
	dealTime := float64(t.Unix())

	w.AppendOrderDeal(DealRecord{
		ID:        ask.ID,
		Time:      dealTime,
		UserID:    ask.UserID,
		Market:    ask.Market,
		DealID:    dealID,
		OrderID:   ask.ID,
		DealOrder: bid.ID,
		Role:      askRole,
		Price:     price,
		Amount:    amount,
		Deal:      deal,
		Fee:       askFee,
		DealFee:   bidFee,
	}, askHash)

	w.AppendOrderDeal(DealRecord{
		ID:        bid.ID,
		Time:      dealTime,
		UserID:    bid.UserID,
		Market:    bid.Market,
		DealID:    dealID,
		OrderID:   bid.ID,
		DealOrder: ask.ID,
		Role:      bidRole,
		Price:     price,
		Amount:    amount,
		Deal:      deal,
		Fee:       bidFee,
		DealFee:   askFee,
	}, bidHash)

	askUserHash := uint32(ask.UserID % HistoryHashNum)
	bidUserHash := uint32(bid.UserID % HistoryHashNum)

	w.AppendUserDeal(DealRecord{
		ID:        ask.ID,
		Time:      dealTime,
		UserID:    ask.UserID,
		Market:    ask.Market,
		DealID:    dealID,
		OrderID:   ask.ID,
		DealOrder: bid.ID,
		Side:      int(ask.Side),
		Role:      askRole,
		Price:     price,
		Amount:    amount,
		Deal:      deal,
		Fee:       askFee,
		DealFee:   bidFee,
	}, askUserHash)

	w.AppendUserDeal(DealRecord{
		ID:        bid.ID,
		Time:      dealTime,
		UserID:    bid.UserID,
		Market:    bid.Market,
		DealID:    dealID,
		OrderID:   bid.ID,
		DealOrder: ask.ID,
		Side:      int(bid.Side),
		Role:      bidRole,
		Price:     price,
		Amount:    amount,
		Deal:      deal,
		Fee:       bidFee,
		DealFee:   askFee,
	}, bidUserHash)
}

func (w *HistoryWriter) AppendUserBalanceHistory(userID uint32, asset, business string, change decimal.Decimal, balance decimal.Decimal, detail string) {
	w.AppendBalanceChange(BalanceChange{
		Time:     float64(time.Now().Unix()),
		UserID:   userID,
		Asset:    asset,
		Business: business,
		Change:   change,
		Balance:  balance,
		Detail:   detail,
	})
}
