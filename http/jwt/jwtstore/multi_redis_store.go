package jwtstore

import (
	"context"
	"fmt"
	"time"

	"github.com/save95/xerror"

	"github.com/gomooth/pkg/http/jwt"

	"github.com/redis/go-redis/v9"
)

type multiRedisStore struct {
	client *redis.Client
}

// NewMultiRedisStore 有状态 token 存储
func NewMultiRedisStore(client *redis.Client) jwt.StatefulStore {
	return &multiRedisStore{
		client: client,
	}
}

func (s *multiRedisStore) getKey(userID uint) string {
	return fmt.Sprintf("jwt:multi:%d", userID)
}

func (s *multiRedisStore) cleanExpired(ctx context.Context, userID uint) error {
	key := s.getKey(userID)

	minScore := "0"
	maxScore := fmt.Sprintf("%d", time.Now().Unix())

	return s.client.ZRemRangeByScore(ctx, key, minScore, maxScore).Err()
}

func (s *multiRedisStore) Save(userID uint, token string, expireTs int64) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)

	// 清理过期的数据
	if err := s.cleanExpired(ctx, userID); nil != err {
		return err
	}

	err := s.client.ZAddArgs(ctx, key, redis.ZAddArgs{
		Ch: true,
		Members: []redis.Z{
			{
				Score:  float64(expireTs),
				Member: token,
			},
		},
	}).Err()
	if nil != err {
		return err
	}

	expire := time.Second * time.Duration(expireTs-time.Now().Unix())
	_ = s.client.Expire(ctx, key, expire).Err()
	return nil
}

func (s *multiRedisStore) Check(userID uint, token string) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)

	// 如果不存在，则返回 redis.Nil 错误，存在则返回分数
	return s.client.ZScore(ctx, key, token).Err()
}

func (s *multiRedisStore) Remove(userID uint, token string) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)

	return s.client.ZRem(ctx, key, token).Err()
}

func (s *multiRedisStore) Clean(userID uint) error {
	if s.client == nil {
		return xerror.New("jwt store redis client is nil")
	}

	ctx := context.Background()
	key := s.getKey(userID)

	return s.client.Del(ctx, key).Err()
}
