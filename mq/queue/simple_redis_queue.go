package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type queue struct {
	name        string
	timeout     time.Duration
	redisClient *redis.Client
}

// RedisQueueConfig Redis 队列参数
type RedisQueueConfig struct {
	Addr     string
	Password string
	DB       int
	Timeout  time.Duration
}

// NewSimpleRedis 创建简单的 Redis 队列
func NewSimpleRedis(cnf *RedisQueueConfig, name string) IQueue {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cnf.Addr,
		Password: cnf.Password,
		DB:       cnf.DB,
	})

	timeout := 15 * time.Second
	if cnf.Timeout > 0 {
		timeout = cnf.Timeout
	}

	return &queue{
		name:        fmt.Sprintf("queue:%s", name),
		timeout:     timeout,
		redisClient: redisClient,
	}
}

func (q *queue) Push(ctx context.Context, value string) error {
	_, err := q.redisClient.LPush(ctx, q.name, value).Result()
	if nil != err {
		return err
	}

	// 推送完成就立即释放，防止占用过多的链接
	_ = q.redisClient.Close()

	return err
}

func (q *queue) Pop(ctx context.Context) (string, error) {
	// 先从备份 List 获取数据（保证消息的可靠性）
	bak, err := q.redisClient.RPop(ctx, q.backupName()).Result()
	if nil != err {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	if len(bak) > 0 {
		return bak, nil
	}

	// 取出数据，并写入备份 List，防止当机丢失数据
	str, err := q.redisClient.BRPopLPush(ctx, q.name, q.backupName(), q.timeout).Result()
	if nil != err {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}

	return str, nil
}

func (q *queue) backupName() string {
	return fmt.Sprintf("%s_backup", q.name)
}
