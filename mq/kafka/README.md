# mq/kafka — Kafka 消费者与生产者

基于 IBM/sarama 实现的 Kafka 客户端，提供消费者和生产者，均实现 `app.IApp` 接口，可通过 `app.Manager` 统一管理生命周期。

## 特性

- **消费组模式**：基于 Sarama ConsumerGroup，支持多实例水平扩展
- **双模式重试**：同步阻塞 / 异步非阻塞
- **可插拔重试存储**：内存水位线存储 / Redis 持久化存储
- **水位线机制**：MemoryRetryStore 下保证消息顺序性，只提交水位线以内的 offset
- **有序发送**：生产者支持按 partitionKey 有序发送
- **自动重连**：生产者内置断线重连机制
- **死信处理**：重试耗尽后支持自定义死信处理器
- **指标集成**：内置 OpenTelemetry 指标（消费/重试/死信/生产计数）

---

## 快速开始

### 生产者

```go
producer := kafka.NewProducer([]string{"localhost:9092"})

mgr := app.NewManager()
mgr.Register(producer)
mgr.MustRun(context.Background())

// 单条发送
err := producer.Produce(context.Background(), "orders", []byte(`{"id":1}`))

// 批量发送
err = producer.ProduceBatch(context.Background(), "orders",
    []byte(`{"id":2}`),
    []byte(`{"id":3}`),
)

// 有序发送（按 partitionKey 路由到同一 partition）
err = producer.ProduceOrdered(context.Background(), "orders",
    []byte("user-123"),   // partitionKey
    []byte(`{"id":4}`),
    []byte(`{"id":5}`),
)
```

### 消费者

```go
consumer := kafka.NewConsumer([]string{"localhost:9092"},
    kafka.WithConsumer("order-group", orderHandler, "orders"),
    kafka.WithMaxRetry(3),
)

mgr := app.NewManager()
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

---

## 消费者

### 创建

```go
consumer := kafka.NewConsumer(brokers []string, opts ...ConsumerOption)
```

### 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithMaxRetry(n)` | 最大重试次数，0=不重试 | 0 |
| `WithBackoff(b)` | 退避策略 | — |
| `WithHandlerTimeout(d)` | 单次 handler 超时 | 0（不限） |
| `WithRetryMode(mode)` | 重试模式 | `RetryModeSync` |
| `WithRetryWorkers(n)` | 异步重试 worker 数 | CPU 核数 |
| `WithRetryStore(store)` | 异步重试存储后端 | MemoryRetryStore |
| `WithRetryMaxQueueSize(n)` | 内存重试队列容量 | 10000 |
| `WithSyncRetryMaxTotalTimeout(d)` | 同步重试总超时 | 0（不限） |
| `WithFailedHandler(fn)` | 全局失败处理回调 | 日志记录 |
| `WithConsumeGroupFailedHandler(group, fn)` | 指定消费组的失败处理回调 | — |
| `WithPanicHandler(fn)` | panic 恢复后的回调 | 无 |
| `WithConsumerLogger(l)` | 日志器 | `slog.Default()` |
| `WithConsumerTimeout(d)` | 连接超时 | 5s |
| `WithConsumerSaramaConfig(cfg)` | 自定义 sarama.Config | 默认构建 |
| `WithConsumer(group, handler, topic, ...)` | 预注册消费者 | — |
| `WithConsumers(regs...)` | 批量预注册消费者 | — |

### 注册处理器

```go
// 方式一：预注册（推荐）
consumer := kafka.NewConsumer(brokers,
    kafka.WithConsumer("order-group", orderHandler, "orders"),
)

// 方式二：启动前注册（同一 group 可订阅多个 topic）
consumer := kafka.NewConsumer(brokers)
consumer.Register("order-group", orderHandler, "orders", "orders-retry")
```

### IHandler 接口

```go
type IHandler interface {
    Handle(ctx context.Context, topic string, message []byte) error
}
```

**函数适配器**：无需定义结构体，直接用函数

```go
handler := kafka.FuncHandler(func(ctx context.Context, topic string, msg []byte) error {
    fmt.Println(topic, string(msg))
    return nil
})
```

### 死信处理器（可选）

实现 `DeadLetterHandler` 接口，重试耗尽后自动调用：

```go
type OrderHandler struct{}

func (h *OrderHandler) Handle(ctx context.Context, topic string, msg []byte) error {
    return errors.New("处理失败")
}

func (h *OrderHandler) OnDeadLetter(ctx context.Context, topic string, msg []byte, lastErr error) error {
    // 写入死信表、发送告警等
    return nil
}
```

---

## 重试模式

### RetryModeSync（同步阻塞重试）

Handle 失败后在当前 goroutine 中立即重试，阻塞该 partition 的消费。

```go
consumer := kafka.NewConsumer(brokers,
    kafka.WithRetryMode(kafka.RetryModeSync),
    kafka.WithMaxRetry(3),
    kafka.WithSyncRetryMaxTotalTimeout(30*time.Second),
)
```

**适用场景**：消息重要性高、重试次数少、可接受 partition 暂停

### RetryModeAsync（异步非阻塞重试）

Handle 失败后将消息存入 RetryStore，由独立 worker 异步处理，不阻塞消费。

```go
// 内存水位线模式（进程重启后重试项丢失）
consumer := kafka.NewConsumer(brokers,
    kafka.WithRetryMode(kafka.RetryModeAsync),
    kafka.WithMaxRetry(3),
)

// Redis 持久化模式（进程重启后可恢复）
store := kafka.NewRedisRetryStore(redisClient)
consumer := kafka.NewConsumer(brokers,
    kafka.WithRetryMode(kafka.RetryModeAsync),
    kafka.WithMaxRetry(3),
    kafka.WithRetryStore(store),
)
```

**适用场景**：高吞吐、不可阻塞 partition 消费

### 对比

| 维度 | Sync | Async (Memory) | Async (Redis) |
|------|------|-----------------|----------------|
| 阻塞 | 阻塞 partition | 不阻塞 | 不阻塞 |
| 顺序保证 | 严格顺序 | 水位线保证 | 无保证 |
| 重启恢复 | 依赖 Kafka rebalance | 丢失重试项 | 恢复重试项 |
| 存储开销 | 无 | 内存 | Redis |
| 复杂度 | 低 | 中 | 高 |

---

## 重试存储

### MemoryRetryStore（内存水位线存储）

默认存储后端，同时实现 `RetryStore` 和 `WatermarkStore` 接口。

```go
store := kafka.NewMemoryRetryStore(
    kafka.WithMemoryMaxQueueSize(20000),
)
```

**核心机制**：
- 使用优先队列（最小堆）按 `NextRetryAt` 排序
- `WatermarkTracker` 分 16 个 shard 跟踪每个 partition 的水位线
- 只提交水位线以内的 offset，保证消息顺序性
- 超过容量限制时返回 `ErrRetryQueueFull`

### RedisRetryStore（Redis 持久化存储）

基于 Redis 的持久化存储，仅实现 `RetryStore` 接口。

```go
store := kafka.NewRedisRetryStore(redisClient,
    kafka.WithRedisKeyPrefix("myapp:kafka:retry"),
    kafka.WithRedisFetchLimit(50),
)
```

**核心机制**：
- 使用 Redis Sorted Set 存储调度信息（score = NextRetryAt）
- 使用 Redis Hash 存储消息数据
- 使用 Lua 脚本保证原子性
- 启动时自动恢复所有待重试项（`LoadAll`）
- offset 立即提交，不跟踪水位线

### RetryItem 结构

```go
type RetryItem struct {
    Topic         string
    Partition     int32
    Offset        int64
    Key           []byte
    Value         []byte
    Headers       []HeaderKV
    Attempt       int
    NextRetryAt   time.Time
    ConsumerGroup string
}
```

---

## 生产者

### 创建

```go
producer := kafka.NewProducer(brokers []string, opts ...ProducerOption)
```

### 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithProducerTimeout(d)` | 连接超时 | 5s |
| `WithProducerLogger(l)` | 日志器 | `slog.Default()` |
| `WithProducerSaramaConfig(cfg)` | 自定义 sarama.Config | 默认构建 |

### 发送消息

```go
// 单条发送
err := producer.Produce(ctx, "orders", []byte(`{"id":1}`))

// 批量发送（无序，分布在多个 partition）
err := producer.ProduceBatch(ctx, "orders",
    []byte(`{"id":2}`),
    []byte(`{"id":3}`),
)

// 有序发送（相同 partitionKey 的消息路由到同一 partition）
err := producer.ProduceOrdered(ctx, "orders",
    []byte("user-123"),   // partitionKey
    []byte(`{"id":4}`),
    []byte(`{"id":5}`),
)
```

### 自动重连

生产者内置断线重连机制：
- 发送失败时标记为断开状态
- 触发后台重连循环（指数退避，1s → 30s）
- 重连成功后自动恢复发送能力

---

## 生命周期管理

Consumer 和 Producer 均实现 `app.IApp` + `app.HealthChecker`，推荐通过 `app.Manager` 统一管理：

```go
consumer := kafka.NewConsumer(brokers,
    kafka.WithConsumer("order-group", orderHandler, "orders"),
    kafka.WithRetryMode(kafka.RetryModeAsync),
    kafka.WithMaxRetry(3),
)
producer := kafka.NewProducer(brokers)

mgr := app.NewManager()
mgr.Register(producer)
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

Manager 负责：
- 顺序启动所有服务
- 捕获 SIGINT/SIGTERM 信号
- 逆序关闭所有服务（含超时保护，消费者关闭前排空重试队列）
- 统一健康检查

---

## 内部原理

### 状态机

Consumer 和 Producer 均使用 CAS 无锁状态机：

```
Idle(0) ──CAS──▶ Running(1) ──CAS──▶ ShuttingDown(2) ──▶ Closed(3)
```

### 消费循环容错

消费循环内置指数退避 + 最大错误次数保护：
- 连续失败 50 次后暂停 5 分钟
- 成功一次即重置计数器

### 异步重试流程

```
消息消费失败 → 存入 RetryStore → Worker 轮询/通知取回 → 重新执行 Handle
                                                    ↓ 成功
                                          WatermarkStore.MarkSuccess
                                                    ↓ 提交
                                          commitWatermark(session)
```

### 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `kafka.consumer.messages` | Int64Counter | 成功消费消息数 |
| `kafka.consumer.retries` | Int64Counter | 重试次数 |
| `kafka.consumer.dead_letters` | Int64Counter | 死信消息数 |
| `kafka.producer.messages` | Int64Counter | 成功生产消息数 |
| `kafka.producer.errors` | Int64Counter | 生产错误数 |
