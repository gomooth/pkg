package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// popLuaScript 原子性 Pop 脚本：先从 backup 列表取数据，backup 为空则从主队列阻塞取并写入 backup
// 使用 BLMOVE 替代已废弃的 BRPOPLPUSH（Redis 6.2+ 废弃）
var popLuaScript = redis.NewScript(`
local backup_key = KEYS[2]
local main_key = KEYS[1]
local timeout = tonumber(ARGV[1])

-- 先尝试从 backup 获取
local bak = redis.call('RPOP', backup_key)
if bak then
    return bak
end

-- backup 为空，从主队列阻塞获取
-- BLMOVE source destination LEFT RIGHT timeout 等价于 BRPOPLPUSH source destination timeout
local result = redis.call('BLMOVE', main_key, backup_key, 'LEFT', 'RIGHT', timeout)
return result
`)

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
	return err
}

func (q *queue) Pop(ctx context.Context) (string, error) {
	// 使用 Lua 脚本原子性完成：先从 backup 取，backup 为空则从主队列阻塞取并写入 backup
	result, err := popLuaScript.Run(ctx, q.redisClient, []string{q.name, q.backupName()}, int64(q.timeout.Seconds())).Text()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}

	if len(result) == 0 {
		return "", nil
	}

	return result, nil
}

func (q *queue) Close() error {
	if q.redisClient != nil {
		return q.redisClient.Close()
	}
	return nil
}

func (q *queue) backupName() string {
	return fmt.Sprintf("%s_backup", q.name)
}
