# mq/redis — Redis 队列消费者与生产者

基于 Redis List 实现的消息队列，提供消费者和生产者，均实现 `app.IApp` 接口，可通过 `app.Manager` 统一管理生命周期。

## 特性

- **原子性消费**：使用 Lua 脚本 + BLMOVE 实现 Pop，消息不丢失
- **备份机制**：每个队列对应 `backup` 列表，保证处理事务性
- **双模式重试**：同步阻塞重试 / 再入队重试
- **死信处理**：重试耗尽后支持自定义死信处理器
- **Pipeline 优化**：生产者批量推送使用 Pipeline
- **指标集成**：内置 OpenTelemetry 指标（消费/重试/死信/生产计数）

---

## 快速开始

### 生产者

```go
producer := redis.NewProducer("localhost:6379")

mgr := app.NewManager()
mgr.Register(producer)
mgr.MustRun(context.Background())

// 发送单条消息
err := producer.Produce(context.Background(), "orders", []byte(`{"id":1}`))

// 批量发送
err = producer.ProduceBatch(context.Background(), "orders",
    []byte(`{"id":2}`),
    []byte(`{"id":3}`),
)
```

### 消费者

```go
consumer := redis.NewConsumer("localhost:6379",
    redis.WithConsumer("orders", orderHandler),
    redis.WithMaxRetry(5),
)

mgr := app.NewManager()
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

---

## 消费者

### 创建

```go
consumer := redis.NewConsumer(addr string, opts ...ConsumerOption)
```

### 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithMaxRetry(n)` | 最大重试次数，0=不重试 | 3 |
| `WithBackoff(b)` | 退避策略 | `ExponentialDelay{Base:1s, Max:5min}` |
| `WithHandlerTimeout(d)` | 单次 handler 超时 | 0（不限） |
| `WithRetryMode(mode)` | 重试模式 | `RetryModeSync` |
| `WithEmptyQueueSleep(d)` | 队列空时休眠间隔 | 1s |
| `WithQueuePrefix(prefix)` | 队列名前缀 | `"queue:"` |
| `WithFailedHandler(fn)` | 重试耗尽后的回调 | 日志记录 |
| `WithPanicHandler(fn)` | panic 恢复后的回调 | 无 |
| `WithConsumerLogger(l)` | 日志器 | `slog.Default()` |
| `WithConsumerRedisConfig(opt)` | Redis 连接配置 | 默认配置 |
| `WithConsumer(queue, handler)` | 预注册消费者 | — |
| `WithConsumers(regs...)` | 批量预注册消费者 | — |

### 注册处理器

```go
// 方式一：预注册（推荐）
consumer := redis.NewConsumer(addr,
    redis.WithConsumer("orders", orderHandler),
)

// 方式二：启动前注册
consumer := redis.NewConsumer(addr)
consumer.Register("orders", orderHandler)
```

### IHandler 接口

```go
type IHandler interface {
    Handle(ctx context.Context, queue string, message []byte) error
}
```

**函数适配器**：无需定义结构体，直接用函数

```go
handler := redis.FuncHandler(func(ctx context.Context, queue string, msg []byte) error {
    fmt.Println(string(msg))
    return nil
})
```

### 死信处理器（可选）

实现 `DeadLetterHandler` 接口，重试耗尽后自动调用：

```go
type OrderHandler struct{}

func (h *OrderHandler) Handle(ctx context.Context, queue string, msg []byte) error {
    return errors.New("处理失败")
}

func (h *OrderHandler) OnDeadLetter(ctx context.Context, queue string, msg []byte, lastErr error) error {
    // 写入死信表、发送告警等
    return nil
}
```

---

## 重试模式

### RetryModeSync（同步阻塞重试）

Handle 失败后在当前循环中立即重试，阻塞该队列的消费。

```go
consumer := redis.NewConsumer(addr,
    redis.WithRetryMode(redis.RetryModeSync),
    redis.WithMaxRetry(3),
)
```

**适用场景**：消息重要性高，需要快速重试

### RetryModeRequeue（再入队重试）

Handle 失败后将消息 Push 回队列尾部，不阻塞当前消费者。使用 `AttemptTracker`（基于 SHA256）跟踪重试次数，配合退避策略延迟再入队。

```go
consumer := redis.NewConsumer(addr,
    redis.WithRetryMode(redis.RetryModeRequeue),
    redis.WithMaxRetry(3),
)
```

**适用场景**：分布式消费、避免单点阻塞

### 对比

| 维度 | Sync | Requeue |
|------|------|---------|
| 阻塞 | 阻塞当前队列消费 | 不阻塞 |
| 顺序 | 严格顺序 | 可能乱序 |
| 分布式 | 单消费者 | 多消费者 |
| 复杂度 | 低 | 中（需要 AttemptTracker） |

---

## 生产者

### 创建

```go
producer := redis.NewProducer(addr string, opts ...ProducerOption)
```

### 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithProducerLogger(l)` | 日志器 | `slog.Default()` |
| `WithProducerRedisConfig(opt)` | Redis 连接配置 | 默认配置 |
| `WithProducerQueuePrefix(prefix)` | 队列名前缀 | `"queue:"` |

### 发送消息

```go
// 单条
err := producer.Produce(ctx, "orders", []byte(`{"id":1}`))

// 批量（Pipeline 优化）
err := producer.ProduceBatch(ctx, "orders",
    []byte(`{"id":2}`),
    []byte(`{"id":3}`),
)
```

---

## 生命周期管理

Consumer 和 Producer 均实现 `app.IApp` + `app.HealthChecker`，推荐通过 `app.Manager` 统一管理：

```go
consumer := redis.NewConsumer(addr,
    redis.WithConsumer("orders", orderHandler),
)
producer := redis.NewProducer(addr)

mgr := app.NewManager()
mgr.Register(producer)
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

Manager 负责：
- 顺序启动所有服务
- 捕获 SIGINT/SIGTERM 信号
- 逆序关闭所有服务（含超时保护）
- 统一健康检查

---

## 内部原理

### 队列数据结构

```
queue:orders        ← 主队列（List）
queue:orders_backup ← 备份队列（List）
```

### 消费流程

1. `BLMOVE queue:orders queue:orders_backup LEFT RIGHT 5`（原子性 Pop + 备份）
2. 执行 Handler 处理消息
3. 处理成功：从 backup 列表移除
4. 处理失败：根据重试模式决定后续行为

### 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `redis.consumer.messages` | Int64Counter | 成功消费消息数 |
| `redis.consumer.retries` | Int64Counter | 重试次数 |
| `redis.consumer.dead_letters` | Int64Counter | 死信消息数 |
| `redis.producer.messages` | Int64Counter | 成功生产消息数 |
| `redis.producer.errors` | Int64Counter | 生产错误数 |
