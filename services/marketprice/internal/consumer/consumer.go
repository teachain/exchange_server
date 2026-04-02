package consumer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
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
	reader  *kafka.Reader
	handler func(*Deal)
	redis   *redis.Client
}

func NewDealConsumer(brokers []string, group string, handler func(*Deal), redisAddr string, redisPassword string) (*DealConsumer, error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          "deals",
		GroupID:        group,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	return &DealConsumer{
		reader:  reader,
		handler: handler,
		redis:   redisClient,
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

func (c *DealConsumer) Start(topic string, partition int32) error {
	lastOffset, err := c.GetLastOffset()
	if err != nil {
		log.Printf("get last offset failed: %v", err)
	}

	go func() {
		if lastOffset >= 0 {
			for {
				msg, err := c.reader.ReadMessage(context.Background())
				if err != nil {
					log.Printf("skip message error: %v", err)
					break
				}
				if msg.Offset < lastOffset {
					continue
				}
				var deal Deal
				if err := json.Unmarshal(msg.Value, &deal); err != nil {
					log.Printf("unmarshal deal failed: %v", err)
					continue
				}
				c.handler(&deal)
				c.SaveOffset(msg.Offset + 1)
				break
			}
		}

		for {
			msg, err := c.reader.ReadMessage(context.Background())
			if err != nil {
				if err.Error() == "context canceled" {
					return
				}
				log.Printf("read message error: %v", err)
				continue
			}

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

func (c *DealConsumer) Close() error {
	return c.reader.Close()
}
