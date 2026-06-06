package memstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/jellydator/ttlcache/v3"
)

const StoreType = "ttlcache"

// 缓存指标命名约定：所有缓存相关指标统一使用 cache.<subsystem>.<metric> 前缀。
//
// 当前各子系统的指标前缀：
//   - cache.core.*      — framework/cache 通用缓存（hit, miss, set, evict）
//   - cache.dbcache.*   — framework/dbcache 数据库缓存（hit, miss, renew, write, error_cache.hit, operation.duration）
//   - cache.httpcache.* — http/middleware/internal/httpcache HTTP 响应缓存（hit, miss, write, error）
//
// 新增缓存模块的指标应遵循此命名规范，确保监控系统能够通过 cache.* 前缀统一聚合。

// ttlCacheStore implements store.StoreInterface using jellydator/ttlcache as the backend.
// Replaces the unmaintained patrickmn/go-cache store adapter.
type ttlCacheStore struct {
	mu      sync.RWMutex
	client  *ttlcache.Cache[string, any]
	options *store.Options
}

// NewTTLCache 创建基于 jellydator/ttlcache 的 gocache store 适配器
func NewTTLCache(client *ttlcache.Cache[string, any], options ...store.Option) store.StoreInterface {
	return &ttlCacheStore{
		client:  client,
		options: store.ApplyOptions(options...),
	}
}

func (s *ttlCacheStore) Get(_ context.Context, key any) (any, error) {
	item := s.client.Get(key.(string))
	if item == nil || item.IsExpired() {
		return nil, store.NotFoundWithCause(errors.New("value not found in ttlcache store"))
	}
	return item.Value(), nil
}

func (s *ttlCacheStore) GetWithTTL(_ context.Context, key any) (any, time.Duration, error) {
	item := s.client.Get(key.(string))
	if item == nil || item.IsExpired() {
		return nil, 0, store.NotFoundWithCause(errors.New("value not found in ttlcache store"))
	}
	return item.Value(), item.TTL(), nil
}

func (s *ttlCacheStore) Set(ctx context.Context, key any, value any, options ...store.Option) error {
	opts := store.ApplyOptions(options...)
	if opts == nil {
		opts = s.options
	}

	ttl := opts.Expiration
	if ttl == 0 {
		ttl = ttlcache.NoTTL
	}
	s.client.Set(key.(string), value, ttl)

	if tags := opts.Tags; len(tags) > 0 {
		s.setTags(ctx, key, tags)
	}

	return nil
}

func (s *ttlCacheStore) setTags(ctx context.Context, key any, tags []string) {
	for _, tag := range tags {
		tagKey := fmt.Sprintf("gocache_tag_%s", tag)

		s.mu.Lock()
		var cacheKeys map[string]struct{}
		if result, err := s.Get(ctx, tagKey); err == nil {
			if m, ok := result.(map[string]struct{}); ok {
				// Copy-on-write: create a new map to avoid modifying shared reference
				newKeys := make(map[string]struct{}, len(m)+1)
				for k, v := range m {
					newKeys[k] = v
				}
				cacheKeys = newKeys
			}
		}
		if cacheKeys == nil {
			cacheKeys = make(map[string]struct{})
		}
		cacheKeys[key.(string)] = struct{}{}
		s.client.Set(tagKey, cacheKeys, 720*time.Hour)
		s.mu.Unlock()
	}
}

func (s *ttlCacheStore) Delete(_ context.Context, key any) error {
	s.client.Delete(key.(string))
	return nil
}

func (s *ttlCacheStore) Invalidate(ctx context.Context, options ...store.InvalidateOption) error {
	opts := store.ApplyInvalidateOptions(options...)

	if tags := opts.Tags; len(tags) > 0 {
		for _, tag := range tags {
			tagKey := fmt.Sprintf("gocache_tag_%s", tag)
			result, err := s.Get(ctx, tagKey)
			if err != nil {
				return nil
			}

			var cacheKeys map[string]struct{}
			if m, ok := result.(map[string]struct{}); ok {
				cacheKeys = m
			}

			s.mu.Lock()
			for cacheKey := range cacheKeys {
				_ = s.Delete(ctx, cacheKey)
			}
			s.mu.Unlock()
		}
	}

	return nil
}

func (s *ttlCacheStore) Clear(_ context.Context) error {
	s.client.DeleteAll()
	return nil
}

func (s *ttlCacheStore) GetType() string {
	return StoreType
}

// ItemCount returns the number of items in the cache, for use with WithItemCountFunc.
func ItemCount(client *ttlcache.Cache[string, any]) func() int {
	return func() int {
		return len(client.Items())
	}
}
