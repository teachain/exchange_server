package engine

import (
	"container/heap"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

var TakerFeeRate = decimal.NewFromFloat(0.001)
var MakerFeeRate = decimal.NewFromFloat(0.001)

const (
	OrderEventPut    = 1
	OrderEventUpdate = 2
	OrderEventFinish = 3
)

func (e *Engine) ProcessOrder(incoming *order.Order) ([]*Trade, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	market, ok := e.GetMarket(incoming.Market)
	if !ok {
		return nil, errors.New("market not found")
	}

	if incoming.Side == order.SideBuy {
		cost := incoming.Price.Mul(incoming.Amount)
		if err := e.balances.LockBalance(incoming.UserID, market.Money, cost); err != nil {
			return nil, err
		}
	} else {
		if err := e.balances.LockBalance(incoming.UserID, market.Stock, incoming.Amount); err != nil {
			return nil, err
		}
	}

	if e.producer != nil {
		e.producer.SendOrderEventAsync(OrderEventPut, incoming)
	}

	ob := e.GetOrCreateOrderBook(incoming.Market)
	return e.match(ob, incoming)
}

func (e *Engine) match(ob *order.OrderBook, incoming *order.Order) ([]*Trade, error) {
	var trades []*Trade

	if incoming.Side == order.SideBuy {
		trades = e.matchAsTaker(incoming, ob.Asks, ob)
	} else {
		trades = e.matchAsTaker(incoming, ob.Bids, ob)
	}

	if incoming.Deal.LessThan(incoming.Amount) {
		incoming.Status = order.OrderStatusPartial
		if incoming.Deal.IsZero() {
			incoming.Status = order.OrderStatusPending
		}
		ob.Add(incoming)
	} else {
		incoming.Status = order.OrderStatusFilled
		now := time.Now()
		incoming.FinishedAt = &now
	}

	return trades, nil
}

func (e *Engine) matchAsTaker(incoming *order.Order, makerQueue *order.PriceQueue, ob *order.OrderBook) []*Trade {
	var trades []*Trade

	for makerQueue.Len() > 0 && incoming.Deal.LessThan(incoming.Amount) {
		maker := makerQueue.Orders[0]

		if incoming.Side == order.SideBuy && maker.Price.GreaterThan(incoming.Price) {
			break
		}
		if incoming.Side == order.SideSell && maker.Price.LessThan(incoming.Price) {
			break
		}

		remainingAmount := incoming.Amount.Sub(incoming.Deal)
		makerRemaining := maker.Amount.Sub(maker.Deal)
		tradeAmount := decimal.Min(remainingAmount, makerRemaining)
		tradePrice := maker.Price

		trade := &Trade{
			ID:           e.idGenerator.NextID(),
			TakerOrderID: incoming.ID,
			MakerOrderID: maker.ID,
			TakerUserID:  incoming.UserID,
			MakerUserID:  maker.UserID,
			Market:       incoming.Market,
			Side:         incoming.Side,
			Price:        tradePrice,
			Amount:       tradeAmount,
			TakerFee:     tradeAmount.Mul(TakerFeeRate),
			MakerFee:     tradeAmount.Mul(MakerFeeRate),
			CreatedAt:    time.Now(),
		}
		trades = append(trades, trade)

		incoming.Deal = incoming.Deal.Add(tradeAmount)
		maker.Deal = maker.Deal.Add(tradeAmount)

		e.settleTrade(trade)

		if maker.Deal.GreaterThanOrEqual(maker.Amount) {
			heap.Pop(makerQueue)
			ob.Remove(maker.ID)
		}
	}

	return trades
}

func (e *Engine) settleTrade(trade *Trade) {
	market, ok := e.GetMarket(trade.Market)
	if !ok {
		return
	}

	if e.producer != nil {
		e.producer.SendDealEventAsync(trade)
	}

	if trade.Side == order.SideBuy {
		cost := trade.Price.Mul(trade.Amount)
		e.balances.DeductBalance(trade.TakerUserID, market.Money, cost)
		e.balances.AddBalance(trade.MakerUserID, market.Money, cost.Sub(trade.MakerFee))
		e.balances.UnlockBalance(trade.TakerUserID, market.Money, cost)

		if e.producer != nil {
			e.producer.SendBalanceUpdateAsync(trade.TakerUserID, market.Money, cost.Neg())
			e.producer.SendBalanceUpdateAsync(trade.MakerUserID, market.Money, cost.Sub(trade.MakerFee))
		}
	} else {
		e.balances.DeductBalance(trade.TakerUserID, market.Stock, trade.Amount)
		e.balances.AddBalance(trade.MakerUserID, market.Stock, trade.Amount.Sub(trade.MakerFee))
		e.balances.UnlockBalance(trade.TakerUserID, market.Stock, trade.Amount)

		if e.producer != nil {
			e.producer.SendBalanceUpdateAsync(trade.TakerUserID, market.Stock, trade.Amount.Neg())
			e.producer.SendBalanceUpdateAsync(trade.MakerUserID, market.Stock, trade.Amount.Sub(trade.MakerFee))
		}
	}
}

func (e *Engine) CancelOrder(orderID int64, market string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ob, ok := e.orderBooks[market]
	if !ok {
		return nil
	}

	ord, ok := ob.GetOrder(orderID)
	if !ok {
		return nil
	}

	if ord.Status != order.OrderStatusPending && ord.Status != order.OrderStatusPartial {
		return nil
	}

	ob.Remove(orderID)

	marketConfig, ok := e.GetMarket(ord.Market)
	if !ok {
		return nil
	}

	remaining := ord.Amount.Sub(ord.Deal)
	if ord.Side == order.SideBuy {
		e.balances.UnlockBalance(ord.UserID, marketConfig.Money, ord.Price.Mul(remaining))
	} else {
		e.balances.UnlockBalance(ord.UserID, marketConfig.Stock, remaining)
	}

	now := time.Now()
	ord.Status = order.OrderStatusCancelled
	ord.FinishedAt = &now

	return nil
}
