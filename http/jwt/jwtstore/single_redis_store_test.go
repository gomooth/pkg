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
