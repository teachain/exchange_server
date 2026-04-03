package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	*redis.Client
}

func New(addr string, password string, db int) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &Redis{client}, nil
}
