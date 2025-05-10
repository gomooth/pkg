package cache

import (
	"context"
	"time"
)

type ICache[T any] interface {
	// Get 获取缓存，返回缓存数据的 json 字符串，ttl, 和错误
	Get(ctx context.Context, key string) (*T, time.Duration, error)
	// Pull 获取缓存，并删除缓存
	Pull(ctx context.Context, key string) (*T, error)
	// Set 设置缓存
	Set(ctx context.Context, key string, val *T, expire time.Duration) error
	// Remember 如果缓存不存在，则通过 fun 函数获取数据，并缓存。该函数返回 缓存数据和错误
	Remember(ctx context.Context, key string, expire time.Duration, fun func(ctx context.Context) (*T, error)) (*T, error)
	// Clear 清理缓存
	Clear(ctx context.Context, key string) error
}
