package store

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisStore_SetAndGet(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	// 创建 RedisStore
	store := NewRedisStore(client)

	// 测试数据
	key := "test-key"
	response := &CachedResponse{
		Status: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Data: []byte(`{"message":"test"}`),
	}

	// 测试 Set
	ctx := context.Background()
	err := store.Set(ctx, key, response, 10*time.Minute)
	assert.NoError(t, err)

	// 测试 Get
	var retrieved CachedResponse
	err = store.Get(ctx, key, &retrieved)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, retrieved.Status)
	assert.Equal(t, "application/json", retrieved.Header.Get("Content-Type"))
	assert.Equal(t, []byte(`{"message":"test"}`), retrieved.Data)
}

func TestRedisStore_Get_CacheMiss(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	// 创建 RedisStore
	store := NewRedisStore(client)

	// 测试获取不存在的 key
	ctx := context.Background()
	var retrieved CachedResponse
	err := store.Get(ctx, "non-existent-key", &retrieved)

	// 应该返回 ErrorCacheMiss
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrorCacheMiss))
}

func TestRedisStore_Delete(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	// 创建 RedisStore
	store := NewRedisStore(client)

	// 设置初始数据
	key := "test-key"
	response := &CachedResponse{
		Status: http.StatusOK,
		Data:   []byte("test data"),
	}

	ctx := context.Background()

	// 先设置数据
	err := store.Set(ctx, key, response, 10*time.Minute)
	assert.NoError(t, err)

	// 验证数据存在
	var retrieved CachedResponse
	err = store.Get(ctx, key, &retrieved)
	assert.NoError(t, err)
	assert.Equal(t, "test data", string(retrieved.Data))

	// 删除数据
	err = store.Delete(ctx, key)
	assert.NoError(t, err)

	// 验证数据已删除
	err = store.Get(ctx, key, &retrieved)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrorCacheMiss))
}

func TestRedisStore_Set_ContextCanceled(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	// 创建 RedisStore
	store := NewRedisStore(client)

	// 创建已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 测试数据
	response := &CachedResponse{
		Status: http.StatusOK,
		Data:   []byte("test data"),
	}

	// 使用已取消的 context 调用 Set，应该返回错误
	err := store.Set(ctx, "test-key", response, 10*time.Minute)
	assert.Error(t, err)
}

func TestRedisStore_Delete_NonExistentKey(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	// 创建 RedisStore
	store := NewRedisStore(client)

	// 删除不存在的 key
	ctx := context.Background()
	err := store.Delete(ctx, "non-existent-key")

	// Delete 操作应该成功（不存在的 key 不会报错）
	assert.NoError(t, err)
}

func TestRedisStore_Get_InvalidData(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	defer mr.Close()

	// 创建 RedisStore
	store := NewRedisStore(client)

	// 手动设置无效的数据到 redis
	ctx := context.Background()
	key := "invalid-data-key"

	// 设置无效的 gob 编码数据
	invalidData := "this is not valid gob data"
	err := client.Set(ctx, key, invalidData, 0).Err()
	assert.NoError(t, err)

	// 尝试获取数据，应该返回反序列化错误
	var retrieved CachedResponse
	err = store.Get(ctx, key, &retrieved)

	// 应该返回错误，但不应该是 ErrorCacheMiss
	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrorCacheMiss))
}
