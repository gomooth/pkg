package dbcache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/jellydator/ttlcache/v3"

	"github.com/gomooth/pkg/framework/cache/memstore"
	"github.com/gomooth/pkg/framework/dbquery"
)

// ============================================================
// 辅助：构建基于内存的 cacheManager
// ============================================================

func newBenchMemoryCacheManager() *cache.Cache[string] {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	return cache.New[string](s)
}

// ============================================================
// BenchmarkICache_Remember — DB 缓存核心路径
// ============================================================

func BenchmarkICache_Remember_Hit(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)

	ctx := context.Background()
	data := []byte(`{"id":1,"name":"cached"}`)

	// 预填充缓存
	_, _ = c.Remember(ctx, "hit-key", func(_ context.Context) ([]byte, error) {
		return data, nil
	})

	fun := func(_ context.Context) ([]byte, error) {
		return []byte("should-not-be-called"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Remember(ctx, "hit-key", fun)
	}
}

func BenchmarkICache_Remember_Miss(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("miss-key-%d", i%10000)
		_, _ = c.Remember(ctx, key, func(_ context.Context) ([]byte, error) {
			return []byte(`{"id":1,"name":"computed"}`), nil
		})
	}
}

func BenchmarkICache_Remember_Hit_WithCodec(b *testing.B) {
	b.ReportAllocs()

	for _, codec := range []struct {
		name  string
		codec Codec
	}{
		{"JSON", JSONCodec{}},
		{"Msgpack", MsgpackCodec{}},
	} {
		b.Run(codec.name, func(b *testing.B) {
			mgr := newBenchMemoryCacheManager()
			c := New[benchEntity, benchFilter]("bench", mgr, WithCodec(codec.codec))
			ctx := context.Background()

			data := []byte(`{"id":1,"name":"cached"}`)
			_, _ = c.Remember(ctx, "codec-key", func(_ context.Context) ([]byte, error) {
				return data, nil
			})

			fun := func(_ context.Context) ([]byte, error) {
				return []byte("should-not-be-called"), nil
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = c.Remember(ctx, "codec-key", fun)
			}
		})
	}
}

// ============================================================
// BenchmarkICache_First — 单条缓存命中/未命中
// ============================================================

func BenchmarkICache_First_Hit(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)
	ctx := context.Background()

	entity := &benchEntity{ID: 1, Name: "hello"}
	_, _ = c.First(ctx, 1, func(_ context.Context) (*benchEntity, error) {
		return entity, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.First(ctx, 1, func(_ context.Context) (*benchEntity, error) {
			return entity, nil
		})
	}
}

func BenchmarkICache_First_Miss(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entity := &benchEntity{ID: uint(i%10000) + 1, Name: "hello"}
		_, _ = c.First(ctx, uint(i%10000)+1, func(_ context.Context) (*benchEntity, error) {
			return entity, nil
		})
	}
}

// ============================================================
// BenchmarkICache_Paginate — 分页缓存命中/未命中
// ============================================================

func BenchmarkICache_Paginate_Hit(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)
	ctx := context.Background()

	q := dbquery.NewQuery(benchFilter{Name: "foo"}, dbquery.WithOffsetPage[benchFilter](0, 10))
	records := []*benchEntity{{ID: 1, Name: "foo"}}
	total := uint(1)

	// 预填充
	_, _, _ = c.Paginate(ctx, q, func(_ context.Context) ([]*benchEntity, uint, error) {
		return records, total, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.Paginate(ctx, q, func(_ context.Context) ([]*benchEntity, uint, error) {
			return records, total, nil
		})
	}
}

func BenchmarkICache_Paginate_Miss(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := dbquery.NewQuery(benchFilter{Name: fmt.Sprintf("page-%d", i%10000)}, dbquery.WithOffsetPage[benchFilter](0, 10))
		records := []*benchEntity{{ID: 1, Name: "foo"}}
		total := uint(1)
		_, _, _ = c.Paginate(ctx, q, func(_ context.Context) ([]*benchEntity, uint, error) {
			return records, total, nil
		})
	}
}

// ============================================================
// BenchmarkICache_List — 列表缓存命中/未命中
// ============================================================

func BenchmarkICache_List_Hit(b *testing.B) {
	b.ReportAllocs()

	mgr := newBenchMemoryCacheManager()
	c := New[benchEntity, benchFilter]("bench", mgr)
	ctx := context.Background()

	q := dbquery.NewQuery(benchFilter{Name: "foo"})
	records := []*benchEntity{{ID: 1, Name: "foo"}}

	// 预填充
	_, _ = c.List(ctx, q, func(_ context.Context) ([]*benchEntity, error) {
		return records, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.List(ctx, q, func(_ context.Context) ([]*benchEntity, error) {
			return records, nil
		})
	}
}

// ============================================================
// Benchmark Codec — 编解码器性能对比
// ============================================================

func BenchmarkCodec_Marshal(b *testing.B) {
	b.ReportAllocs()

	data := &benchEntity{ID: 1, Name: "benchmark-entity"}

	codecs := []struct {
		name  string
		codec Codec
	}{
		{"JSON", JSONCodec{}},
		{"Msgpack", MsgpackCodec{}},
		{"Gob", GobCodec{}},
	}

	for _, cc := range codecs {
		b.Run(cc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = cc.codec.Marshal(data)
			}
		})
	}
}

func BenchmarkCodec_Unmarshal(b *testing.B) {
	b.ReportAllocs()

	data := &benchEntity{ID: 1, Name: "benchmark-entity"}

	codecs := []struct {
		name  string
		codec Codec
	}{
		{"JSON", JSONCodec{}},
		{"Msgpack", MsgpackCodec{}},
		{"Gob", GobCodec{}},
	}

	for _, cc := range codecs {
		b.Run(cc.name, func(b *testing.B) {
			bs, _ := cc.codec.Marshal(data)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var result benchEntity
				_ = cc.codec.Unmarshal(bs, &result)
			}
		})
	}
}

// ============================================================
// 测试类型定义
// ============================================================

type benchEntity struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type benchFilter struct {
	Name string `json:"name"`
}
