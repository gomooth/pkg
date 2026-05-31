# mq/httpsqs — HTTPSQS 队列消费者

基于 HTTPSQS 协议实现的队列消费者，通过 HTTP API 与 HTTPSQS 服务交互。实现 `app.IApp` 接口，可通过 `app.Manager` 统一管理生命周期。

> **注意**：本模块仅提供消费者，不提供生产者。生产消息请直接使用 `gomooth/httpsqs` 客户端。

## 特性

- **Pull 模式消费**：客户端主动拉取消息
- **双模式重试**：同步阻塞重试 / 再入队重试
- **Per-Queue 配置**：支持每个队列独立覆盖全局配置（客户端、重试次数、退避策略等）
- **死信处理**：重试耗尽后支持自定义死信处理器
- **优雅关闭**：失败处理器支持优雅关闭，不丢消息
- **指标集成**：内置 OpenTelemetry 指标（消费/重试/死信计数）

---

## 快速开始

```go
client := httpsqs.NewClient("http://localhost:1218")

consumer := httpsqs.NewConsumer(
    httpsqs.WithHTTPSQSClient(client),
    httpsqs.WithConsumer("orders", orderHandler),
    httpsqs.WithMaxRetry(3),
)

mgr := app.NewManager()
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

---

## 消费者

### 创建

```go
consumer := httpsqs.NewConsumer(opts ...ConsumerOption)
```

### 全局配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithHTTPSQSClient(client)` | HTTPSQS 客户端（必填） | — |
| `WithMaxRetry(n)` | 最大重试次数，0=不重试 | 3 |
| `WithBackoff(b)` | 退避策略 | `ExponentialDelay{Base:1s, Max:5min}` |
| `WithRetryMode(mode)` | 重试模式 | `RetryModeSync` |
| `WithHandlerTimeout(d)` | 单次 handler 超时 | 0（不限） |
| `WithEmptyQueueSleep(d)` | 队列空时休眠间隔 | 1s |
| `WithFailedHandler(fn)` | 重试耗尽后的回调 | 日志记录 |
| `WithPanicHandler(fn)` | panic 恢复后的回调 | 无 |
| `WithConsumerLogger(l)` | 日志器 | `slog.Default()` |
| `WithConsumer(queue, handler, ...)` | 预注册消费者 | — |
| `WithConsumers(regs...)` | 批量预注册消费者 | — |

### 注册处理器

```go
// 方式一：预注册（推荐）
consumer := httpsqs.NewConsumer(
    httpsqs.WithHTTPSQSClient(client),
    httpsqs.WithConsumer("orders", orderHandler),
)

// 方式二：启动前注册
consumer := httpsqs.NewConsumer(httpsqs.WithHTTPSQSClient(client))
consumer.Register("orders", orderHandler)
```

### IHandler 接口

```go
type IHandler interface {
    Handle(ctx context.Context, queue string, data string, pos int64) error
}
```

注意参数与 Redis/Kafka 的区别：
- `data string`：消息内容为字符串（非 `[]byte`），因为 HTTPSQS 返回文本
- `pos int64`：消息位置，可用于确认或追踪

**函数适配器**：

```go
handler := httpsqs.FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
    fmt.Println(queue, data, pos)
    return nil
})
```

### 死信处理器（可选）

实现 `DeadLetterHandler` 接口，重试耗尽后自动调用：

```go
type OrderHandler struct{}

func (h *OrderHandler) Handle(ctx context.Context, queue string, data string, pos int64) error {
    return errors.New("处理失败")
}

func (h *OrderHandler) OnDeadLetter(ctx context.Context, queue string, data string, pos int64, lastErr error) error {
    // 写入死信表、发送告警等
    return nil
}
```

---

## Per-Queue 配置

HTTPSQS 支持每个队列独立覆盖全局配置，这是与 Redis/Kafka 的关键差异：

```go
consumer := httpsqs.NewConsumer(
    httpsqs.WithHTTPSQSClient(client),

    // orders 队列：默认配置
    httpsqs.WithConsumer("orders", orderHandler),

    // notifications 队列：使用独立客户端 + 更多重试
    httpsqs.WithConsumer("notifications", notifyHandler,
        httpsqs.WithQueueHTTPSQSClient(notifyClient),
        httpsqs.WithQueueMaxRetry(10),
    ),

    // logs 队列：再入队模式 + 自定义退避
    httpsqs.WithConsumer("logs", logHandler,
        httpsqs.WithQueueRetryMode(httpsqs.RetryModeRequeue),
        httpsqs.WithQueueBackoff(&retry.LinearDelay{Base: 2 * time.Second}),
    ),
)
```

### 队列级别选项

| 选项 | 说明 |
|------|------|
| `WithQueueHTTPSQSClient(client)` | 独立 HTTPSQS 客户端 |
| `WithQueueMaxRetry(n)` | 最大重试次数 |
| `WithQueueBackoff(b)` | 退避策略 |
| `WithQueueRetryMode(mode)` | 重试模式 |
| `WithQueueFailedHandler(fn)` | 失败处理回调 |

---

## 重试模式

### RetryModeSync（同步阻塞重试）

Handle 失败后在当前 goroutine 中立即重试，阻塞该队列的消费。

```go
consumer := httpsqs.NewConsumer(
    httpsqs.WithHTTPSQSClient(client),
    httpsqs.WithRetryMode(httpsqs.RetryModeSync),
    httpsqs.WithMaxRetry(3),
)
```

**适用场景**：消息重要性高，需要快速重试

### RetryModeRequeue（再入队重试）

Handle 失败后通过 HTTPSQS Put 将消息放回队列尾部，不阻塞当前消费者。使用 `AttemptTracker`（基于 SHA256 前 16 字符）跟踪重试次数。

```go
consumer := httpsqs.NewConsumer(
    httpsqs.WithHTTPSQSClient(client),
    httpsqs.WithRetryMode(httpsqs.RetryModeRequeue),
    httpsqs.WithMaxRetry(3),
)
```

**适用场景**：多消费者分布式处理、避免单点阻塞

### 对比

| 维度 | Sync | Requeue |
|------|------|---------|
| 阻塞 | 阻塞当前队列消费 | 不阻塞 |
| 顺序 | 严格顺序 | 可能乱序 |
| 分布式 | 单消费者 | 多消费者 |
| 复杂度 | 低 | 中（需要 AttemptTracker） |

---

## 生命周期管理

Consumer 实现 `app.IApp` + `app.HealthChecker`，推荐通过 `app.Manager` 统一管理：

```go
client := httpsqs.NewClient("http://localhost:1218")
consumer := httpsqs.NewConsumer(
    httpsqs.WithHTTPSQSClient(client),
    httpsqs.WithConsumer("orders", orderHandler),
)

mgr := app.NewManager()
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

Manager 负责：
- 顺序启动所有服务
- 捕获 SIGINT/SIGTERM 信号
- 逆序关闭所有服务（含超时保护，失败处理器优雅关闭）
- 统一健康检查

---

## 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `httpsqs.consumer.messages` | Int64Counter | 成功消费消息数 |
| `httpsqs.consumer.retries` | Int64Counter | 重试次数 |
| `httpsqs.consumer.dead_letters` | Int64Counter | 死信消息数 |
