package cache

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"

	"github.com/gomooth/utils/strutil"

	"github.com/save95/xerror"
)

var single singleflight.Group

type anyCache[T any] struct {
	cacheManager *cache.Cache[T]
	name         string
}

func New[T any](nameSpace string, cacheManager *cache.Cache[T]) ICache[T] {
	return &anyCache[T]{
		name:         nameSpace,
		cacheManager: cacheManager,
	}
}

func (c *anyCache[T]) getKey(key string) string {
	return fmt.Sprintf("%s:%s", strutil.Camel(c.name), key)
}

func (c *anyCache[T]) Get(ctx context.Context, key string) (*T, time.Duration, error) {
	if c.cacheManager == nil {
		return nil, 0, xerror.New("cache manager no init")
	}

	key = c.getKey(key)
	cacheData, d, err := c.cacheManager.GetWithTTL(ctx, key)
	if nil == err {
		return &cacheData, d, nil
	}

	return nil, 0, err
}

func (c *anyCache[T]) Pull(ctx context.Context, key string) (*T, error) {
	if c.cacheManager == nil {
		return nil, xerror.New("cache manager no init")
	}

	key = c.getKey(key)
	cacheData, err := c.cacheManager.Get(ctx, key)
	if nil == err {
		if err := c.cacheManager.Delete(ctx, key); nil != err {
			return nil, err
		}

		return &cacheData, nil
	}

	return nil, err
}

func (c *anyCache[T]) Set(ctx context.Context, key string, val *T, expire time.Duration) error {
	if c.cacheManager == nil {
		return xerror.New("cache manager no init")
	}

	// 禁止设置永久缓存
	if expire == 0 {
		expire = 5 * time.Minute
	}

	key = c.getKey(key)
	return c.cacheManager.Set(ctx, key, *val, store.WithExpiration(expire))
}

func (c *anyCache[T]) Remember(
	ctx context.Context,
	key string,
	expire time.Duration,
	fun func(ctx context.Context) (*T, error),
) (*T, error) {
	if c.cacheManager == nil {
		return nil, xerror.New("cache manager no init")
	}

	key = c.getKey(key)
	cd, err := c.cacheManager.Get(ctx, key)
	if nil == err {
		return &cd, nil
	}

	v, err, _ := single.Do(key, func() (interface{}, error) {
		return fun(ctx)
	})
	if nil != err {
		return nil, err
	}

	data := v.(*T)

	// 保存缓存
	if err = c.Set(ctx, key, data, expire); nil != err {
		return nil, err
	}

	return data, nil
}

func (c *anyCache[T]) Clear(ctx context.Context, key string) error {
	if c.cacheManager == nil {
		return xerror.New("cache manager no init")
	}

	key = c.getKey(key)
	return c.cacheManager.Delete(ctx, key)
}
