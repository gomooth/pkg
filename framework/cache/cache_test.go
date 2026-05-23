package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/framework/cache/memstore"
)

func newTestCacheManager[T any]() *gocache.Cache[T] {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	return gocache.New[T](s)
}

func TestNew(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	assert.NotNil(t, c)
}

func TestGetAndSet(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	// Set a value
	val := "hello"
	err := c.Set(ctx, "key1", &val, 5*time.Minute)
	assert.Nil(t, err)

	// Get the value
	got, ttl, err := c.Get(ctx, "key1")
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "hello", *got)
	assert.True(t, ttl > 0)

	// Get a non-existent key
	got2, ttl2, err := c.Get(ctx, "nonexistent")
	assert.NotNil(t, err)
	assert.Nil(t, got2)
	assert.Equal(t, time.Duration(0), ttl2)
}

func TestSet_DefaultExpiration(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	// Set with expire=0 should use defaultExpire (5min)
	val := "default-expire"
	err := c.Set(ctx, "key_default", &val, 0)
	assert.Nil(t, err)

	// Should be able to retrieve it
	got, _, err := c.Get(ctx, "key_default")
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "default-expire", *got)
}

func TestGetAndDelete(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	// Set a value
	val := "pull-value"
	err := c.Set(ctx, "key_pull", &val, 5*time.Minute)
	assert.Nil(t, err)

	// GetAndDelete should return the value
	got, err := c.GetAndDelete(ctx, "key_pull")
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "pull-value", *got)

	// After GetAndDelete, the key should be gone
	got2, _, err := c.Get(ctx, "key_pull")
	assert.NotNil(t, err)
	assert.Nil(t, got2)

	// GetAndDelete non-existent key should return error
	got3, err := c.GetAndDelete(ctx, "nonexistent")
	assert.NotNil(t, err)
	assert.Nil(t, got3)
}

func TestRemember_CacheMiss(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	callCount := 0
	fun := func(ctx context.Context) (*string, error) {
		callCount++
		v := fmt.Sprintf("computed-%d", callCount)
		return &v, nil
	}

	// First call: cache miss, should call fun
	result, err := c.Remember(ctx, "remember_key", 5*time.Minute, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "computed-1", *result)
	assert.Equal(t, 1, callCount)
}

func TestRemember_CacheHit(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	// Pre-set the cache value
	val := "already-cached"
	err := c.Set(ctx, "remember_hit", &val, 5*time.Minute)
	assert.Nil(t, err)

	callCount := 0
	fun := func(ctx context.Context) (*string, error) {
		callCount++
		v := "should-not-be-used"
		return &v, nil
	}

	// Should return cached value without calling fun
	result, err := c.Remember(ctx, "remember_hit", 5*time.Minute, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "already-cached", *result)
	assert.Equal(t, 0, callCount)
}

func TestRemember_CacheMiss_StoresResult(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	callCount := 0
	fun := func(ctx context.Context) (*string, error) {
		callCount++
		v := "computed-value"
		return &v, nil
	}

	// First call: cache miss
	result, err := c.Remember(ctx, "remember_store", 5*time.Minute, fun)
	assert.Nil(t, err)
	assert.Equal(t, "computed-value", *result)
	assert.Equal(t, 1, callCount)

	// Second call: should use cache, fun should not be called again
	result2, err := c.Remember(ctx, "remember_store", 5*time.Minute, fun)
	assert.Nil(t, err)
	assert.Equal(t, "computed-value", *result2)
	assert.Equal(t, 1, callCount)
}

func TestRemember_FunError(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	fun := func(ctx context.Context) (*string, error) {
		return nil, fmt.Errorf("fun error")
	}

	result, err := c.Remember(ctx, "remember_err", 5*time.Minute, fun)
	assert.NotNil(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "fun error", err.Error())
}

func TestClear(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	// Set a value
	val := "clear-me"
	err := c.Set(ctx, "key_clear", &val, 5*time.Minute)
	assert.Nil(t, err)

	// Clear the key
	err = c.Clear(ctx, "key_clear")
	assert.Nil(t, err)

	// Key should be gone
	got, _, err := c.Get(ctx, "key_clear")
	assert.NotNil(t, err)
	assert.Nil(t, got)
}

func TestNilCacheManager(t *testing.T) {
	c := New[string]("test", nil)
	ctx := context.Background()

	// Get with nil manager
	got, ttl, err := c.Get(ctx, "key")
	assert.NotNil(t, err)
	assert.Nil(t, got)
	assert.Equal(t, time.Duration(0), ttl)

	// Set with nil manager
	val := "test"
	err = c.Set(ctx, "key", &val, 5*time.Minute)
	assert.NotNil(t, err)

	// GetAndDelete with nil manager
	got, err = c.GetAndDelete(ctx, "key")
	assert.NotNil(t, err)
	assert.Nil(t, got)

	// Remember with nil manager
	fun := func(ctx context.Context) (*string, error) { v := "x"; return &v, nil }
	got, err = c.Remember(ctx, "key", 5*time.Minute, fun)
	assert.NotNil(t, err)
	assert.Nil(t, got)

	// Clear with nil manager
	err = c.Clear(ctx, "key")
	assert.NotNil(t, err)
}

func TestKeyNamespacing(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c1 := New[string]("namespace1", mgr)
	c2 := New[string]("namespace2", mgr)
	ctx := context.Background()

	val1 := "value1"
	val2 := "value2"

	err := c1.Set(ctx, "samekey", &val1, 5*time.Minute)
	assert.Nil(t, err)

	err = c2.Set(ctx, "samekey", &val2, 5*time.Minute)
	assert.Nil(t, err)

	// Same key name but different namespaces should not collide
	got1, _, err := c1.Get(ctx, "samekey")
	assert.Nil(t, err)
	assert.Equal(t, "value1", *got1)

	got2, _, err := c2.Get(ctx, "samekey")
	assert.Nil(t, err)
	assert.Equal(t, "value2", *got2)
}

func TestSetAndGet_IntType(t *testing.T) {
	mgr := newTestCacheManager[int]()
	c := New[int]("test", mgr)
	ctx := context.Background()

	val := 42
	err := c.Set(ctx, "intkey", &val, 5*time.Minute)
	assert.Nil(t, err)

	got, _, err := c.Get(ctx, "intkey")
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, 42, *got)
}

func TestWithMaxItems_RejectsNewItem(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	c := New[string]("cap", mgr,
		WithMaxItems[string](2),
		WithItemCountFunc[string](memstore.ItemCount(client)),
	)
	ctx := context.Background()

	// 添加 2 个 key，应该成功
	val1 := "v1"
	err := c.Set(ctx, "k1", &val1, 5*time.Minute)
	assert.Nil(t, err)
	val2 := "v2"
	err = c.Set(ctx, "k2", &val2, 5*time.Minute)
	assert.Nil(t, err)

	// 第 3 个 key 应被拒绝
	val3 := "v3"
	err = c.Set(ctx, "k3", &val3, 5*time.Minute)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "capacity limit reached")
}

func TestWithMaxItems_AllowsUpdate(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	c := New[string]("cap", mgr,
		WithMaxItems[string](1),
		WithItemCountFunc[string](memstore.ItemCount(client)),
	)
	ctx := context.Background()

	// 添加 1 个 key
	val1 := "v1"
	err := c.Set(ctx, "k1", &val1, 5*time.Minute)
	assert.Nil(t, err)

	// 更新已有 key 应该成功
	val1Updated := "v1-updated"
	err = c.Set(ctx, "k1", &val1Updated, 5*time.Minute)
	assert.Nil(t, err)

	// 验证值已更新
	got, _, err := c.Get(ctx, "k1")
	assert.Nil(t, err)
	assert.Equal(t, "v1-updated", *got)

	// 新 key 应被拒绝
	val2 := "v2"
	err = c.Set(ctx, "k2", &val2, 5*time.Minute)
	assert.NotNil(t, err)
}

func TestWithoutMaxItems_NoLimit(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("nolimit", mgr)
	ctx := context.Background()

	// 无容量限制，可以随意添加
	for i := 0; i < 100; i++ {
		val := fmt.Sprintf("val-%d", i)
		err := c.Set(ctx, fmt.Sprintf("key-%d", i), &val, 5*time.Minute)
		assert.Nil(t, err)
	}
}
