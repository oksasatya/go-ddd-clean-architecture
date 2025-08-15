package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initializes a redis client
func NewRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func RedisSetJSON(ctx context.Context, rdb *redis.Client, key string, value interface{}, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, b, ttl).Err()
}

func RedisGetJSON[T any](ctx context.Context, rdb *redis.Client, key string, dest *T) (bool, error) {
	res, err := rdb.Get(ctx, key).Bytes()
	if errors.Is(redis.Nil, err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(res, dest); err != nil {
		return false, err
	}
	return true, nil
}

func RedisDel(ctx context.Context, rdb *redis.Client, key string) error {
	return rdb.Del(ctx, key).Err()
}
