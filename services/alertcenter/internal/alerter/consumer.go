package alerter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type AlertMessage struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type Consumer struct {
	redisClient redis.Cmdable
	emailSender *EmailSender
	rateLimit   time.Duration
	lastSent    time.Time
	stopCh      chan struct{}
	mu          sync.Mutex
}

func NewConsumer(redisClient redis.Cmdable, emailSender *EmailSender, rateLimit time.Duration) *Consumer {
	return &Consumer{
		redisClient: redisClient,
		emailSender: emailSender,
		rateLimit:   rateLimit,
		lastSent:    time.Time{},
		stopCh:      make(chan struct{}),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	go c.blpopLoop(ctx)
}

func (c *Consumer) Stop() {
	close(c.stopCh)
}

func (c *Consumer) blpopLoop(ctx context.Context) {
	queueName := "alert:message"
	timeout := 60 * time.Second

	for {
		select {
		case <-c.stopCh:
			log.Println("Consumer stopping...")
			return
		case <-ctx.Done():
			log.Println("Consumer context cancelled...")
			return
		default:
		}

		result, err := c.redisClient.BLPop(ctx, timeout, queueName).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			log.Printf("BLPOP error: %v", err)
			continue
		}

		if len(result) < 2 {
			continue
		}

		message := result[1]
		if err := c.processMessage(ctx, message); err != nil {
			log.Printf("Failed to process message: %v", err)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, message string) error {
	if err := c.checkRateLimit(); err != nil {
		log.Printf("Rate limited: %v", err)
		return err
	}

	var alert AlertMessage
	if err := json.Unmarshal([]byte(message), &alert); err != nil {
		var rawSubject, rawBody string
		parts := strings.SplitN(message, "|", 2)
		if len(parts) == 2 {
			rawSubject = strings.TrimSpace(parts[0])
			rawBody = strings.TrimSpace(parts[1])
		} else {
			rawSubject = "Alert"
			rawBody = message
		}
		alert.Subject = rawSubject
		alert.Body = rawBody
	}

	c.mu.Lock()
	c.lastSent = time.Now()
	c.mu.Unlock()

	return c.emailSender.SendEmail(ctx, alert.Subject, alert.Body)
}

func (c *Consumer) checkRateLimit() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastSent.IsZero() {
		return nil
	}

	elapsed := time.Since(c.lastSent)
	if elapsed < c.rateLimit {
		return fmt.Errorf("rate limit: need to wait %v", c.rateLimit-elapsed)
	}

	return nil
}
