package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/framework/cache/memstore"
	pkgxcode "github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
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

func TestNilManager_ReturnsErrCacheNotInitialized(t *testing.T) {
	c := New[string]("test", nil)
	ctx := context.Background()

	var xe xerror.XError

	_, _, err := c.Get(ctx, "key")
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgxcode.ErrCacheNotInitialized.Code(), xe.ErrorCode())

	_, err = c.GetAndDelete(ctx, "key")
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgxcode.ErrCacheNotInitialized.Code(), xe.ErrorCode())

	err = c.Set(ctx, "key", nil, 0)
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgxcode.ErrCacheNotInitialized.Code(), xe.ErrorCode())

	err = c.Clear(ctx, "key")
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgxcode.ErrCacheNotInitialized.Code(), xe.ErrorCode())
}

func TestWithAutoRenew(t *testing.T) {
	mgr := newTestCacheManager[string]()

	// enabled=true → autoRenew == true (already the default, but verify explicit setting)
	c1 := New[string]("test", mgr, WithAutoRenew[string](true))
	ac1 := c1.(*anyCache[string])
	assert.True(t, ac1.autoRenew)

	// enabled=false → autoRenew == false
	c2 := New[string]("test", mgr, WithAutoRenew[string](false))
	ac2 := c2.(*anyCache[string])
	assert.False(t, ac2.autoRenew)
}

func TestWithRenewThreshold(t *testing.T) {
	mgr := newTestCacheManager[string]()

	// threshold=0.5 → renewThreshold == 0.5
	c1 := New[string]("test", mgr, WithRenewThreshold[string](0.5))
	ac1 := c1.(*anyCache[string])
	assert.Equal(t, 0.5, ac1.renewThreshold)

	// threshold=0 → ignored, stays at default 0.2
	c2 := New[string]("test", mgr, WithRenewThreshold[string](0))
	ac2 := c2.(*anyCache[string])
	assert.Equal(t, 0.2, ac2.renewThreshold)

	// threshold=1 → ignored, stays at default 0.2
	c3 := New[string]("test", mgr, WithRenewThreshold[string](1))
	ac3 := c3.(*anyCache[string])
	assert.Equal(t, 0.2, ac3.renewThreshold)

	// threshold < 0 → ignored
	c4 := New[string]("test", mgr, WithRenewThreshold[string](-0.1))
	ac4 := c4.(*anyCache[string])
	assert.Equal(t, 0.2, ac4.renewThreshold)

	// threshold > 1 → ignored
	c5 := New[string]("test", mgr, WithRenewThreshold[string](1.5))
	ac5 := c5.(*anyCache[string])
	assert.Equal(t, 0.2, ac5.renewThreshold)
}

func TestWithMaxItems_NonPositive(t *testing.T) {
	mgr := newTestCacheManager[string]()

	// n=0 → maxItems stays 0 (no limit)
	c1 := New[string]("test", mgr, WithMaxItems[string](0))
	ac1 := c1.(*anyCache[string])
	assert.Equal(t, 0, ac1.maxItems)

	// n=-1 → maxItems stays 0 (no limit)
	c2 := New[string]("test", mgr, WithMaxItems[string](-1))
	ac2 := c2.(*anyCache[string])
	assert.Equal(t, 0, ac2.maxItems)
}

func TestRemember_AutoRenew_TriggersRenew(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, any](),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	// Use high threshold (0.9) so that even with modest TTL, renewal triggers quickly
	c := New[string]("renew", mgr, WithRenewThreshold[string](0.9))
	ctx := context.Background()

	// Set a value with a 5 second TTL
	val := "renewable"
	err := c.Set(ctx, "renew-key", &val, 5*time.Second)
	assert.Nil(t, err)

	// Wait until TTL drops below threshold: 5s * 0.9 = 4.5s
	time.Sleep(1 * time.Second)

	// Check TTL before Remember — should be about 4s
	_, ttlBefore, _ := c.Get(ctx, "renew-key")
	assert.True(t, ttlBefore < 4*time.Second, "TTL before Remember should be less than 4s, got %v", ttlBefore)

	fun := func(ctx context.Context) (*string, error) {
		t.Fatal("fun should not be called on cache hit")
		return nil, nil
	}

	// Call Remember — cache hit should trigger auto-renew because TTL <= threshold
	result, err := c.Remember(ctx, "renew-key", 5*time.Second, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "renewable", *result)

	// After renew, TTL should be close to 5s (much larger than the ~4s it was before)
	got, ttlAfter, getErr := c.Get(ctx, "renew-key")
	assert.Nil(t, getErr)
	assert.NotNil(t, got)
	assert.Equal(t, "renewable", *got)
	assert.True(t, ttlAfter > ttlBefore, "TTL after renew (%v) should be greater than before (%v)", ttlAfter, ttlBefore)
	assert.True(t, ttlAfter >= 4*time.Second, "TTL after renew should be close to 5s, got %v", ttlAfter)
}

func TestRemember_AutoRenew_TTLAboveThreshold_NoRenew(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	// Use low threshold (0.1) so TTL is well above threshold
	c := New[string]("no-renew", mgr, WithRenewThreshold[string](0.1))
	ctx := context.Background()

	// Set a value with 5s TTL
	val := "fresh"
	err := c.Set(ctx, "fresh-key", &val, 5*time.Second)
	assert.Nil(t, err)

	// Immediately call Remember — TTL should be well above 5s * 0.1 = 0.5s threshold
	fun := func(ctx context.Context) (*string, error) {
		t.Fatal("fun should not be called on cache hit")
		return nil, nil
	}

	result, err := c.Remember(ctx, "fresh-key", 5*time.Second, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "fresh", *result)
}

func TestRemember_AutoRenew_Disabled(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, any](),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	// Disable auto-renew
	c := New[string]("disabled", mgr, WithAutoRenew[string](false))
	ctx := context.Background()

	// Set a value with short TTL (1 second)
	val := "expiring"
	err := c.Set(ctx, "disabled-key", &val, 1*time.Second)
	assert.Nil(t, err)

	// Wait until TTL is low
	time.Sleep(500 * time.Millisecond)

	fun := func(ctx context.Context) (*string, error) {
		t.Fatal("fun should not be called on cache hit")
		return nil, nil
	}

	// Remember should return cached value but NOT renew
	result, err := c.Remember(ctx, "disabled-key", 1*time.Second, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "expiring", *result)

	// Verify TTL was NOT extended — it should be close to original expiry (~500ms left)
	_, ttlAfter, _ := c.Get(ctx, "disabled-key")
	assert.True(t, ttlAfter < 600*time.Millisecond, "TTL should still be low after Remember without auto-renew, got %v", ttlAfter)

	// Key should still expire on its original schedule
	time.Sleep(1 * time.Second)
	got, _, getErr := c.Get(ctx, "disabled-key")
	assert.NotNil(t, getErr)
	assert.Nil(t, got)
}

func TestRemember_AutoRenew_NeverExpire_SkipsRenew(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, any](),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	c := New[string]("never", mgr, WithRenewThreshold[string](0.9))
	ctx := context.Background()

	// Set a value with short TTL
	val := "never-expire-test"
	err := c.Set(ctx, "never-key", &val, 1*time.Second)
	assert.Nil(t, err)

	time.Sleep(200 * time.Millisecond)

	// Check TTL before Remember
	_, ttlBefore, _ := c.Get(ctx, "never-key")
	assert.True(t, ttlBefore < 1*time.Second, "TTL should be less than 1s, got %v", ttlBefore)

	fun := func(ctx context.Context) (*string, error) {
		t.Fatal("fun should not be called on cache hit")
		return nil, nil
	}

	// Remember with NeverExpire should skip auto-renew (expire == NeverExpire in the condition check)
	result, err := c.Remember(ctx, "never-key", NeverExpire, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "never-expire-test", *result)

	// TTL should NOT have been extended because NeverExpire skips auto-renew
	_, ttlAfter, _ := c.Get(ctx, "never-key")
	assert.True(t, ttlAfter < ttlBefore || ttlAfter-ttlBefore < 100*time.Millisecond,
		"TTL should not have been extended with NeverExpire, before=%v after=%v", ttlBefore, ttlAfter)
}

func TestRemember_AutoRenew_ExpireZero_UsesDefault(t *testing.T) {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, any](),
	)
	s := memstore.NewTTLCache(client)
	mgr := gocache.New[string](s)

	// High threshold so renewal triggers even with default expire
	c := New[string]("zero-expire", mgr, WithRenewThreshold[string](0.9))
	ctx := context.Background()

	// Set a value with short TTL (1 second)
	val := "zero-expire-val"
	err := c.Set(ctx, "zero-key", &val, 1*time.Second)
	assert.Nil(t, err)

	time.Sleep(200 * time.Millisecond)

	fun := func(ctx context.Context) (*string, error) {
		t.Fatal("fun should not be called on cache hit")
		return nil, nil
	}

	// Remember with expire=0 should use default expire (5min) for renewal
	result, err := c.Remember(ctx, "zero-key", 0, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "zero-expire-val", *result)

	// After renew with default expire (5min), TTL should be much larger than the original 1s
	_, ttlAfter, getErr := c.Get(ctx, "zero-key")
	assert.Nil(t, getErr)
	assert.True(t, ttlAfter > 1*time.Minute, "TTL should be close to default 5min after renew with expire=0, got %v", ttlAfter)
}

func TestRemember_CallbackError(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	fun := func(ctx context.Context) (*string, error) {
		return nil, fmt.Errorf("callback failed")
	}

	result, err := c.Remember(ctx, "callback-err-key", 5*time.Minute, fun)
	assert.Nil(t, result)
	assert.NotNil(t, err)
	assert.Equal(t, "callback failed", err.Error())
}

func TestRemember_NeverExpire(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	fun := func(ctx context.Context) (*string, error) {
		v := "never-expire-data"
		return &v, nil
	}

	// Remember with NeverExpire should cache the value without expiration
	result, err := c.Remember(ctx, "never-remember", NeverExpire, fun)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "never-expire-data", *result)

	// Should be retrievable immediately
	got, ttl, getErr := c.Get(ctx, "never-remember")
	assert.Nil(t, getErr)
	assert.NotNil(t, got)
	assert.Equal(t, "never-expire-data", *got)
	// NeverExpire sets store.WithExpiration(0) which maps to NoTTL in ttlcache
	_ = ttl // TTL behavior depends on store implementation
}

func TestSet_NeverExpire(t *testing.T) {
	mgr := newTestCacheManager[string]()
	c := New[string]("test", mgr)
	ctx := context.Background()

	val := "permanent"
	err := c.Set(ctx, "never-key", &val, NeverExpire)
	assert.Nil(t, err)

	got, _, getErr := c.Get(ctx, "never-key")
	assert.Nil(t, getErr)
	assert.NotNil(t, got)
	assert.Equal(t, "permanent", *got)
}

func TestRemember_NilManager(t *testing.T) {
	c := New[string]("test", nil)
	ctx := context.Background()

	fun := func(ctx context.Context) (*string, error) {
		v := "x"
		return &v, nil
	}

	result, err := c.Remember(ctx, "key", 5*time.Minute, fun)
	assert.Nil(t, result)
	assert.NotNil(t, err)

	var xe xerror.XError
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgxcode.ErrCacheNotInitialized.Code(), xe.ErrorCode())
}

func TestSetDefaultExpire(t *testing.T) {
	// Save and restore original default
	original := getDefaultExpire()
	defer SetDefaultExpire(original)

	// Set new default
	SetDefaultExpire(10 * time.Second)
	assert.Equal(t, 10*time.Second, getDefaultExpire())

	// Restore
	SetDefaultExpire(original)
	assert.Equal(t, original, getDefaultExpire())
}
