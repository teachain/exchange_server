package balance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrDuplicateUpdate = errors.New("duplicate update rejected")

type UpdateManager struct {
	redisClient redis.Cmdable
	ttl         time.Duration
}

func NewUpdateManager(redisClient redis.Cmdable) *UpdateManager {
	return &UpdateManager{
		redisClient: redisClient,
		ttl:         24 * time.Hour,
	}
}

func (m *UpdateManager) key(userID uint32, asset, business, businessID string) string {
	return fmt.Sprintf("update:%d:%s:%s:%s", userID, asset, business, businessID)
}

func (m *UpdateManager) UpdateBalance(ctx context.Context, userID uint32, asset, business, businessID string, change int64) error {
	k := m.key(userID, asset, business, businessID)

	exists, err := m.redisClient.Exists(ctx, k).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return ErrDuplicateUpdate
	}

	err = m.redisClient.Set(ctx, k, time.Now().Unix(), m.ttl).Err()
	if err != nil {
		return err
	}

	return nil
}
