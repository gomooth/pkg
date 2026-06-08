package memstore

import (
	"context"
	"testing"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient 创建用于测试的 ttlcache 客户端
func newTestClient() *ttlcache.Cache[string, any] {
	return ttlcache.New[string, any](
		ttlcache.WithTTL[string, any](10 * time.Minute),
	)
}

// TestNewTTLCache 验证构造函数返回非 nil 的 StoreInterface
func TestNewTTLCache(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	assert.NotNil(t, s)
}

// TestNewTTLCache_WithOptions 验证构造函数接受 store.Option 参数
func TestNewTTLCache_WithOptions(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client, store.WithExpiration(5*time.Minute))
	assert.NotNil(t, s)
}

// TestGetType 验证 GetType 返回正确的存储类型
func TestGetType(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	assert.Equal(t, StoreType, s.GetType())
	assert.Equal(t, "ttlcache", s.GetType())
}

// TestGet_SetAndGet 验证基本的 Set/Get 操作
func TestGet_SetAndGet(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	// Set 一个值
	err := s.Set(ctx, "key1", "value1")
	assert.Nil(t, err)

	// Get 应返回对应值
	val, err := s.Get(ctx, "key1")
	assert.Nil(t, err)
	assert.Equal(t, "value1", val)
}

// TestGet_NotFound 验证 Get 不存在的 key 返回错误
func TestGet_NotFound(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	val, err := s.Get(ctx, "nonexistent")
	assert.NotNil(t, err)
	assert.Nil(t, val)
}

// TestGet_ExpiredItem 验证 Get 过期项返回错误
func TestGet_ExpiredItem(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	// Set 一个很短 TTL 的值
	err := s.Set(ctx, "short-lived", "data", store.WithExpiration(1*time.Millisecond))
	require.Nil(t, err)

	// 等待过期
	time.Sleep(10 * time.Millisecond)

	val, err := s.Get(ctx, "short-lived")
	assert.NotNil(t, err)
	assert.Nil(t, val)
}

// TestGetWithTTL_SetAndGet 验证基本的 GetWithTTL 操作
func TestGetWithTTL_SetAndGet(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	ttl := 5 * time.Minute
	err := s.Set(ctx, "ttl-key", "ttl-value", store.WithExpiration(ttl))
	require.Nil(t, err)

	val, duration, err := s.GetWithTTL(ctx, "ttl-key")
	assert.Nil(t, err)
	assert.Equal(t, "ttl-value", val)
	// TTL 应该接近 5 分钟（允许微小误差）
	assert.True(t, duration > 4*time.Minute, "TTL should be close to 5 minutes, got %v", duration)
}

// TestGetWithTTL_NotFound 验证 GetWithTTL 不存在的 key 返回错误
func TestGetWithTTL_NotFound(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	val, duration, err := s.GetWithTTL(ctx, "nonexistent")
	assert.NotNil(t, err)
	assert.Nil(t, val)
	assert.Equal(t, time.Duration(0), duration)
}

// TestGetWithTTL_ExpiredItem 验证 GetWithTTL 过期项返回错误
func TestGetWithTTL_ExpiredItem(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Set(ctx, "expired-ttl", "data", store.WithExpiration(1*time.Millisecond))
	require.Nil(t, err)

	time.Sleep(10 * time.Millisecond)

	val, duration, err := s.GetWithTTL(ctx, "expired-ttl")
	assert.NotNil(t, err)
	assert.Nil(t, val)
	assert.Equal(t, time.Duration(0), duration)
}

// TestSet_DefaultExpiration 验证 Set 不带选项时使用 NoTTL（永不过期）
func TestSet_DefaultExpiration(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	// 不传选项，使用 store 默认配置
	err := s.Set(ctx, "no-expire", "forever")
	require.Nil(t, err)

	// 应该能获取到值
	val, err := s.Get(ctx, "no-expire")
	assert.Nil(t, err)
	assert.Equal(t, "forever", val)
}

// TestSet_WithExpiration 验证 Set 带自定义过期时间
func TestSet_WithExpiration(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Set(ctx, "custom-ttl", "custom", store.WithExpiration(30*time.Second))
	require.Nil(t, err)

	val, err := s.Get(ctx, "custom-ttl")
	assert.Nil(t, err)
	assert.Equal(t, "custom", val)
}

// TestSet_WithTags 验证 Set 带 tags 能正确存储 tag 映射
func TestSet_WithTags(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Set(ctx, "tagged-key", "tagged-value", store.WithTags([]string{"tag1", "tag2"}))
	require.Nil(t, err)

	// 验证值能正常获取
	val, err := s.Get(ctx, "tagged-key")
	assert.Nil(t, err)
	assert.Equal(t, "tagged-value", val)

	// 验证 tag 内部映射存在
	tagKey1 := "gocache_tag_tag1"
	tagVal1, err := s.Get(ctx, tagKey1)
	assert.Nil(t, err)
	tagMap1, ok := tagVal1.(map[string]struct{})
	require.True(t, ok)
	_, exists := tagMap1["tagged-key"]
	assert.True(t, exists)

	tagKey2 := "gocache_tag_tag2"
	tagVal2, err := s.Get(ctx, tagKey2)
	assert.Nil(t, err)
	tagMap2, ok := tagVal2.(map[string]struct{})
	require.True(t, ok)
	_, exists = tagMap2["tagged-key"]
	assert.True(t, exists)
}

// TestSet_WithTags_MultipleKeysSameTag 验证同一 tag 下多个 key
func TestSet_WithTags_MultipleKeysSameTag(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Set(ctx, "key-a", "val-a", store.WithTags([]string{"shared-tag"}))
	require.Nil(t, err)

	err = s.Set(ctx, "key-b", "val-b", store.WithTags([]string{"shared-tag"}))
	require.Nil(t, err)

	// 两个 key 都应存在
	valA, err := s.Get(ctx, "key-a")
	assert.Nil(t, err)
	assert.Equal(t, "val-a", valA)

	valB, err := s.Get(ctx, "key-b")
	assert.Nil(t, err)
	assert.Equal(t, "val-b", valB)

	// tag 映射中应包含两个 key
	tagVal, err := s.Get(ctx, "gocache_tag_shared-tag")
	assert.Nil(t, err)
	tagMap, ok := tagVal.(map[string]struct{})
	require.True(t, ok)
	assert.Len(t, tagMap, 2)
	_, hasA := tagMap["key-a"]
	assert.True(t, hasA)
	_, hasB := tagMap["key-b"]
	assert.True(t, hasB)
}

// TestDelete_ExistingKey 验证删除已存在的 key
func TestDelete_ExistingKey(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Set(ctx, "del-key", "del-value")
	require.Nil(t, err)

	err = s.Delete(ctx, "del-key")
	assert.Nil(t, err)

	// 删除后 Get 应返回 not found
	val, err := s.Get(ctx, "del-key")
	assert.NotNil(t, err)
	assert.Nil(t, val)
}

// TestDelete_NonExistentKey 验证删除不存在的 key 不会报错
func TestDelete_NonExistentKey(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	assert.Nil(t, err)
}

// TestClear 验证清空所有缓存
func TestClear(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	// 添加多个 key
	err := s.Set(ctx, "key1", "val1")
	require.Nil(t, err)
	err = s.Set(ctx, "key2", "val2")
	require.Nil(t, err)

	// 清空
	err = s.Clear(ctx)
	assert.Nil(t, err)

	// 所有 key 都应不可访问
	_, err = s.Get(ctx, "key1")
	assert.NotNil(t, err)
	_, err = s.Get(ctx, "key2")
	assert.NotNil(t, err)
}

// TestInvalidate_ByTag 验证通过 tag 使关联 key 失效
func TestInvalidate_ByTag(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	// 设置带 tag 的 key
	err := s.Set(ctx, "inv-key1", "inv-val1", store.WithTags([]string{"inv-tag"}))
	require.Nil(t, err)
	err = s.Set(ctx, "inv-key2", "inv-val2", store.WithTags([]string{"inv-tag"}))
	require.Nil(t, err)

	// 设置不带 tag 的 key，不应被影响
	err = s.Set(ctx, "other-key", "other-val")
	require.Nil(t, err)

	// 通过 tag 失效
	err = s.Invalidate(ctx, store.WithInvalidateTags([]string{"inv-tag"}))
	assert.Nil(t, err)

	// 带 tag 的 key 应已被删除
	_, err = s.Get(ctx, "inv-key1")
	assert.NotNil(t, err)
	_, err = s.Get(ctx, "inv-key2")
	assert.NotNil(t, err)

	// 不带 tag 的 key 应不受影响
	val, err := s.Get(ctx, "other-key")
	assert.Nil(t, err)
	assert.Equal(t, "other-val", val)
}

// TestInvalidate_NonExistentTag 验证使不存在的 tag 失效不报错
func TestInvalidate_NonExistentTag(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Invalidate(ctx, store.WithInvalidateTags([]string{"no-such-tag"}))
	assert.Nil(t, err)
}

// TestInvalidate_NoTags 验证 Invalidate 不带 tag 选项时不做任何操作
func TestInvalidate_NoTags(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	err := s.Set(ctx, "safe-key", "safe-val")
	require.Nil(t, err)

	err = s.Invalidate(ctx)
	assert.Nil(t, err)

	// key 应该还在
	val, err := s.Get(ctx, "safe-key")
	assert.Nil(t, err)
	assert.Equal(t, "safe-val", val)
}

// TestItemCount 验证 ItemCount 函数返回正确的缓存项数量
func TestItemCount(t *testing.T) {
	client := newTestClient()
	countFn := ItemCount(client)

	// 初始应为 0
	assert.Equal(t, 0, countFn())

	ctx := context.Background()
	s := NewTTLCache(client)

	// 添加项
	err := s.Set(ctx, "count1", "v1")
	require.Nil(t, err)
	assert.Equal(t, 1, countFn())

	err = s.Set(ctx, "count2", "v2")
	require.Nil(t, err)
	assert.Equal(t, 2, countFn())

	// 删除一项
	err = s.Delete(ctx, "count1")
	require.Nil(t, err)
	assert.Equal(t, 1, countFn())

	// 清空
	err = s.Clear(ctx)
	require.Nil(t, err)
	assert.Equal(t, 0, countFn())
}

// TestSet_UsesDefaultOptionsWhenNoneProvided 验证 Set 在不传选项时使用构造时的默认选项
func TestSet_UsesDefaultOptionsWhenNoneProvided(t *testing.T) {
	client := newTestClient()
	// 构造时设置默认过期时间
	s := NewTTLCache(client, store.WithExpiration(5*time.Minute))
	ctx := context.Background()

	err := s.Set(ctx, "default-opt-key", "default-opt-val")
	require.Nil(t, err)

	val, err := s.Get(ctx, "default-opt-key")
	assert.Nil(t, err)
	assert.Equal(t, "default-opt-val", val)
}

// TestSet_ZeroExpirationUsesNoTTL 验证 Set 时选项中 Expiration=0 使用 NoTTL
func TestSet_ZeroExpirationUsesNoTTL(t *testing.T) {
	client := newTestClient()
	s := NewTTLCache(client)
	ctx := context.Background()

	// 使用 store.WithExpiration(0) 传递零值过期时间
	// 内部逻辑：ttl == 0 时使用 ttlcache.NoTTL（永不过期）
	err := s.Set(ctx, "zero-ttl-key", "zero-ttl-val", store.WithExpiration(0))
	require.Nil(t, err)

	val, err := s.Get(ctx, "zero-ttl-key")
	assert.Nil(t, err)
	assert.Equal(t, "zero-ttl-val", val)
}
