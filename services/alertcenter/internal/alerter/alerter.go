package alerter

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Alerter struct {
	redisClient redis.Cmdable
}

func NewAlerter(client redis.Cmdable) *Alerter {
	return &Alerter{
		redisClient: client,
	}
}

func (a *Alerter) SendAlert(ctx context.Context, message string) error {
	return a.redisClient.RPush(ctx, "alert:message", message).Err()
}

func (a *Alerter) GetAlerts(ctx context.Context, limit int64) ([]string, error) {
	return a.redisClient.LRange(ctx, "alert:message", 0, limit-1).Result()
}
