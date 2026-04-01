package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/marketprice/internal/kline"
	"github.com/viabtc/go-project/services/marketprice/internal/model"
)

type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient(host string, port int, password string, db int) *redis.Client {
	addr := fmt.Sprintf("%s:%d", host, port)
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func NewRedisCache(addr string) (*RedisCache, error) {
	return NewRedisCacheWithPassword(addr, "", 0)
}

func NewRedisCacheWithPassword(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	_, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{client: client, ctx: ctx}, nil
}

func NewRedisCacheWithSentinel(masterName string, sentinelAddrs []string, password string, db int) (*RedisCache, error) {
	client := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    masterName,
		SentinelAddrs: sentinelAddrs,
		Password:      password,
		DB:            db,
	})

	ctx := context.Background()
	_, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{client: client, ctx: ctx}, nil
}

func (c *RedisCache) SaveKline(ctx context.Context, k *kline.Kline) error {
	key := c.klineKey(k.Market, k.Interval, k.Timestamp)

	data, err := json.Marshal(k)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, 0).Err()
}

func (c *RedisCache) GetKline(ctx context.Context, market string, interval kline.Interval, ts int64) (*kline.Kline, error) {
	key := c.klineKey(market, interval, ts)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var k kline.Kline
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, err
	}

	return &k, nil
}

func (c *RedisCache) klineKey(market string, interval kline.Interval, ts int64) string {
	return fmt.Sprintf("kline:%s:%s:%d", market, interval, ts)
}

func (c *RedisCache) FlushKline(market, interval string, timestamp int64, kline *model.KlineInfo) error {
	key := fmt.Sprintf("k:%s:%s", market, interval)
	field := strconv.FormatInt(timestamp, 10)
	value := fmt.Sprintf("%s,%s,%s,%s,%s,%s",
		kline.Open.String(),
		kline.Close.String(),
		kline.High.String(),
		kline.Low.String(),
		kline.Volume.String(),
		kline.Deal.String())
	return c.client.HSet(c.ctx, key, field, value).Err()
}

func (c *RedisCache) FlushDeal(market string, deal *model.Deal) error {
	key := fmt.Sprintf("k:%s:deals", market)
	data, _ := json.Marshal(deal)
	return c.client.LPush(c.ctx, key, data).Err()
}

func (c *RedisCache) FlushLastPrice(market string, price decimal.Decimal) error {
	key := fmt.Sprintf("k:%s:last", market)
	return c.client.Set(c.ctx, key, price.String(), 0).Err()
}

func (c *RedisCache) LoadKlines(market, interval string, start int64) (map[int64]*model.KlineInfo, error) {
	key := fmt.Sprintf("k:%s:%s", market, interval)
	result := make(map[int64]*model.KlineInfo)

	fields, err := c.client.HGetAll(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	for field, value := range fields {
		timestamp, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			continue
		}
		if timestamp < start {
			continue
		}

		var open, close, high, low, volume, deal decimal.Decimal
		parts := splitString(value, ",")
		if len(parts) >= 6 {
			open, _ = decimal.NewFromString(parts[0])
			close, _ = decimal.NewFromString(parts[1])
			high, _ = decimal.NewFromString(parts[2])
			low, _ = decimal.NewFromString(parts[3])
			volume, _ = decimal.NewFromString(parts[4])
			deal, _ = decimal.NewFromString(parts[5])
		}
		result[timestamp] = &model.KlineInfo{
			Open:   open,
			Close:  close,
			High:   high,
			Low:    low,
			Volume: volume,
			Deal:   deal,
		}
	}

	return result, nil
}

func (c *RedisCache) LoadDeals(market string, limit int) ([]*model.Deal, error) {
	key := fmt.Sprintf("k:%s:deals", market)
	data, err := c.client.LRange(c.ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	deals := make([]*model.Deal, 0, len(data))
	for _, item := range data {
		var deal model.Deal
		if err := json.Unmarshal([]byte(item), &deal); err != nil {
			continue
		}
		deals = append(deals, &deal)
	}

	return deals, nil
}

func (c *RedisCache) LoadLastPrice(market string) (decimal.Decimal, error) {
	key := fmt.Sprintf("k:%s:last", market)
	val, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return decimal.Zero, nil
		}
		return decimal.Zero, err
	}
	return decimal.NewFromString(val)
}

func splitString(s string, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}
