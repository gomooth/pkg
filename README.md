# pkg

Go 语言通用工具包，提供企业级应用开发的基础框架和工具库。主要配合 [gomooth/quick-server-template](https://github.com/gomooth/quick-server-template) 使用。

## 模块概览

| 模块 | 说明 |
|------|------|
| [framework](./framework/) | 基础框架层：应用生命周期、缓存、数据库访问、日志、指标、重试、分页、校验 |
| [http](./http/) | HTTP 服务：JWT 认证、Gin 中间件、RESTful 响应、XSS 防护 |
| [mq](./mq/) | 消息队列：Kafka 消费/生产、Redis 队列消费、通用队列接口 |
| [job](./job/) | 定时任务：Cron 包装器、命令式任务（含自动重试） |
| [storage](./storage/) | 文件存储：公开/私有/临时文件管理，路径安全防护 |

## 安装

```bash
go get github.com/gomooth/pkg
```

要求 Go 1.25+。

## 快速开始

### 数据库 CRUD（泛型 DAO）

```go
type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
}

dao, err := dbrepo.NewDAO[User](db)
user, err := dao.First(ctx, dbrepo.WithWhere("name = ?", "test"))
```

### 带重试的操作

```go
err := retry.Do(ctx, retry.Config{
    MaxAttempts: 3,
    Strategy: &retry.ExponentialDelay{
        Base:   time.Second,
        Max:    30 * time.Second,
        Jitter: true,
    },
}, func(attempt uint) error {
    return callExternalAPI()
})
```

### HTTP 日志中间件（含敏感字段脱敏）

```go
r.Use(middleware.HttpLogger(middleware.HttpLoggerOption{
    Logger:    logger,
    OnlyError: false,
}))
```

### JWT 认证

```go
opt := jwt.NewOption([]byte("your-secret-key-32-bytes-long!!"), roleConvert)

// 无状态模式
r.Use(middleware.JWTWith(opt))

// 有状态模式（Redis 存储，支持 Token 吊销）
r.Use(middleware.JWTStatefulWith(opt, jwtStore))
```

### Kafka 消费

```go
consumer := kafka.NewConsumer([]string{"localhost:9092"},
kafka.WithConsumer("order-group", orderHandler, "orders"),
kafka.WithMaxRetry(3),
)

mgr := app.NewManager()
mgr.Register(consumer)
mgr.MustRun(context.Background())
```

### 文件存储

```go
// 公开文件
p := storage.Public().AppendDir("avatars", "2024").SetName("photo.jpg")
path, _ := p.Path()    // storage/public/avatars/2024/photo.jpg
url, _ := p.URL()      // /storage/avatars/2024/photo.jpg

// 私有文件
d := storage.Disk("exports").AppendDir("reports").SetName("data.csv")
path, _ := d.Path()    // storage/exports/reports/data.csv
```

## 包文档

各子包提供独立的 README 文档：

### framework — [README](./framework/README.md)

| 子包 | 说明 |
|------|------|
| [app](./framework/app/) | 应用生命周期管理（graceful shutdown + 健康检查） |
| [cache](./framework/cache/) | 泛型缓存（gocache + singleflight + OTel 指标） |
| [dbcache](./framework/dbcache/) | 数据库查询结果缓存（自动续期、错误短缓存、tag 失效） |
| [dberror](./framework/dberror/) | 数据库错误识别（唯一键/外键/非空约束冲突） |
| [dbmanager](./framework/dbmanager/) | 数据库连接管理器 |
| [dbquery](./framework/dbquery/) | 查询构建器（Filter/Sort/Page 三维正交分解） |
| [dbrepo](./framework/dbrepo/) | 泛型 DAO（CRUD + 事务 + 游标分页） |
| [dbutil](./framework/dbutil/) | 数据库连接工具（健康检查 + 自动重连） |
| [logger](./framework/logger/) | 日志（slog + OTel 链路追踪 + 采样限流） |
| [metrics](./framework/metrics/) | 指标（OTel Counter/Histogram/Gauge 委托模式） |
| [pager](./framework/pager/) | 分页器（偏移量 + 游标分页） |
| [retry](./framework/retry/) | 重试（固定/线性/指数退避 + jitter + 谓词过滤） |
| [validator](./framework/validator/) | 结构体校验 |
| [xcode](./framework/xcode/) | 错误码（分类定义 + HTTP 状态映射） |

### http — [README](./http/README.md)

| 子包 | 说明 |
|------|------|
| [httpcontext](./http/httpcontext/) | 请求上下文（用户信息、角色、追踪） |
| [httpmodel](./http/httpmodel/) | HTTP 数据模型（搜索/游标/统计请求） |
| [jwt](./http/jwt/) | JWT 认证（无状态/有状态双模式 + Leeway + HashFunc） |
| [middleware](./http/middleware/) | Gin 中间件（CORS/JWT/限流/日志/缓存/XSS/Session） |
| [restful](./http/restful/) | RESTful 标准响应 |
| [xss](./http/xss/) | XSS 防护策略 |

### mq — [README](./mq/README.md)

| 子包 | 说明                                   |
|------|--------------------------------------|
| [kafka](./mq/kafka/) | Kafka 生产者（批量/顺序发送），消费者（同步/异步重试 + 死信） |
| [redis](./mq/redis/) | Redis 队列生产者，消费者                    |
| [httpsqs](./mq/httpsqs/) | HTTPSQS 消费者                        |

### job — [README](./job/README.md)

Cron 包装器、命令式任务（含自动重试 + 超时控制）

### storage — [README](./storage/README.md)

公开/私有/临时文件管理，路径安全防护

## 架构

```
pkg/
├── framework/           # 基础框架层
│   ├── app/            # 应用生命周期管理（graceful shutdown + 健康检查）
│   ├── cache/          # 泛型缓存（gocache + singleflight + OTel 指标）
│   ├── dbcache/        # 数据库查询结果缓存（自动续期、错误短缓存、tag 失效）
│   ├── dberror/        # 数据库错误识别（唯一键/外键/非空约束冲突）
│   ├── dbmanager/      # 数据库连接管理器
│   ├── dbquery/        # 查询构建器（Filter/Sort/Page 三维正交分解）
│   ├── dbrepo/         # 泛型 DAO（CRUD + 事务 + 游标分页）
│   ├── dbutil/         # 数据库连接工具（健康检查 + 自动重连）
│   ├── logger/         # 日志（slog + OTel 链路追踪 + 采样限流）
│   ├── metrics/        # 指标（OTel Counter/Histogram/Gauge 委托模式）
│   ├── pager/          # 分页器（偏移量 + 游标分页）
│   ├── retry/          # 重试（固定/线性/指数退避 + jitter + 谓词过滤）
│   ├── validator/      # 结构体校验
│   └── xcode/          # 错误码（分类定义 + HTTP 状态映射）
├── http/               # HTTP 相关
│   ├── httpcontext/    # 请求上下文（用户信息、角色、追踪）
│   ├── httpmodel/      # HTTP 数据模型（搜索/游标/统计请求）
│   ├── jwt/            # JWT 认证（无状态/有状态双模式）
│   ├── middleware/     # Gin 中间件（CORS/JWT/限流/日志/缓存/XSS/Session）
│   ├── restful/        # RESTful 标准响应
│   └── xss/            # XSS 防护策略
├── mq/                 # 消息队列
│   ├── kafka/          # Kafka 生产者（批量/顺序），消费者（重试 + 死信）
│   ├── redis/          # Redis 队列消费者
│   └── httpsqs/        # HTTPSQS 消费者
├── job/                # 定时任务
│   ├── cron_wrapper.go # Cron 包装器
│   └── job.go          # 命令式任务（自动重试 + 超时）
└── storage/            # 文件存储
    ├── public.go       # 公开文件（URL 生成）
    ├── storage.go      # 私有/临时文件
    └── type_vars.go    # 接口定义
```

## 设计模式

- **选项模式（Option Pattern）**：配置初始化，如 `WithMaxRetry(3)`
- **接口模式（Interface Pattern）**：关键功能提供接口，便于测试和扩展
- **仓库模式（Repository Pattern）**：`dbrepo` 提供泛型 DAO
- **中间件模式**：`middleware` 提供可插拔的 Gin 中间件
- **委托模式（Delegate Pattern）**：`metrics` 委托 OTel 全局 Provider，支持延迟绑定

## License

MIT
