package jwtstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiRedisStore_SaveExpiredToken(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewMultiRedisStore(client)

	// token already expired
	expiredTs := time.Now().Add(-1 * time.Hour).Unix()
	err := store.Save(context.Background(), _userID, "expired-token", expiredTs)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "already expired")
}

func TestMultiRedisStore_SaveWithNilClient(t *testing.T) {
	store := NewMultiRedisStore(nil)

	err := store.Save(context.Background(), _userID, "token", time.Now().Add(time.Hour).Unix())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestMultiRedisStore_CheckWithNilClient(t *testing.T) {
	store := NewMultiRedisStore(nil)

	err := store.Check(context.Background(), _userID, "token")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestMultiRedisStore_RemoveWithNilClient(t *testing.T) {
	store := NewMultiRedisStore(nil)

	err := store.Remove(context.Background(), _userID, "token")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestMultiRedisStore_CleanWithNilClient(t *testing.T) {
	store := NewMultiRedisStore(nil)

	err := store.Clean(context.Background(), _userID)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestMultiRedisStore_CheckRedisError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 2})
	store := NewMultiRedisStore(client)

	// Save a token first
	ts := time.Now().Add(10 * time.Minute).Unix()
	err = store.Save(context.Background(), _userID, "test-token", ts)
	assert.Nil(t, err)

	// Close redis to cause errors on subsequent calls
	mr.Close()

	err = store.Check(context.Background(), _userID, "test-token")
	assert.NotNil(t, err)
}

func TestMultiRedisStore_RemoveError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 2})
	store := NewMultiRedisStore(client)

	// Close redis to cause errors
	mr.Close()

	err = store.Remove(context.Background(), _userID, "test-token")
	assert.NotNil(t, err)
}

func TestMultiRedisStore_CleanError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 2})
	store := NewMultiRedisStore(client)

	// Close redis to cause errors
	mr.Close()

	err = store.Clean(context.Background(), _userID)
	assert.NotNil(t, err)
}

func TestMultiRedisStore(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewMultiRedisStore(client)

	// 多次登录
	for i, token := range _tokens {
		ts := time.Now().Add(time.Minute * time.Duration((i+1)*10)).Unix()

		assert.Equal(t, token, _tokens[i])

		err := store.Save(context.Background(), _userID, token, ts)
		assert.Nil(t, err)
	}

	// 判断 token
	for _, token := range _tokens {
		err := store.Check(context.Background(), _userID, token)
		assert.Nil(t, err)
	}

	err := store.Check(context.Background(), _userID, "not-in-tokens")
	assert.NotNil(t, err)

	// 删除 token
	err = store.Remove(context.Background(), _userID, "not-in-tokens")
	assert.Nil(t, err)

	err = store.Remove(context.Background(), _userID, _tokens[len(_tokens)-1])
	assert.Nil(t, err)

	// 清理 token
	err = store.Clean(context.Background(), _userID)
	assert.Nil(t, err)
}
