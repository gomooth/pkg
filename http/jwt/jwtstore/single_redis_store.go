package jwtstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/save95/xerror"

	"github.com/redis/go-redis/v9"

	"github.com/gomooth/pkg/http/jwt"
)

type singleRedisStore struct {
	client *redis.Client
}

// NewSingleRedisStore 单客户端有状态 token 存储
// 一个用户只能登录一个客户端，旧的客户端会被踢掉
func NewSingleRedisStore(client *redis.Client) jwt.StatefulStore {
	return &singleRedisStore{
		client: client,
	}
}

func (s *singleRedisStore) getKey(userID uint) string {
	return fmt.Sprintf("jwt:single:%d", userID)
}

func (s *singleRedisStore) Save(userID uint, token string, expireTs int64) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)
	expire := time.Second * time.Duration(expireTs)
	return s.client.Set(ctx, key, token, expire).Err()
}

func (s *singleRedisStore) Check(userID uint, token string) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)
	str, err := s.client.Get(ctx, key).Result()
	if nil != err {
		return err
	}

	if str != token {
		return errors.New("token error")
	}

	return nil
}

func (s *singleRedisStore) Remove(userID uint, token string) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)

	str, err := s.client.Get(ctx, key).Result()
	if nil != err {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}

	if str == token {
		return s.client.Del(ctx, key).Err()
	}

	return errors.New("token err")
}

func (s *singleRedisStore) Clean(userID uint) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)
	return s.client.Del(ctx, key).Err()
}
