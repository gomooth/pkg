package store

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore store http response in redis
type RedisStore struct {
	RedisClient *redis.Client
	ownClient   bool // 是否由 store 内部创建的 client（需负责关闭）
}

// NewRedisStore create a redis memory store with redis client
// 调用方负责关闭 redis.Client。若需要 store 自行管理生命周期，使用 NewOwnedRedisStore。
func NewRedisStore(redisClient *redis.Client) *RedisStore {
	return &RedisStore{
		RedisClient: redisClient,
		ownClient:   false,
	}
}

// NewOwnedRedisStore 创建拥有 Redis client 生命周期的 store。
// 调用 Close() 时会关闭内部创建的 redis.Client。
func NewOwnedRedisStore(redisClient *redis.Client) *RedisStore {
	return &RedisStore{
		RedisClient: redisClient,
		ownClient:   true,
	}
}

// Close 关闭 Redis 连接。仅当 client 由 store 内部创建时才关闭，否则为空操作。
func (store *RedisStore) Close() error {
	if store.ownClient && store.RedisClient != nil {
		return store.RedisClient.Close()
	}
	return nil
}

// Set put key value pair to redis, and expire after expireDuration
func (store *RedisStore) Set(ctx context.Context, key string, value *CachedResponse, expire time.Duration) error {
	payload, err := serialize(value)
	if err != nil {
		return err
	}

	return store.RedisClient.Set(ctx, key, payload, expire).Err()
}

// Delete remove key in redis, do nothing if key doesn't exist
func (store *RedisStore) Delete(ctx context.Context, key string) error {
	return store.RedisClient.Del(ctx, key).Err()
}

// Get retrieves an item from redis, if key doesn't exist, return ErrorCacheMiss
func (store *RedisStore) Get(ctx context.Context, key string, value *CachedResponse) error {
	payload, err := store.RedisClient.Get(ctx, key).Bytes()

	if errors.Is(err, redis.Nil) {
		return ErrorCacheMiss
	}

	if err != nil {
		return err
	}
	return unserialize(payload, value)
}
