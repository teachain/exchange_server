package kafka

import (
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
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
	producer      sarama.SyncProducer
}

func NewProducer(brokers []string, ordersTopic, dealsTopic, balancesTopic string) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		ordersTopic:   ordersTopic,
		dealsTopic:    dealsTopic,
		balancesTopic: balancesTopic,
		producer:      producer,
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

	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: p.ordersTopic,
		Key:   sarama.StringEncoder(string(rune(order.ID))),
		Value: sarama.ByteEncoder(data),
	})
	return err
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

	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: p.dealsTopic,
		Key:   sarama.StringEncoder(string(rune(trade.ID))),
		Value: sarama.ByteEncoder(data),
	})
	return err
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

	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: p.balancesTopic,
		Key:   sarama.StringEncoder(string(rune(userID))),
		Value: sarama.ByteEncoder(data),
	})
	return err
}

func (p *Producer) Close() error {
	return p.producer.Close()
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
