# CLAUDE.md

项目级指令。Claude Code 在此仓库中工作时自动加载。

## 项目概述

Go 通用工具包，提供企业级应用基础框架（HTTP、数据库、消息队列、缓存、JWT 等），配合 `gomooth/quick-server-template` 使用。

## 常用命令

```bash
go test -race ./...              # 全量测试（含竞态检测）
go test ./<pkg>/... -cover       # 单包覆盖率
go test -race ./<pkg>/...        # 单包竞态检测
go build ./...                   # 构建
go vet ./...                     # 静态分析
go mod tidy                      # 整理依赖
```

## 关键约定

- **Go 版本**: 1.25.0
- **选项模式**: 配置统一用 `func(*Option)` 选项函数，命名 `WithXxx`
- **接口优先**: 公开 API 返回接口（如 `IDAO[T]`、`ICache`），便于 mock 和扩展
- **internal/**: 实现细节放 internal，不对外暴露
- **错误处理**: `github.com/gomooth/xerror` 包装错误，`framework/dberror` 处理数据库错误码
- **测试**: testify/assert + sqlmock + miniredis，public API 包覆盖率 ≥80%

## 模块概览

| 模块 | 职责 | 关键入口 |
|------|------|----------|
| `framework/app` | 应用生命周期 | `MustRun`, `IManager` |
| `framework/cache` | 缓存抽象 | `New`, `ICache` |
| `framework/dbcache` | DB 缓存 | `ICache.Remember` |
| `framework/dbquery` | 查询构建 | `Build`, `ApplySort`, `ApplyPage` |
| `framework/dbrepo` | 通用 DAO | `NewDAO[T]`, `IDAO[T]` |
| `framework/dbutil` | 数据库连接 | `Connect`, `ConnectWith` |
| `framework/pager` | 分页计算 | `ParseSorts` |
| `http/httpcontext` | 请求上下文 | `NewContext`, `IHttpContext` |
| `http/jwt` | JWT 认证 | `NewTokenBuilder`, `IToken` |
| `http/middleware` | Gin 中间件 | `HttpContext`, `XSSFilter`, `RateLimit` |
| `http/restful` | 响应封装 | `NewResponse`, `IResponse` |
| `http/xss` | XSS 防护 | `StrictPolicy`, `UGCPolicy` |
| `job` | 定时任务 | `NewCronJobWrapper` |
| `mq/kafka` | Kafka 生产/消费 | `NewProducer`, `NewConsumer` |
| `mq/redis` | Redis 队列 | `Register` |
| `mq/httpsqs` | HTTPSQS 消费 | `Register` |
| `storage` | 存储抽象 | `Public`, `Disk`, `Temp` |

详细架构见各包的 godoc。

## 开发规范

1. 提交 git 时，必须严格遵守 .gitignore 文件约定
