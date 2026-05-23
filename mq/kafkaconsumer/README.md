# kafkaconsumer — Kafka 消费者

基于 Sarama 的 Kafka 消费者组，支持消息重试、死信处理和三种重试模式。

## 重试模式

| 模式 | 说明 | 依赖 | 重启行为 |
|------|------|------|---------|
| `RetryModeSync` | 同步阻塞重试（默认） | 无 | 无特殊处理 |
| `RetryModeAsyncWatermark` | 异步重试 + 水位线 offset | 无 | Kafka 重投递未提交消息 |
| `RetryModeAsyncRedis` | 异步重试 + Redis 持久化 | Redis | 从 Redis 恢复重试项 |

同步模式下，消息处理失败时在消费协程内重试，阻塞 partition。异步模式将失败消息转入后台重试，不阻塞消费。

## 接口

### IHandler — 消息处理器

```go
type IHandler interface {
    Handle(topic string, msg []byte) error
}
```

### DeadLetterHandler — 死信处理器（可选）

实现此接口的 Handler 在重试耗尽后走死信逻辑：

```go
type DeadLetterHandler interface {
    OnDeadLetter(topic string, msg []byte, err error) error
}
```

## 使用示例

### 基本用法（同步重试）

```go
srv := kafkaconsumer.NewServer(
    []string{"localhost:9092"},
    kafkaconsumer.WithMaxRetry(3),
    kafkaconsumer.WithBackoff(&retry.ExponentialDelay{
        Base: time.Second, Max: 30 * time.Second, Jitter: true,
    }),
)

handler := kafkaconsumer.FuncHandler(func(topic string, msg []byte) error {
    return processMessage(msg)
})

srv.Register("my-group", handler, "my-topic")
srv.Run(context.Background())
```

### 方案B：异步重试 + 水位线

```go
srv := kafkaconsumer.NewServer(
    []string{"localhost:9092"},
    kafkaconsumer.WithRetryMode(internal.RetryModeAsyncWatermark),
    kafkaconsumer.WithRetryWorkers(4),
    kafkaconsumer.WithMaxRetry(3),
)
```

失败消息入内存优先队列，Worker 协程异步重试。仅提交水位线以内的 offset，保证重启后 Kafka 重投递未完成的消息。不依赖外部存储，但重启时可能有少量重复处理。

### 方案C：异步重试 + Redis 持久化

```go
store := internal.NewDefaultRedisRetryStore(redisClient, "my-group")
srv := kafkaconsumer.NewServer(
    []string{"localhost:9092"},
    kafkaconsumer.WithRetryMode(internal.RetryModeAsyncRedis),
    kafkaconsumer.WithRetryRedisStore(store),
    kafkaconsumer.WithMaxRetry(3),
)
```

失败消息持久化到 Redis 后立即提交 offset，不阻塞 partition。重启后从 Redis 恢复未完成的重试项。需要 Redis 依赖。

### 死信处理

```go
type orderHandler struct{}

func (h *orderHandler) Handle(topic string, msg []byte) error {
    return processOrder(msg)
}

func (h *orderHandler) OnDeadLetter(topic string, msg []byte, err error) error {
    return saveToDeadLetterTable(topic, msg, err)
}
```

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithMaxRetry(n)` | 最大重试次数 | 0 |
| `WithBackoff(strategy)` | 退避策略 | ExponentialDelay{Base:10s, Max:5min} |
| `WithRetryMode(mode)` | 重试模式 | RetryModeSync |
| `WithRetryWorkers(n)` | 异步 Worker 数量 | runtime.NumCPU() |
| `WithRetryRedisStore(store)` | Redis 重试存储 | nil |
| `WithLogger(logger)` | 日志 | 控制台日志 |
| `WithPanicHandler(fn)` | Panic 拦截器 | nil |

## Redis 重试存储

方案C 需要实现 `internal.RedisRetryStore` 接口，也可使用默认的 go-redis 实现：

```go
store := internal.NewDefaultRedisRetryStore(redisClient, "consumerGroup")
```

默认实现使用 Sorted Set 调度重试时间 + Hash 存储消息数据，通过 Lua 脚本保证原子操作。
