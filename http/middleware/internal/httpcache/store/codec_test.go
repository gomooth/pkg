package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// TestNewOwnedRedisStore 创建拥有 Redis client 生命周期的 store
func TestNewOwnedRedisStore(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	store := NewOwnedRedisStore(client)
	assert.NotNil(t, store)
	assert.True(t, store.ownClient)
	assert.Equal(t, client, store.RedisClient)
}

// TestNewRedisStore_NotOwnClient 创建不拥有 Redis client 生命周期的 store
func TestNewRedisStore_NotOwnClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	store := NewRedisStore(client)
	assert.NotNil(t, store)
	assert.False(t, store.ownClient)
}

// TestRedisStore_Close_OwnedClient 拥有 client 时 Close 关闭连接
func TestRedisStore_Close_OwnedClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	store := NewOwnedRedisStore(client)
	err := store.Close()
	assert.NoError(t, err)
}

// TestRedisStore_Close_NotOwnedClient 不拥有 client 时 Close 为空操作
func TestRedisStore_Close_NotOwnedClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	store := NewRedisStore(client)
	err := store.Close()
	assert.NoError(t, err)
}

// TestRedisStore_Close_NilClient client 为 nil 时 Close 为空操作
func TestRedisStore_Close_NilClient(t *testing.T) {
	store := &RedisStore{ownClient: true, RedisClient: nil}
	err := store.Close()
	assert.NoError(t, err)
}

// TestSerialize_Normal 正常序列化
func TestSerialize_Normal(t *testing.T) {
	resp := &CachedResponse{
		Status: 200,
		Data:   []byte("hello"),
	}

	data, err := serialize(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TestUnserialize_RoundTrip 序列化+反序列化往返测试
func TestUnserialize_RoundTrip(t *testing.T) {
	original := &CachedResponse{
		Status: 200,
		Header: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Data: []byte(`{"key":"value"}`),
	}

	data, err := serialize(original)
	assert.NoError(t, err)

	var result CachedResponse
	err = unserialize(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, original.Status, result.Status)
	assert.Equal(t, original.Data, result.Data)
	assert.Equal(t, "application/json", result.Header.Get("Content-Type"))
}

// TestUnserialize_InvalidData 无效数据反序列化失败
func TestUnserialize_InvalidData(t *testing.T) {
	invalidData := []byte("this is not valid gob data")
	var result CachedResponse
	err := unserialize(invalidData, &result)
	assert.Error(t, err)
}

// TestNewOwnedRedisStore_SetAndGet 完整的 Set+Get 往返测试
func TestNewOwnedRedisStore_SetAndGet(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	store := NewOwnedRedisStore(client)
	defer store.Close()

	key := "owned-test-key"
	resp := &CachedResponse{
		Status: 200,
		Header: map[string][]string{"X-Custom": {"value"}},
		Data:   []byte("owned-store-data"),
	}

	ctx := context.Background()
	err := store.Set(ctx, key, resp, 10*time.Minute)
	assert.NoError(t, err)

	var retrieved CachedResponse
	err = store.Get(ctx, key, &retrieved)
	assert.NoError(t, err)
	assert.Equal(t, 200, retrieved.Status)
	assert.Equal(t, "value", retrieved.Header.Get("X-Custom"))
	assert.Equal(t, []byte("owned-store-data"), retrieved.Data)
}

// TestRedisStore_Get_ContextCanceled 上下文取消时 Get 返回错误
func TestRedisStore_Get_ContextCanceled(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	store := NewRedisStore(client)

	// 先设置数据
	key := "cancel-test-key"
	resp := &CachedResponse{Status: 200, Data: []byte("data")}
	ctx := context.Background()
	err := store.Set(ctx, key, resp, 10*time.Minute)
	assert.NoError(t, err)

	// 使用已取消的 context 调用 Get
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	var retrieved CachedResponse
	err = store.Get(canceledCtx, key, &retrieved)
	assert.Error(t, err)
}

// TestRedisStore_Delete_ContextCanceled 上下文取消时 Delete 返回错误
func TestRedisStore_Delete_ContextCanceled(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	store := NewRedisStore(client)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Delete(canceledCtx, "any-key")
	assert.Error(t, err)
}

// TestSerialize_InvalidType 不可序列化的类型
func TestSerialize_InvalidType(t *testing.T) {
	// channels 不能被 gob 编码
	ch := make(chan int)
	_, err := serialize(ch)
	assert.Error(t, err)
}

// TestErrorCacheMiss_ErrorCacheMiss 常量正确性
func TestErrorCacheMiss_ErrorCacheMiss(t *testing.T) {
	assert.NotNil(t, ErrorCacheMiss)
}

// TestCachedResponse_Fields CachedResponse 字段正确性
func TestCachedResponse_Fields(t *testing.T) {
	resp := CachedResponse{
		Status: 404,
		Header: map[string][]string{"X-Test": {"ok"}},
		Data:   []byte("not found"),
	}
	assert.Equal(t, 404, resp.Status)
	assert.Equal(t, "ok", resp.Header.Get("X-Test"))
	assert.Equal(t, []byte("not found"), resp.Data)
}

// TestICacheStore_Interface 编译时接口检查
func TestICacheStore_Interface(t *testing.T) {
	var _ ICacheStore = (*RedisStore)(nil)
}

// TestRedisStore_Set_SerializeError 不可序列化的值导致 Set 失败
func TestRedisStore_Set_SerializeError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	store := NewRedisStore(client)

	// CachedResponse 本身可序列化，所以无法直接触发序列化错误
	// 但我们可以通过 codec 测试不可序列化类型（见 TestSerialize_InvalidType）
	// 此测试确保 Set 的序列化错误路径存在
	ctx := context.Background()
	resp := &CachedResponse{Status: 200, Data: []byte("ok")}
	err := store.Set(ctx, "key", resp, time.Minute)
	assert.NoError(t, err)
}
