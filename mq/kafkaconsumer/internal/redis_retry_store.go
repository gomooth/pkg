package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
	"github.com/redis/go-redis/v9"
)

// RedisRetryStore 抽象 Redis 重试状态存储操作，用户可实现此接口自定义存储细节
type RedisRetryStore interface {
	// ScheduleRetry 持久化重试项并调度到指定时间执行
	ScheduleRetry(ctx context.Context, item *RetryItem) error

	// FetchPending 原子地取出到期待重试项（并从调度集合中移除），最多 limit 条
	FetchPending(ctx context.Context, now time.Time, limit int) ([]*RetryItem, error)

	// RemoveRetry 移除重试项（成功或死信后调用）
	RemoveRetry(ctx context.Context, item *RetryItem) error

	// AtomicReschedule 原子地删除旧重试项并调度新重试项。
	// 先写入新项，再删除旧项，保证消息不丢失。
	AtomicReschedule(ctx context.Context, oldItem, newItem *RetryItem) error

	// LoadAllPending 加载所有待重试项（启动恢复用）
	LoadAllPending(ctx context.Context) ([]*RetryItem, error)

	// Close 释放资源
	Close() error
}

// defaultRedisRetryStore 基于 go-redis 的默认实现
type defaultRedisRetryStore struct {
	client    redis.UniversalClient
	keyPrefix string // "kafka:retry:{consumerGroup}"
	logger    *slog.Logger
}

// NewDefaultRedisRetryStore 创建默认 Redis 重试存储
func NewDefaultRedisRetryStore(client redis.UniversalClient, consumerGroup string) RedisRetryStore {
	return &defaultRedisRetryStore{
		client:    client,
		keyPrefix: fmt.Sprintf("kafka:retry:%s", consumerGroup),
		logger:    slog.Default(),
	}
}

func (s *defaultRedisRetryStore) scheduleKey() string {
	return s.keyPrefix + ":schedule"
}

func (s *defaultRedisRetryStore) dataKey(itemID string) string {
	return fmt.Sprintf("%s:msg:%s", s.keyPrefix, itemID)
}

// itemID 生成重试项的唯一标识。
// 格式为 topic:partition:offset，不含 consumerGroup，因为 keyPrefix 已包含 consumerGroup 前缀。
func (s *defaultRedisRetryStore) itemID(item *RetryItem) string {
	return fmt.Sprintf("%s:%d:%d", item.Topic, item.Partition, item.Offset)
}

func (s *defaultRedisRetryStore) ScheduleRetry(ctx context.Context, item *RetryItem) error {
	id := s.itemID(item)
	dk := s.dataKey(id)
	sk := s.scheduleKey()

	data, err := s.serializeItem(item)
	if err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	pipe := s.client.Pipeline()
	pipe.HSet(ctx, dk, data)
	pipe.ZAdd(ctx, sk, redis.Z{
		Score:  float64(item.NextRetryAt.UnixMilli()),
		Member: id,
	})
	_, err = pipe.Exec(ctx)
	return err
}

func (s *defaultRedisRetryStore) FetchPending(ctx context.Context, now time.Time, limit int) ([]*RetryItem, error) {
	sk := s.scheduleKey()
	nowMs := now.UnixMilli()

	// 步骤 1: Lua 原子获取并移除到期 ID（不取数据，避免嵌套数组解析问题）
	luaScript := `
local scheduleKey = KEYS[1]
local nowMs = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ids = redis.call('ZRANGEBYSCORE', scheduleKey, 0, nowMs, 'LIMIT', 0, limit)
local claimed = {}
for _, id in ipairs(ids) do
    local removed = redis.call('ZREM', scheduleKey, id)
    if removed == 1 then
        table.insert(claimed, id)
    end
end
return claimed
`

	result, err := s.client.Eval(ctx, luaScript, []string{sk}, nowMs, limit).Result()
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	// 解析 ID 列表
	var ids []string
	switch v := result.(type) {
	case []string:
		ids = v
	case []interface{}:
		ids = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				ids = append(ids, s)
			}
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	// 步骤 2: Pipeline 批量获取数据
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		dk := s.dataKey(id)
		cmds[i] = pipe.HGetAll(ctx, dk)
	}
	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	var items []*RetryItem
	for i, cmd := range cmds {
		data := cmd.Val()
		if len(data) == 0 {
			continue
		}
		item, err := s.deserializeItem(data)
		if err != nil {
			s.logger.Error("deserialize retry item failed", "id", ids[i], "error", err)
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *defaultRedisRetryStore) RemoveRetry(ctx context.Context, item *RetryItem) error {
	id := s.itemID(item)
	dk := s.dataKey(id)
	sk := s.scheduleKey()

	pipe := s.client.Pipeline()
	pipe.ZRem(ctx, sk, id)
	pipe.Del(ctx, dk)
	_, err := pipe.Exec(ctx)
	return err
}

// rescheduleScript 原子地删除旧项数据并写入新项数据和调度记录。
// KEYS[1] = 旧项数据 key, KEYS[2] = 新项数据 key, KEYS[3] = 调度 sorted set key
// ARGV[1] = 旧项 schedule member ID, ARGV[2] = 新项 schedule member ID
// ARGV[3] = 新项调度时间 (unix ms), ARGV[4..] = 新项数据字段 (key-value pairs)
var rescheduleScript = redis.NewScript(`
local oldDataKey = KEYS[1]
local newDataKey = KEYS[2]
local scheduleKey = KEYS[3]
local oldID = ARGV[1]
local newID = ARGV[2]
local newScore = tonumber(ARGV[3])

-- 先写入新项
local newDataFields = {}
for i = 4, #ARGV, 2 do
    table.insert(newDataFields, ARGV[i])
    table.insert(newDataFields, ARGV[i+1])
end
redis.call('HSET', newDataKey, unpack(newDataFields))
redis.call('ZADD', scheduleKey, newScore, newID)

-- 仅当新旧项不是同一条目时才删除旧项
-- 当 oldDataKey == newDataKey 时（重试场景：同一消息只是更新 Attempt/NextRetryAt），
-- HSET + ZADD 已完成原位更新，无需删除
if oldDataKey ~= newDataKey then
    redis.call('ZREM', scheduleKey, oldID)
    redis.call('DEL', oldDataKey)
elseif oldID ~= newID then
    -- 相同 data key 但不同 schedule member：移除旧调度记录
    redis.call('ZREM', scheduleKey, oldID)
end

return 1
`)

func (s *defaultRedisRetryStore) AtomicReschedule(ctx context.Context, oldItem, newItem *RetryItem) error {
	oldID := s.itemID(oldItem)
	newID := s.itemID(newItem)
	oldDataKey := s.dataKey(oldID)
	newDataKey := s.dataKey(newID)
	scheduleKey := s.scheduleKey()

	data, err := s.serializeItem(newItem)
	if err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	// 构建 flat ARGV: [oldID, newID, score, field1, val1, field2, val2, ...]
	argv := make([]any, 0, 3+len(data)*2)
	argv = append(argv, oldID)
	argv = append(argv, newID)
	argv = append(argv, newItem.NextRetryAt.UnixMilli())
	for k, v := range data {
		argv = append(argv, k)
		argv = append(argv, v)
	}

	_, err = rescheduleScript.Run(ctx, s.client, []string{oldDataKey, newDataKey, scheduleKey}, argv...).Result()
	return err
}

func (s *defaultRedisRetryStore) LoadAllPending(ctx context.Context) ([]*RetryItem, error) {
	sk := s.scheduleKey()

	// 获取所有 item ID
	ids, err := s.client.ZRange(ctx, sk, 0, -1).Result()
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	// Pipeline 批量获取数据
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, s.dataKey(id))
	}
	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	var items []*RetryItem
	var orphanIDs []string // schedule 有记录但数据 hash 不存在的 ID
	for i, cmd := range cmds {
		data := cmd.Val()
		if len(data) == 0 {
			orphanIDs = append(orphanIDs, ids[i])
			continue
		}

		item, err := s.deserializeItem(data)
		if err != nil {
			s.logger.Error("deserialize retry item failed", "id", ids[i], "error", err)
			continue
		}
		items = append(items, item)
	}

	// 清理孤立的 schedule 记录
	if len(orphanIDs) > 0 {
		pipe := s.client.Pipeline()
		for _, id := range orphanIDs {
			pipe.ZRem(ctx, sk, id)
		}
		_, _ = pipe.Exec(ctx)
	}

	return items, nil
}

func (s *defaultRedisRetryStore) Close() error {
	return nil
}

// redisItemData Redis hash 存储的序列化格式
type redisItemData struct {
	Value         string `redis:"value"`
	Key           string `redis:"key"`
	Headers       string `redis:"headers"`
	Topic         string `redis:"topic"`
	Partition     int32  `redis:"partition"`
	Offset        int64  `redis:"offset"`
	Attempt       uint   `redis:"attempt"`
	NextRetryAtMs int64  `redis:"nextRetryAt"`
	ConsumerGroup string `redis:"consumerGroup"`
}

func (s *defaultRedisRetryStore) serializeItem(item *RetryItem) (map[string]interface{}, error) {
	keyB64 := base64.StdEncoding.EncodeToString(item.Key)
	valueB64 := base64.StdEncoding.EncodeToString(item.Value)

	headersJSON, err := json.Marshal(item.Headers)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	return map[string]interface{}{
		"value":         valueB64,
		"key":           keyB64,
		"headers":       string(headersJSON),
		"topic":         item.Topic,
		"partition":     item.Partition,
		"offset":        item.Offset,
		"attempt":       item.Attempt,
		"nextRetryAt":   item.NextRetryAt.UnixMilli(),
		"consumerGroup": item.ConsumerGroup,
	}, nil
}

func (s *defaultRedisRetryStore) deserializeItem(data map[string]string) (*RetryItem, error) {
	value, err := base64.StdEncoding.DecodeString(data["value"])
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	key, err := base64.StdEncoding.DecodeString(data["key"])
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	var headers []HeaderKV
	if data["headers"] != "" {
		if err := json.Unmarshal([]byte(data["headers"]), &headers); err != nil {
			return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
		}
	}

	partition64, err := strconv.ParseInt(data["partition"], 10, 32)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	offset, err := strconv.ParseInt(data["offset"], 10, 64)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	attempt64, err := strconv.ParseUint(data["attempt"], 10, 32)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	nextRetryAtMs, err := strconv.ParseInt(data["nextRetryAt"], 10, 64)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	return &RetryItem{
		Topic:         data["topic"],
		Partition:     int32(partition64),
		Offset:        offset,
		Key:           key,
		Value:         value,
		Headers:       headers,
		Attempt:       uint(attempt64),
		NextRetryAt:   time.UnixMilli(nextRetryAtMs),
		ConsumerGroup: data["consumerGroup"],
	}, nil
}
