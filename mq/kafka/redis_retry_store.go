package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/kafka/internal"
	"github.com/gomooth/xerror"
	"github.com/redis/go-redis/v9"
)

// 编译时接口检查
var _ RetryStore = (*RedisRetryStore)(nil)

// RedisStoreOption Redis 重试存储配置选项
type RedisStoreOption func(*redisStoreConfig)

type redisStoreConfig struct {
	keyPrefix  string
	fetchLimit int
}

// WithRedisKeyPrefix 设置 Redis key 前缀（默认 "kafka:retry"）
func WithRedisKeyPrefix(prefix string) RedisStoreOption {
	return func(c *redisStoreConfig) {
		c.keyPrefix = prefix
	}
}

// WithRedisFetchLimit 设置 Fetch 单次获取上限（默认 100）
func WithRedisFetchLimit(n int) RedisStoreOption {
	return func(c *redisStoreConfig) {
		c.fetchLimit = n
	}
}

// RedisRetryStore 基于 Redis 的重试存储，仅实现 RetryStore 接口。
// Redis 模式下 offset 立即提交，无需水位线跟踪。
type RedisRetryStore struct {
	client     redis.UniversalClient
	keyPrefix  string
	fetchLimit int
}

// NewRedisRetryStore 创建 Redis 重试存储实例
func NewRedisRetryStore(client redis.UniversalClient, opts ...RedisStoreOption) *RedisRetryStore {
	cfg := redisStoreConfig{
		keyPrefix:  "kafka:retry",
		fetchLimit: 100,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &RedisRetryStore{
		client:     client,
		keyPrefix:  cfg.keyPrefix,
		fetchLimit: cfg.fetchLimit,
	}
}

// scheduleKey 返回 sorted set 的 key
func (s *RedisRetryStore) scheduleKey() string {
	return s.keyPrefix + ":schedule"
}

// dataKey 返回消息数据的 key
func (s *RedisRetryStore) dataKey(topic string, partition int32, offset int64) string {
	return fmt.Sprintf("%s:msg:%s:%d:%d", s.keyPrefix, topic, partition, offset)
}

// Schedule 将消息加入重试队列
func (s *RedisRetryStore) Schedule(ctx context.Context, item *RetryItem) error {
	internalItem := toInternalRetryItem(item)
	fields := s.toRedisFields(internalItem)
	dKey := s.dataKey(item.Topic, item.Partition, item.Offset)
	score := float64(item.NextRetryAt.UnixMilli())

	pipe := s.client.Pipeline()
	pipe.HSet(ctx, dKey, fields)
	pipe.ZAdd(ctx, s.scheduleKey(), redis.Z{
		Score:  score,
		Member: dKey,
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}
	return nil
}

// Fetch 获取已到期的重试消息
func (s *RedisRetryStore) Fetch(ctx context.Context, now time.Time, limit int) ([]*RetryItem, error) {
	if limit <= 0 {
		limit = s.fetchLimit
	}
	if limit > s.fetchLimit {
		limit = s.fetchLimit
	}

	// 使用 Lua 脚本原子地获取并移除到期项
	keys, err := fetchScript.Run(ctx, s.client,
		[]string{s.scheduleKey()},
		now.UnixMilli(),
		limit,
	).StringSlice()
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// 批量获取消息数据
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	var items []*RetryItem
	var dataKeysToClean []string
	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil {
			dataKeysToClean = append(dataKeysToClean, keys[i])
			continue
		}
		if len(fields) == 0 {
			dataKeysToClean = append(dataKeysToClean, keys[i])
			continue
		}
		internalItem, err := s.fromRedisFields(fields)
		if err != nil {
			dataKeysToClean = append(dataKeysToClean, keys[i])
			continue
		}
		items = append(items, toPublicRetryItem(internalItem))
	}

	// 清理孤立的数据 key
	if len(dataKeysToClean) > 0 {
		cleanPipe := s.client.Pipeline()
		for _, key := range dataKeysToClean {
			cleanPipe.Del(ctx, key)
		}
		cleanPipe.Exec(ctx) // 忽略清理错误
	}

	return items, nil
}

// Remove 从重试队列中移除消息
func (s *RedisRetryStore) Remove(ctx context.Context, item *RetryItem) error {
	dKey := s.dataKey(item.Topic, item.Partition, item.Offset)

	pipe := s.client.Pipeline()
	pipe.ZRem(ctx, s.scheduleKey(), dKey)
	pipe.Del(ctx, dKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}
	return nil
}

// Reschedule 将旧的重试项替换为新的重试项
func (s *RedisRetryStore) Reschedule(ctx context.Context, oldItem, newItem *RetryItem) error {
	oldDKey := s.dataKey(oldItem.Topic, oldItem.Partition, oldItem.Offset)
	newDKey := s.dataKey(newItem.Topic, newItem.Partition, newItem.Offset)
	internalNewItem := toInternalRetryItem(newItem)
	newFields := s.toRedisFields(internalNewItem)
	newScore := float64(newItem.NextRetryAt.UnixMilli())

	args := []interface{}{oldDKey, newDKey, newScore}
	args = append(args, s.flattenFields(newFields)...)
	err := rescheduleScript.Run(ctx, s.client,
		[]string{s.scheduleKey(), oldDKey, newDKey},
		args...,
	).Err()
	if err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}
	return nil
}

// LoadAll 加载所有待重试消息（用于启动时恢复）
func (s *RedisRetryStore) LoadAll(ctx context.Context) ([]*RetryItem, error) {
	// 获取所有调度 key
	keys, err := s.client.ZRange(ctx, s.scheduleKey(), 0, -1).Result()
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// 批量获取消息数据
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrMQConsume)
	}

	var items []*RetryItem
	var orphanKeys []string
	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			orphanKeys = append(orphanKeys, keys[i])
			continue
		}
		internalItem, err := s.fromRedisFields(fields)
		if err != nil {
			orphanKeys = append(orphanKeys, keys[i])
			continue
		}
		items = append(items, toPublicRetryItem(internalItem))
	}

	// 清理孤立的数据 key
	if len(orphanKeys) > 0 {
		cleanPipe := s.client.Pipeline()
		for _, key := range orphanKeys {
			cleanPipe.Del(ctx, key)
			cleanPipe.ZRem(ctx, s.scheduleKey(), key)
		}
		cleanPipe.Exec(ctx) // 忽略清理错误
	}

	return items, nil
}

// Close 关闭存储（Redis 模式由调用方管理连接生命周期）
func (s *RedisRetryStore) Close() error {
	return nil
}

// ==================== 序列化 ====================

// toRedisFields 将内部 RetryItem 转换为 Redis hash 字段
func (s *RedisRetryStore) toRedisFields(item *internal.RetryItem) map[string]interface{} {
	fields := map[string]interface{}{
		"topic":         item.Topic,
		"partition":     strconv.FormatInt(int64(item.Partition), 10),
		"offset":        strconv.FormatInt(item.Offset, 10),
		"attempt":       strconv.FormatUint(uint64(item.Attempt), 10),
		"nextRetryAt":   strconv.FormatInt(item.NextRetryAt.UnixMilli(), 10),
		"consumerGroup": item.ConsumerGroup,
	}

	if len(item.Key) > 0 {
		fields["key"] = base64.StdEncoding.EncodeToString(item.Key)
	}
	if len(item.Value) > 0 {
		fields["value"] = base64.StdEncoding.EncodeToString(item.Value)
	}
	if len(item.Headers) > 0 {
		headers := make([]map[string]string, len(item.Headers))
		for i, h := range item.Headers {
			headers[i] = map[string]string{
				"key":   base64.StdEncoding.EncodeToString(h.Key),
				"value": base64.StdEncoding.EncodeToString(h.Value),
			}
		}
		if data, err := json.Marshal(headers); err == nil {
			fields["headers"] = string(data)
		}
	}

	return fields
}

// fromRedisFields 将 Redis hash 字段转换为内部 RetryItem
func (s *RedisRetryStore) fromRedisFields(fields map[string]string) (*internal.RetryItem, error) {
	partition, err := strconv.ParseInt(fields["partition"], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid partition: %w", err)
	}
	offset, err := strconv.ParseInt(fields["offset"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid offset: %w", err)
	}
	attempt, err := strconv.ParseUint(fields["attempt"], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid attempt: %w", err)
	}
	nextRetryAtMs, err := strconv.ParseInt(fields["nextRetryAt"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid nextRetryAt: %w", err)
	}

	item := &internal.RetryItem{
		Topic:         fields["topic"],
		Partition:     int32(partition),
		Offset:        offset,
		Attempt:       uint(attempt),
		NextRetryAt:   time.UnixMilli(nextRetryAtMs),
		ConsumerGroup: fields["consumerGroup"],
	}

	if v, ok := fields["key"]; ok {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid key base64: %w", err)
		}
		item.Key = decoded
	}

	if v, ok := fields["value"]; ok {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid value base64: %w", err)
		}
		item.Value = decoded
	}

	if v, ok := fields["headers"]; ok && v != "" {
		var headers []map[string]string
		if err := json.Unmarshal([]byte(v), &headers); err != nil {
			return nil, fmt.Errorf("invalid headers json: %w", err)
		}
		item.Headers = make([]internal.HeaderKV, len(headers))
		for i, h := range headers {
			key, err := base64.StdEncoding.DecodeString(h["key"])
			if err != nil {
				return nil, fmt.Errorf("invalid header key base64: %w", err)
			}
			val, err := base64.StdEncoding.DecodeString(h["value"])
			if err != nil {
				return nil, fmt.Errorf("invalid header value base64: %w", err)
			}
			item.Headers[i] = internal.HeaderKV{Key: key, Value: val}
		}
	}

	return item, nil
}

// flattenFields 将 map[string]interface{} 展平为 []interface{} 用于 Lua 脚本
func (s *RedisRetryStore) flattenFields(fields map[string]interface{}) []interface{} {
	result := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		result = append(result, k, v)
	}
	return result
}

// ==================== Lua 脚本 ====================

// fetchScript 原子地获取并移除到期项
// KEYS[1] = schedule key
// ARGV[1] = max score (now.UnixMilli())
// ARGV[2] = limit
var fetchScript = redis.NewScript(`
local scheduleKey = KEYS[1]
local maxScore = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])

local items = redis.call('ZRANGEBYSCORE', scheduleKey, '-inf', maxScore, 'LIMIT', 0, limit)
if #items == 0 then
    return {}
end

for _, item in ipairs(items) do
    redis.call('ZREM', scheduleKey, item)
end

return items
`)

// rescheduleScript 原子地将旧项替换为新项
// KEYS[1] = schedule key
// KEYS[2] = old data key
// KEYS[3] = new data key
// ARGV[1] = old data key (for ZREM)
// ARGV[2] = new data key (for ZADD)
// ARGV[3] = new score
// ARGV[4..] = field/value pairs for new data
var rescheduleScript = redis.NewScript(`
local scheduleKey = KEYS[1]
local oldDKey = KEYS[2]
local newDKey = KEYS[3]

-- Remove old item from schedule
redis.call('ZREM', scheduleKey, oldDKey)

-- If old and new data keys are different, delete the old one
if oldDKey ~= newDKey then
    redis.call('DEL', oldDKey)
end

-- Add new item to data hash (overwrites if same key)
local fields = {}
for i = 4, #ARGV, 2 do
    table.insert(fields, ARGV[i])
    table.insert(fields, ARGV[i+1])
end
if #fields > 0 then
    redis.call('HSET', newDKey, unpack(fields))
end

-- Add new item to schedule with new score
redis.call('ZADD', scheduleKey, tonumber(ARGV[3]), newDKey)

return 1
`)
