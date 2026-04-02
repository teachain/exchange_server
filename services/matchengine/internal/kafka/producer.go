package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/engine"
	orderpkg "github.com/viabtc/go-project/services/matchengine/internal/order"
)

const (
	OrderEventPut    = 1
	OrderEventUpdate = 2
	OrderEventFinish = 3
)

type Producer struct {
	ordersTopic   string
	dealsTopic    string
	balancesTopic string
	writer        *kafka.Writer
}

func NewProducer(brokers []string, ordersTopic, dealsTopic, balancesTopic string) (*Producer, error) {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll,
	}

	return &Producer{
		ordersTopic:   ordersTopic,
		dealsTopic:    dealsTopic,
		balancesTopic: balancesTopic,
		writer:        writer,
	}, nil
}

func (p *Producer) SendOrderEvent(event int, order *orderpkg.Order) error {
	msg := struct {
		Event int             `json:"event"`
		Order *orderpkg.Order `json:"order"`
	}{
		Event: event,
		Order: order,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: p.ordersTopic,
		Key:   []byte(string(rune(order.ID))),
		Value: data,
	})
}

func (p *Producer) SendDealEvent(trade *engine.Trade) error {
	msg := struct {
		Trade *engine.Trade `json:"trade"`
	}{
		Trade: trade,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: p.dealsTopic,
		Key:   []byte(string(rune(trade.ID))),
		Value: data,
	})
}

func (p *Producer) SendBalanceUpdate(userID int64, asset string, change decimal.Decimal) error {
	msg := []interface{}{
		"balance_update",
		userID,
		asset,
		change.String(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: p.balancesTopic,
		Key:   []byte(string(rune(userID))),
		Value: data,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func (p *Producer) SendOrderEventAsync(event int, order *orderpkg.Order) {
	go func() {
		if err := p.SendOrderEvent(event, order); err != nil {
			log.Printf("failed to send order event: %v", err)
		}
	}()
}

func (p *Producer) SendDealEventAsync(trade *engine.Trade) {
	go func() {
		if err := p.SendDealEvent(trade); err != nil {
			log.Printf("failed to send deal event: %v", err)
		}
	}()
}

func (p *Producer) SendBalanceUpdateAsync(userID int64, asset string, change decimal.Decimal) {
	go func() {
		if err := p.SendBalanceUpdate(userID, asset, change); err != nil {
			log.Printf("failed to send balance update: %v", err)
		}
	}()
}
