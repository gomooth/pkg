package jwtstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomooth/pkg/http/jwt"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_userID = uint(100008)
	_tokens = []string{
		"tIqsOkAqXCum1AhiCTAMB4GqNmduU63l-1",
		"tIqsOkAqXCum1AhiCTAMB4GqNmduU63l-2",
		"tIqsOkAqXCum1AhiCTAMB4GqNmduU63l-3",
		"tIqsOkAqXCum1AhiCTAMB4GqNmduU63l-4",
	}
)

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		DB:   2,
	})
}

func TestSingleRedisStore(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewSingleRedisStore(client)
	_ts := time.Now().Add(24 * time.Hour).Unix()

	// 多次登录
	for _, token := range _tokens {
		err := store.Save(context.Background(), _userID, token, _ts)
		assert.Nil(t, err)
	}

	// 判断 token
	for i, token := range _tokens {
		err := store.Check(context.Background(), _userID, token)
		if i != len(_tokens)-1 {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}
	}

	// 删除不匹配的 token（Lua 脚本不匹配时返回 0 而非错误）
	err := store.Remove(context.Background(), _userID, "error-token")
	assert.Nil(t, err)

	err = store.Remove(context.Background(), _userID, _tokens[len(_tokens)-1])
	assert.Nil(t, err)

	// 清理 token
	err = store.Clean(context.Background(), _userID)
	assert.Nil(t, err)
}

func TestSingleRedisStore_SaveExpiredToken(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewSingleRedisStore(client)

	// token already expired
	expiredTs := time.Now().Add(-1 * time.Hour).Unix()
	err := store.Save(context.Background(), _userID, "expired-token", expiredTs)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "already expired")
}

func TestSingleRedisStore_SaveWithNilClient(t *testing.T) {
	store := NewSingleRedisStore(nil)

	err := store.Save(context.Background(), _userID, "token", time.Now().Add(time.Hour).Unix())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestSingleRedisStore_CheckWithNilClient(t *testing.T) {
	store := NewSingleRedisStore(nil)

	err := store.Check(context.Background(), _userID, "token")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestSingleRedisStore_RemoveWithNilClient(t *testing.T) {
	store := NewSingleRedisStore(nil)

	err := store.Remove(context.Background(), _userID, "token")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestSingleRedisStore_CleanWithNilClient(t *testing.T) {
	store := NewSingleRedisStore(nil)

	err := store.Clean(context.Background(), _userID)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}

func TestSingleRedisStore_CheckTokenMismatch(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewSingleRedisStore(client, WithSingleHashFunc(jwt.IdentityHash))
	_ts := time.Now().Add(24 * time.Hour).Unix()

	// Save token for user
	err := store.Save(context.Background(), _userID, "original-token", _ts)
	assert.Nil(t, err)

	// Check with a different token should return mismatch/revoked error
	err = store.Check(context.Background(), _userID, "different-token")
	assert.NotNil(t, err)
}

func TestSingleRedisStore_CheckTokenNotFound(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewSingleRedisStore(client, WithSingleHashFunc(jwt.IdentityHash))

	// No token saved, check should fail
	err := store.Check(context.Background(), _userID, "any-token")
	assert.NotNil(t, err)
}

func TestSingleRedisStore_CheckRedisError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 2})
	store := NewSingleRedisStore(client)

	// Save first
	ts := time.Now().Add(10 * time.Minute).Unix()
	err = store.Save(context.Background(), _userID, "test-token", ts)
	assert.Nil(t, err)

	// Close redis to cause errors
	mr.Close()

	err = store.Check(context.Background(), _userID, "test-token")
	assert.NotNil(t, err)
}

func TestSingleRedisStore_CleanError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 2})
	store := NewSingleRedisStore(client)

	// Close redis to cause errors
	mr.Close()

	err = store.Clean(context.Background(), _userID)
	assert.NotNil(t, err)
}

func TestSingleRedisStore_RemoveError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 2})
	store := NewSingleRedisStore(client)

	// Close redis to cause errors
	mr.Close()

	err = store.Remove(context.Background(), _userID, "test-token")
	assert.NotNil(t, err)
}
