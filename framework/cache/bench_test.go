package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/jellydator/ttlcache/v3"

	"github.com/gomooth/pkg/framework/cache/memstore"
)

// ============================================================
// 辅助：构建基于内存的 cacheManager
// ============================================================

func newBenchCacheManager[T any]() *gocache.Cache[T] {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	return gocache.New[T](s)
}

// ============================================================
// BenchmarkICache_Get — 缓存命中 / 未命中
// ============================================================

func BenchmarkICache_Get_Hit(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	val := "hello-world"
	_ = c.Set(ctx, "hit-key", &val, 5*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.Get(ctx, "hit-key")
	}
}

func BenchmarkICache_Get_Miss(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.Get(ctx, "nonexistent-key")
	}
}

// ============================================================
// BenchmarkICache_Set — 缓存写入
// ============================================================

func BenchmarkICache_Set(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		val := fmt.Sprintf("value-%d", i)
		_ = c.Set(ctx, fmt.Sprintf("key-%d", i), &val, 5*time.Minute)
	}
}

func BenchmarkICache_Set_Overwrite(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	val := "initial"
	_ = c.Set(ctx, "overwrite-key", &val, 5*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updated := fmt.Sprintf("updated-%d", i)
		_ = c.Set(ctx, "overwrite-key", &updated, 5*time.Minute)
	}
}

// ============================================================
// BenchmarkICache_Remember — 缓存命中 / 未命中
// ============================================================

func BenchmarkICache_Remember_Hit(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	val := "cached-value"
	_ = c.Set(ctx, "remember-key", &val, 5*time.Minute)

	fun := func(ctx context.Context) (*string, error) {
		v := "should-not-be-called"
		return &v, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Remember(ctx, "remember-key", 5*time.Minute, fun)
	}
}

func BenchmarkICache_Remember_Miss(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fun := func(ctx context.Context) (*string, error) {
			v := fmt.Sprintf("computed-%d", i)
			return &v, nil
		}
		_, _ = c.Remember(ctx, fmt.Sprintf("miss-key-%d", i%1000), 5*time.Minute, fun)
	}
}

// ============================================================
// BenchmarkICache_GetAndDelete
// ============================================================

func BenchmarkICache_GetAndDelete(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchCacheManager[string]()
	c := New[string]("bench", mgr)
	ctx := context.Background()

	// 预填充足够多的 key
	for i := 0; i < b.N; i++ {
		val := fmt.Sprintf("val-%d", i)
		_ = c.Set(ctx, fmt.Sprintf("gad-%d", i), &val, 5*time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.GetAndDelete(ctx, fmt.Sprintf("gad-%d", i))
	}
}
