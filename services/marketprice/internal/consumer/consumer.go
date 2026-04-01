package consumer

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
)

type Deal struct {
	ID        int64  `json:"id"`
	Market    string `json:"market"`
	Price     string `json:"price"`
	Amount    string `json:"amount"`
	Side      int    `json:"side"`
	CreatedAt int64  `json:"created_at"`
}

type DealConsumer struct {
	consumer sarama.Consumer
	handler  func(*Deal)
	redis    *redis.Client
}

func NewDealConsumer(brokers []string, group string, handler func(*Deal), redisAddr string) (*DealConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		return nil, err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &DealConsumer{
		consumer: consumer,
		handler:  handler,
		redis:    redisClient,
	}, nil
}

func (c *DealConsumer) GetLastOffset() (int64, error) {
	ctx := context.Background()
	val, err := c.redis.Get(ctx, "k:offset").Int64()
	if err == redis.Nil {
		return -1, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (c *DealConsumer) SaveOffset(offset int64) error {
	ctx := context.Background()
	return c.redis.Set(ctx, "k:offset", offset, 0).Err()
}

func (c *DealConsumer) Start(topic string) error {
	offset, err := c.GetLastOffset()
	if err != nil {
		return err
	}

	if offset == -1 {
		offset = sarama.OffsetNewest
	}

	partitionConsumer, err := c.consumer.ConsumePartition(topic, 0, offset)
	if err != nil {
		return err
	}

	go func() {
		for msg := range partitionConsumer.Messages() {
			var deal Deal
			if err := json.Unmarshal(msg.Value, &deal); err != nil {
				log.Printf("unmarshal deal failed: %v", err)
				continue
			}
			c.handler(&deal)
			if err := c.SaveOffset(msg.Offset + 1); err != nil {
				log.Printf("save offset failed: %v", err)
			}
		}
	}()

	return nil
}
