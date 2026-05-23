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
		var cacheKeys map[string]struct{}

		if result, err := s.Get(ctx, tagKey); err == nil {
			if m, ok := result.(map[string]struct{}); ok {
				cacheKeys = m
			}
		}

		s.mu.RLock()
		if _, exists := cacheKeys[key.(string)]; exists {
			s.mu.RUnlock()
			continue
		}
		s.mu.RUnlock()

		if cacheKeys == nil {
			cacheKeys = make(map[string]struct{})
		}

		s.mu.Lock()
		cacheKeys[key.(string)] = struct{}{}
		s.mu.Unlock()

		s.client.Set(tagKey, cacheKeys, 720*time.Hour)
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

			s.mu.RLock()
			for cacheKey := range cacheKeys {
				_ = s.Delete(ctx, cacheKey)
			}
			s.mu.RUnlock()
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
