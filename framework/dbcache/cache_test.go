package dbcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/framework/cache/memstore"
	"github.com/gomooth/pkg/framework/dbquery"
	pkgXcode "github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
)

// ============================================================
// Helper: 构建基于内存的 cacheManager，用于集成测试
// ============================================================

func newMemoryCacheManager() *cache.Cache[string] {
	client := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10*time.Minute),
	)
	s := memstore.NewTTLCache(client)
	return cache.New[string](s)
}

// ============================================================
// Option 函数测试
// ============================================================

func TestWithAutoRenew(t *testing.T) {
	opt := &option{}
	WithAutoRenew(true)(opt)
	assert.True(t, opt.autoRenew)

	WithAutoRenew(false)(opt)
	assert.False(t, opt.autoRenew)
}

func TestWithExpiration(t *testing.T) {
	opt := &option{}

	// 正常值
	WithExpiration(10 * time.Minute)(opt)
	assert.Equal(t, 10*time.Minute, opt.expiration)

	// 零值应回退为默认 5 分钟
	WithExpiration(0)(opt)
	assert.Equal(t, 5*time.Minute, opt.expiration)
}

func TestWithRenewThreshold(t *testing.T) {
	tests := []struct {
		name     string
		ratio    float64
		expected float64
	}{
		{"normal value", 0.5, 0.5},
		{"zero defaults to 0.2", 0, 0.2},
		{"negative defaults to 0.2", -0.1, 0.2},
		{"above 1 capped to 1", 1.5, 1.0},
		{"exactly 1", 1.0, 1.0},
		{"small positive", 0.01, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := &option{}
			WithRenewThreshold(tt.ratio)(opt)
			assert.Equal(t, tt.expected, opt.renewThreshold)
		})
	}
}

func TestWithCodec(t *testing.T) {
	opt := &option{}

	// nil codec 不应覆盖默认值
	defaultCodec := opt.codec
	WithCodec(nil)(opt)
	assert.Equal(t, defaultCodec, opt.codec)

	// 非 nil codec 应覆盖
	msgpackCodec := MsgpackCodec{}
	WithCodec(msgpackCodec)(opt)
	assert.Equal(t, msgpackCodec, opt.codec)
}

func TestWithErrorCacheTTL(t *testing.T) {
	opt := &option{}

	WithErrorCacheTTL(30 * time.Second)(opt)
	assert.Equal(t, 30*time.Second, opt.errorCacheTTL)

	WithErrorCacheTTL(0)(opt)
	assert.Equal(t, time.Duration(0), opt.errorCacheTTL)
}

// ============================================================
// ClearOption 函数测试
// ============================================================

func TestClearWithID(t *testing.T) {
	opt := &clearOption{}
	ClearWithID(1, 2, 3)(opt)

	assert.Equal(t, []uint{1, 2, 3}, opt.ids)
	assert.True(t, opt.single)
	assert.True(t, opt.paginate)
	assert.True(t, opt.list)
	assert.True(t, opt.remember)
	assert.False(t, opt.all)
}

func TestClearWithID_InitializesSlice(t *testing.T) {
	opt := &clearOption{}
	ClearWithID(5)(opt)
	assert.NotNil(t, opt.ids)
	assert.Equal(t, []uint{5}, opt.ids)
}

func TestClearWithKey(t *testing.T) {
	opt := &clearOption{}
	ClearWithKey("key1", "key2")(opt)

	assert.Equal(t, []string{"key1", "key2"}, opt.keys)
	assert.True(t, opt.single)
	assert.False(t, opt.paginate)
	assert.False(t, opt.list)
	assert.False(t, opt.all)
}

func TestClearWithKey_InitializesSlice(t *testing.T) {
	opt := &clearOption{}
	ClearWithKey("k")(opt)
	assert.NotNil(t, opt.keys)
	assert.Equal(t, []string{"k"}, opt.keys)
}

func TestClearWithTags(t *testing.T) {
	opt := &clearOption{}
	ClearWithTags("tag1", "tag2")(opt)

	assert.Equal(t, []string{"tag1", "tag2"}, opt.tags)
	assert.True(t, opt.single)
	assert.False(t, opt.all)
}

func TestClearWithTags_InitializesSlice(t *testing.T) {
	opt := &clearOption{}
	ClearWithTags("t")(opt)
	assert.NotNil(t, opt.tags)
	assert.Equal(t, []string{"t"}, opt.tags)
}

func TestClearWithAll(t *testing.T) {
	t.Run("all=true", func(t *testing.T) {
		opt := &clearOption{}
		ClearWithID(1)(opt) // 先设置 single=true
		ClearWithAll(true)(opt)
		assert.True(t, opt.all)
		assert.False(t, opt.single) // all=true 时 single 应被设为 false
	})

	t.Run("all=false with no ids/keys/tags", func(t *testing.T) {
		opt := &clearOption{}
		ClearWithAll(false)(opt)
		assert.False(t, opt.all)
		assert.False(t, opt.single) // 无 ids/keys/tags 时 single 为 false
	})

	t.Run("all=false with existing ids", func(t *testing.T) {
		opt := &clearOption{}
		ClearWithID(1)(opt)
		ClearWithAll(false)(opt)
		assert.False(t, opt.all)
		assert.True(t, opt.single) // 有 ids 时 single 为 true
	})
}

// ============================================================
// 构造函数测试
// ============================================================

func TestNew_DefaultOptions(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	assert.NotNil(t, c)
	assert.Equal(t, JSONCodec{}, c.Codec())
}

func TestNew_CustomOptions(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr,
		WithAutoRenew(false),
		WithExpiration(10*time.Minute),
		WithRenewThreshold(0.5),
		WithCodec(MsgpackCodec{}),
		WithErrorCacheTTL(30*time.Second),
	)

	assert.NotNil(t, c)
	assert.Equal(t, MsgpackCodec{}, c.Codec())
}

func TestNew_NilCacheManager(t *testing.T) {
	// New 不应 panic，但后续操作应返回错误
	c := New[testEntity, testFilter]("test", nil)
	assert.NotNil(t, c)
}

// ============================================================
// First 方法测试
// ============================================================

func TestFirst_ZeroID(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	_, err := c.First(context.Background(), 0, func(_ context.Context) (*testEntity, error) {
		return &testEntity{ID: 1, Name: "test"}, nil
	})

	assert.Error(t, err)
	// 验证是 ErrBadRequest 类型的 XCode 错误
	var xe xerror.XError
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, xcode.RequestParamError.Code(), xe.XCode().Code())
}

func TestFirst_NilCacheManager(t *testing.T) {
	c := New[testEntity, testFilter]("test", nil)

	_, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return &testEntity{ID: 1, Name: "test"}, nil
	})

	assert.Error(t, err)
	var xe xerror.XError
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgXcode.ErrCacheNotInitialized.Code(), xe.XCode().Code())
}

func TestFirst_QueryError(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	queryErr := fmt.Errorf("db connection lost")
	_, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return nil, queryErr
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, queryErr)
}

func TestFirst_Success(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	entity := &testEntity{ID: 1, Name: "hello"}
	result, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, entity.ID, result.ID)
	assert.Equal(t, entity.Name, result.Name)
}

func TestFirst_CacheHit(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	callCount := 0
	entity := &testEntity{ID: 1, Name: "hello"}

	// 第一次调用：miss → query → set
	result, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次调用：应命中缓存
	result, err = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "query should not be called on cache hit")
	assert.Equal(t, entity.ID, result.ID)
}

// ============================================================
// Remember 方法测试
// ============================================================

func TestRemember_NilCacheManager(t *testing.T) {
	c := New[testEntity, testFilter]("test", nil)

	_, err := c.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		return []byte("data"), nil
	})

	assert.Error(t, err)
	var xe xerror.XError
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgXcode.ErrCacheNotInitialized.Code(), xe.XCode().Code())
}

func TestRemember_QueryError(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	_, err := c.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		return nil, fmt.Errorf("query failed")
	})

	assert.Error(t, err)
}

func TestRemember_Success(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	data := []byte(`{"result":42}`)
	result, err := c.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		return data, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, data, result)
}

func TestRemember_CacheHit(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	callCount := 0
	data := []byte(`{"result":42}`)

	// 第一次：miss → query
	_, err := c.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		callCount++
		return data, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次：hit
	result, err := c.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		callCount++
		return data, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, data, result)
	assert.Equal(t, 1, callCount, "query should not be called on cache hit")
}

// ============================================================
// RememberOf 函数测试
// ============================================================

func TestRememberOf_Success(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	result, err := RememberOf[string](context.Background(), c, "greeting", func(_ context.Context) (string, error) {
		return "hello world", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestRememberOf_CacheHit(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	callCount := 0

	// 第一次：miss
	result, err := RememberOf[string](context.Background(), c, "greeting", func(_ context.Context) (string, error) {
		callCount++
		return "hello world", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "hello world", result)

	// 第二次：hit
	result, err = RememberOf[string](context.Background(), c, "greeting", func(_ context.Context) (string, error) {
		callCount++
		return "should not be called", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "hello world", result)
	assert.Equal(t, 1, callCount)
}

func TestRememberOf_QueryError(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	result, err := RememberOf[string](context.Background(), c, "greeting", func(_ context.Context) (string, error) {
		return "", fmt.Errorf("query error")
	})

	assert.Error(t, err)
	assert.Equal(t, "", result)
}

func TestRememberOf_NilCacheManager(t *testing.T) {
	c := New[testEntity, testFilter]("test", nil)

	result, err := RememberOf[string](context.Background(), c, "greeting", func(_ context.Context) (string, error) {
		return "hello", nil
	})

	assert.Error(t, err)
	assert.Equal(t, "", result) // zero value for string
}

// ============================================================
// RememberWithContext 方法测试
// ============================================================

func TestRemember_ContextSuccess(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	data := []byte(`{"result":42}`)
	result, err := c.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		return data, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, data, result)
}

func TestRemember_ContextCancel(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := c.Remember(ctx, "key1", func(ctx context.Context) ([]byte, error) {
		return []byte("data"), nil
	})

	// context 已取消，应返回错误
	assert.Error(t, err)
}

// ============================================================
// RememberOfWithContext 函数测试
// ============================================================

func TestRememberOf_ContextSuccess(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	result, err := RememberOf[string](context.Background(), c, "greeting", func(_ context.Context) (string, error) {
		return "hello world", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestRememberOf_ContextCancel(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RememberOf[string](ctx, c, "greeting", func(ctx context.Context) (string, error) {
		return "hello", nil
	})

	assert.Error(t, err)
}

// ============================================================
// Paginate 方法测试
// ============================================================

func TestPaginate_Success(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	q := dbquery.NewQuery(testFilter{Name: "foo"}, dbquery.WithOffsetPage[testFilter](0, 10))
	records := []*testEntity{{ID: 1, Name: "foo"}, {ID: 2, Name: "bar"}}
	total := uint(2)

	result, count, err := c.Paginate(context.Background(), q, func(_ context.Context) ([]*testEntity, uint, error) {
		return records, total, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, total, count)
	assert.Len(t, result, 2)
	assert.Equal(t, records[0].ID, result[0].ID)
}

func TestPaginate_QueryError(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	q := dbquery.NewQuery(testFilter{Name: "foo"}, dbquery.WithOffsetPage[testFilter](0, 10))

	_, _, err := c.Paginate(context.Background(), q, func(_ context.Context) ([]*testEntity, uint, error) {
		return nil, 0, fmt.Errorf("db error")
	})

	assert.Error(t, err)
}

func TestPaginate_CacheHit(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	q := dbquery.NewQuery(testFilter{Name: "foo"}, dbquery.WithOffsetPage[testFilter](0, 10))
	records := []*testEntity{{ID: 1, Name: "foo"}}
	total := uint(1)
	callCount := 0

	// 第一次：miss
	result, count, err := c.Paginate(context.Background(), q, func(_ context.Context) ([]*testEntity, uint, error) {
		callCount++
		return records, total, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次：hit
	result, count, err = c.Paginate(context.Background(), q, func(_ context.Context) ([]*testEntity, uint, error) {
		callCount++
		return records, total, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "query should not be called on cache hit")
	assert.Equal(t, total, count)
	assert.Equal(t, records[0].ID, result[0].ID)
}

// ============================================================
// List 方法测试
// ============================================================

func TestList_Success(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	q := dbquery.NewQuery(testFilter{Name: "foo"})
	records := []*testEntity{{ID: 1, Name: "foo"}, {ID: 2, Name: "bar"}}

	result, err := c.List(context.Background(), q, func(_ context.Context) ([]*testEntity, error) {
		return records, nil
	})

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, records[0].ID, result[0].ID)
}

func TestList_QueryError(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	q := dbquery.NewQuery(testFilter{Name: "foo"})

	_, err := c.List(context.Background(), q, func(_ context.Context) ([]*testEntity, error) {
		return nil, fmt.Errorf("db error")
	})

	assert.Error(t, err)
}

func TestList_CacheHit(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	q := dbquery.NewQuery(testFilter{Name: "foo"})
	records := []*testEntity{{ID: 1, Name: "foo"}}
	callCount := 0

	// 第一次：miss
	_, err := c.List(context.Background(), q, func(_ context.Context) ([]*testEntity, error) {
		callCount++
		return records, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次：hit
	result, err := c.List(context.Background(), q, func(_ context.Context) ([]*testEntity, error) {
		callCount++
		return records, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "query should not be called on cache hit")
	assert.Equal(t, records[0].ID, result[0].ID)
}

// ============================================================
// Clear 方法测试
// ============================================================

func TestClear_NilCacheManager(t *testing.T) {
	c := New[testEntity, testFilter]("test", nil)
	err := c.Clear(context.Background())
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, pkgXcode.ErrCacheNotInitialized))
}

func TestClear_DefaultNoOptions(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	// 写入缓存
	entity := &testEntity{ID: 1, Name: "hello"}
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity, nil
	})

	// 无选项调用 Clear：应返回错误
	err := c.Clear(context.Background())
	assert.Error(t, err)

	// 缓存应仍在
	callCount := 0
	result, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, callCount, "cache should still exist after Clear() without options")
	assert.Equal(t, entity.ID, result.ID)
}

func TestClear_WithAllTrue(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	// 写入缓存
	entity := &testEntity{ID: 1, Name: "hello"}
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity, nil
	})

	// ClearWithAll(true) 清理所有缓存
	err := c.Clear(context.Background(), ClearWithAll(true))
	assert.NoError(t, err)

	// 缓存应已被清除，再次查询应触发 query
	callCount := 0
	_, err = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "query should be called after ClearWithAll(true)")
}

func TestClear_All(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	// 先写入一些缓存
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return &testEntity{ID: 1, Name: "hello"}, nil
	})

	// 清理所有缓存
	err := c.Clear(context.Background(), ClearWithAll(true))
	assert.NoError(t, err)
}

func TestClear_ByID(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	// 先写入缓存
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return &testEntity{ID: 1, Name: "hello"}, nil
	})

	// 按 ID 清理
	err := c.Clear(context.Background(), ClearWithID(1))
	assert.NoError(t, err)
}

func TestClear_ByKey(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	err := c.Clear(context.Background(), ClearWithKey("somekey"))
	assert.NoError(t, err)
}

func TestClear_ByTags(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	err := c.Clear(context.Background(), ClearWithTags("tag1"))
	assert.NoError(t, err)
}

// ============================================================
// Forget 方法测试
// ============================================================

func TestForget_NilCacheManager(t *testing.T) {
	c := New[testEntity, testFilter]("test", nil)

	err := c.Forget(context.Background(), "key1")
	assert.Error(t, err)

	var xe xerror.XError
	assert.True(t, errors.As(err, &xe))
	assert.Equal(t, pkgXcode.ErrCacheNotInitialized.Code(), xe.XCode().Code())
}

func TestForget_Success(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	err := c.Forget(context.Background(), "key1")
	assert.NoError(t, err)
}

// ============================================================
// Codec 方法测试
// ============================================================

func TestCodec_Default(t *testing.T) {
	c := New[testEntity, testFilter]("test", nil)
	assert.Equal(t, JSONCodec{}, c.Codec())
}

func TestCodec_Custom(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr, WithCodec(MsgpackCodec{}))
	assert.Equal(t, MsgpackCodec{}, c.Codec())
}

// ============================================================
// 错误占位值测试（errorCacheTTL）
// ============================================================

func TestErrorCache_ErrorPlaceholder(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr,
		WithErrorCacheTTL(30*time.Second),
	)

	callCount := 0

	// 第一次查询失败：错误应被缓存到独立键
	_, err := c.Remember(context.Background(), "error-key", func(_ context.Context) ([]byte, error) {
		callCount++
		return nil, fmt.Errorf("temporary failure")
	})
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次查询同一 key：应命中错误占位值，回调不应被调用
	_, err = c.Remember(context.Background(), "error-key", func(_ context.Context) ([]byte, error) {
		callCount++
		return []byte("should not reach"), nil
	})
	assert.Error(t, err)
	assert.Equal(t, 1, callCount, "query should not be called when error placeholder is hit")

	// 错误占位值被命中时，应返回 ErrCacheMiss 类型的 XCode 错误
	var xe xerror.XError
	if errors.As(err, &xe) {
		assert.Equal(t, pkgXcode.ErrCacheMiss.Code(), xe.XCode().Code())
	}
}

func TestErrorCache_NormalDataNotContaminated(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr,
		WithErrorCacheTTL(30*time.Second),
	)

	// 先让 "ok-key" 查询成功，正常数据写入缓存
	data := []byte(`{"status":"ok"}`)
	result, err := c.Remember(context.Background(), "ok-key", func(_ context.Context) ([]byte, error) {
		return data, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, data, result)

	// 另一个 key 查询失败，错误占位值写入独立键
	_, err = c.Remember(context.Background(), "fail-key", func(_ context.Context) ([]byte, error) {
		return nil, fmt.Errorf("something wrong")
	})
	assert.Error(t, err)

	// "ok-key" 的正常数据应不受影响
	result, err = c.Remember(context.Background(), "ok-key", func(_ context.Context) ([]byte, error) {
		return []byte("should not reach"), nil
	})
	assert.NoError(t, err)
	assert.Equal(t, data, result)
}

// ============================================================
// 自动续期测试
// ============================================================

func TestAutoRenew_Default(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)
	// 默认 autoRenew=true，不应 panic

	_, err := c.Remember(context.Background(), "renew-key", func(_ context.Context) ([]byte, error) {
		return []byte("data"), nil
	})
	assert.NoError(t, err)
}

// ============================================================
// 项目级 xcode 断言测试
// ============================================================

func TestXCodeErrCacheSetFailed(t *testing.T) {
	assert.Equal(t, 13002, pkgXcode.ErrCacheSetFailed.Code())
}

func TestXCodeErrCacheMiss(t *testing.T) {
	assert.Equal(t, 13001, pkgXcode.ErrCacheMiss.Code())
}

// ============================================================
// Tag 生成测试（通过暴露行为的间接测试）
// ============================================================

func TestTagGeneration_DifferentNamesProduceDifferentTags(t *testing.T) {
	mgr := newMemoryCacheManager()
	c1 := New[testEntity, testFilter]("app1", mgr)
	c2 := New[testEntity, testFilter]("app2", mgr)

	// 不同 name 的缓存互不影响
	_, _ = c1.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		return []byte("from-app1"), nil
	})

	result, err := c2.Remember(context.Background(), "key1", func(_ context.Context) ([]byte, error) {
		return []byte("from-app2"), nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []byte("from-app2"), result, "different cache names should not share cache entries")
}

// ============================================================
// 测试类型定义
// ============================================================

type testEntity struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type testFilter struct {
	Name string `json:"name"`
}

// ============================================================
// 并发访问测试（singleflight 验证）
// ============================================================

func TestFirst_ConcurrentAccess(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	var queryCount atomic.Int32
	entity := &testEntity{ID: 1, Name: "hello"}

	// 首次写入缓存
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity, nil
	})

	// 清除缓存，让后续请求需要重新查询
	_ = c.Clear(context.Background(), ClearWithAll(true))

	// 并发 10 个请求，singleflight 应合并为少量查询
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
				queryCount.Add(1)
				time.Sleep(50 * time.Millisecond) // 模拟慢查询
				return entity, nil
			})
			assert.NoError(t, err)
			assert.Equal(t, entity.ID, result.ID)
		}()
	}
	wg.Wait()

	// singleflight 应确保查询次数远少于 10
	assert.Less(t, queryCount.Load(), int32(10), "singleflight should deduplicate concurrent queries")
}

func TestClear_WithAll_InvalidatesCache(t *testing.T) {
	mgr := newMemoryCacheManager()
	c := New[testEntity, testFilter]("test", mgr)

	// 写入多个缓存条目
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return &testEntity{ID: 1, Name: "one"}, nil
	})
	_, _ = c.First(context.Background(), 2, func(_ context.Context) (*testEntity, error) {
		return &testEntity{ID: 2, Name: "two"}, nil
	})

	// 清除所有缓存
	err := c.Clear(context.Background(), ClearWithAll(true))
	assert.NoError(t, err)

	// 验证两个缓存条目均被清除
	callCount := 0
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return &testEntity{ID: 1, Name: "one"}, nil
	})
	_, _ = c.First(context.Background(), 2, func(_ context.Context) (*testEntity, error) {
		callCount++
		return &testEntity{ID: 2, Name: "two"}, nil
	})
	assert.Equal(t, 2, callCount, "all cache entries should be invalidated")
}

func TestAutoRenew_ExtendsTTL(t *testing.T) {
	// 使用短过期时间验证自动续期行为
	tc := ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](200*time.Millisecond),
	)
	s := memstore.NewTTLCache(tc)
	mgr := cache.New[string](s)

	c := New[testEntity, testFilter]("test", mgr,
		WithAutoRenew(true),
		WithExpiration(200*time.Millisecond),
		WithRenewThreshold(0.5),
	)

	entity := &testEntity{ID: 1, Name: "hello"}
	_, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity, nil
	})
	assert.NoError(t, err)

	// 在 TTL 过半后访问（触发续期）
	time.Sleep(120 * time.Millisecond)
	result, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, entity.ID, result.ID)
}
