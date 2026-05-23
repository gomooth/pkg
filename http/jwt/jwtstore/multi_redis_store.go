package jwtstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/xerror"

	"github.com/redis/go-redis/v9"
)

// luaSave 原子地清理过期 token、添加新 token、刷新 key TTL
// KEYS[1] = jwt:multi:{userID}
// ARGV[1] = score (expire timestamp)
// ARGV[2] = member (hashed token)
// ARGV[3] = ttl (seconds)
// ARGV[4] = now (unix timestamp, for cleanup threshold)
var luaSave = redis.NewScript(`
local key = KEYS[1]
local score = tonumber(ARGV[1])
local member = ARGV[2]
local ttl = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
-- 清理过期 token
redis.call('ZREMRANGEBYSCORE', key, 0, now)
-- 添加新 token
redis.call('ZADD', key, score, member)
-- 刷新 key 的 TTL
redis.call('EXPIRE', key, ttl)
return 1
`)

type multiRedisStore struct {
	client   *redis.Client
	hashFunc jwt.HashFunc
}

// NewMultiRedisStore 有状态 token 存储
func NewMultiRedisStore(client *redis.Client, opts ...func(*multiRedisStore)) jwt.StatefulStore {
	s := &multiRedisStore{
		client:   client,
		hashFunc: jwt.DefaultHashFunc,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithMultiHashFunc sets the hash function for token storage.
func WithMultiHashFunc(fn jwt.HashFunc) func(*multiRedisStore) {
	return func(s *multiRedisStore) {
		if fn != nil {
			s.hashFunc = fn
		}
	}
}

func (s *multiRedisStore) getKey(userID uint) string {
	return fmt.Sprintf("jwt:multi:%d", userID)
}

func (s *multiRedisStore) Save(ctx context.Context, userID uint, token string, expireTs int64) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	ttl := time.Until(time.Unix(expireTs, 0))
	if ttl <= 0 {
		// token 已过期，仅清理过期项
		key := s.getKey(userID)
		now := fmt.Sprintf("%d", time.Now().Unix())
		if err := s.client.ZRemRangeByScore(ctx, key, "0", now).Err(); err != nil {
			slog.Warn("jwtstore: clean expired tokens failed", slog.String("component", "jwtstore"), slog.Uint64("userID", uint64(userID)), slog.Any("error", err))
		}
		return xerror.NewXCode(xcode.ErrJWTTokenExpired, "jwtstore: token already expired before save")
	}

	key := s.getKey(userID)
	_, err := luaSave.Run(ctx, s.client, []string{key},
		expireTs,
		s.hashFunc(token),
		int(ttl.Seconds()),
		time.Now().Unix(),
	).Result()
	if err != nil {
		return err
	}

	return nil
}

func (s *multiRedisStore) Check(ctx context.Context, userID uint, token string) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)

	_, err := s.client.ZScore(ctx, key, s.hashFunc(token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return xerror.NewXCode(xcode.ErrJWTTokenRevoked, "jwtstore: token not found or revoked")
		}
		return err
	}
	return nil
}

func (s *multiRedisStore) Remove(ctx context.Context, userID uint, token string) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)

	return s.client.ZRem(ctx, key, s.hashFunc(token)).Err()
}

func (s *multiRedisStore) Clean(ctx context.Context, userID uint) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)

	return s.client.Del(ctx, key).Err()
}
