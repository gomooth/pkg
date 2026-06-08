package jwtstore

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/xerror"

	"github.com/redis/go-redis/v9"
)

type singleRedisStore struct {
	client   *redis.Client
	hashFunc jwt.HashFunc
}

var _ jwt.StatefulStore = (*singleRedisStore)(nil)

// NewSingleRedisStore 单客户端有状态 token 存储
// 一个用户只能登录一个客户端，旧的客户端会被踢掉
func NewSingleRedisStore(client *redis.Client, opts ...func(*singleRedisStore)) jwt.StatefulStore {
	s := &singleRedisStore{
		client:   client,
		hashFunc: jwt.DefaultHashFunc,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithSingleHashFunc sets the hash function for token storage.
func WithSingleHashFunc(fn jwt.HashFunc) func(*singleRedisStore) {
	return func(s *singleRedisStore) {
		if fn != nil {
			s.hashFunc = fn
		}
	}
}

func (s *singleRedisStore) getKey(userID uint) string {
	return fmt.Sprintf("jwt:single:%d", userID)
}

var singleRemoveScript = redis.NewScript(`
	local val = redis.call('GET', KEYS[1])
	if val == ARGV[1] then
		return redis.call('DEL', KEYS[1])
	end
	return 0
`)

var singleCheckScript = redis.NewScript(`
	local val = redis.call('GET', KEYS[1])
	if val == false then
		return -1
	end
	-- 注意：~= 是非常量时间比较，理论上存在时序攻击风险。
	-- 但由于存储的已经是 SHA256 哈希值（非原始 token），攻击者需先破解哈希才能利用时序差，
	-- 实际攻击难度极高。若需进一步加固，可改用 HMAC-SHA256 对哈希值做二次处理。
	if val ~= ARGV[1] then
		return -2
	end
	return 1
`)

func (s *singleRedisStore) Save(ctx context.Context, userID uint, token string, expireTs int64) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)
	ttl := time.Until(time.Unix(expireTs, 0))
	if ttl <= 0 {
		return xerror.NewXCode(xcode.ErrJWTTokenExpired, "jwtstore: token already expired before save")
	}
	return s.client.Set(ctx, key, s.hashFunc(token), ttl).Err()
}

func (s *singleRedisStore) Check(ctx context.Context, userID uint, token string) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)
	result, err := singleCheckScript.Run(ctx, s.client, []string{key}, s.hashFunc(token)).Int64()
	if err != nil {
		return err
	}
	switch result {
	case 1:
		return nil
	case -1:
		return xerror.NewXCode(xcode.ErrJWTTokenRevoked, "jwtstore: token not found or revoked")
	case -2:
		return xerror.NewXCode(xcode.ErrJWTTokenRevoked, "jwtstore: token mismatch, token revoked by newer login")
	default:
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, fmt.Sprintf("jwtstore: unexpected check result %d", result))
	}
}

func (s *singleRedisStore) Remove(ctx context.Context, userID uint, token string) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)

	_, err := singleRemoveScript.Run(ctx, s.client, []string{key}, s.hashFunc(token)).Result()
	return err
}

func (s *singleRedisStore) Clean(ctx context.Context, userID uint) error {
	if s.client == nil {
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwtstore: redis client is nil")
	}

	key := s.getKey(userID)
	return s.client.Del(ctx, key).Err()
}
