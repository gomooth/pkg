# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

这是一个 Go 语言通用工具包项目，提供企业级应用开发的基础框架和工具库。主要配合 `gomooth/quick-server-template` 项目使用，包含 HTTP 服务、数据库访问、消息队列、日志、缓存等核心模块。

## 开发环境

- **Go 版本**: 1.23.4
- **模块管理**: Go Modules
- **主要依赖**: 
  - Web框架: Gin
  - ORM: GORM
  - 消息队列: Sarama (Kafka), Redis队列
  - 日志: Logrus + xlog 封装
  - 缓存: gocache + Redis
  - JWT: golang-jwt/jwt/v5

## 常用命令

### 构建和测试
```bash
# 运行所有测试（注意：部分测试文件需要修复）
go test ./...

# 运行特定包的测试
go test ./framework/logger/...

# 构建整个项目
go build ./...

# 检查依赖
go mod tidy
```

### 代码质量
```bash
# 格式化代码
go fmt ./...

# 静态分析
go vet ./...
```

## 项目架构

### 核心模块结构

```
pkg/
├── framework/          # 基础框架层
│   ├── app/           # 应用生命周期管理
│   ├── cache/         # 缓存抽象层
│   ├── dbcache/       # 数据库缓存
│   ├── dberror/       # 数据库错误处理
│   ├── dbmanager/     # 数据库连接管理
│   ├── dbquery/       # 数据库查询构建器
│   ├── dbrepo/        # 数据库仓库模式实现
│   │   ├── dao.go     # 通用DAO（数据访问对象）
│   │   ├── query_builder.go # 查询构建器
│   │   └── factory.go # 工厂模式
│   ├── dbutil/        # 数据库工具
│   ├── logger/        # 日志框架
│   │   ├── internal/  # 内部实现
│   │   │   ├── console/ # 控制台日志
│   │   │   ├── logrus/  # Logrus实现
│   │   │   └── types/   # 类型定义
│   │   └── option.go  # 配置选项
│   ├── pager/         # 分页器
│   └── validator/     # 验证器
├── http/              # HTTP相关模块
│   ├── httpcontext/   # HTTP上下文管理
│   ├── httpmodel/     # HTTP数据模型
│   ├── jwt/           # JWT认证
│   │   ├── jwtstore/  # JWT存储（Redis）
│   │   └── token.go   # Token管理
│   ├── middleware/    # Gin中间件
│   │   ├── internal/  # 中间件内部实现
│   │   │   ├── cors/  # CORS
│   │   │   ├── httpcache/ # HTTP缓存
│   │   │   ├── jwt/   # JWT处理
│   │   │   ├── limit/ # 限流
│   │   │   ├── logger/ # 日志
│   │   │   ├── restful/ # RESTful
│   │   │   └── xss/   # XSS过滤
│   │   └── jwt.go     # JWT中间件入口
│   ├── restful/       # RESTful响应
│   └── xss/           # XSS防护
├── job/               # 定时任务
│   ├── cron_wrapper.go # Cron包装器
│   └── job.go         # 任务定义
├── mq/                # 消息队列
│   ├── httpsqsconsumer/ # HTTPSQS消费者
│   ├── kafkaconsumer/  # Kafka消费者
│   ├── kafkaproducer/  # Kafka生产者
│   ├── queue/          # 通用队列
│   └── redisconsumer/  # Redis消费者
└── storage/           # 存储抽象
    ├── public.go      # 公共存储
    └── func.go        # 存储函数
```

### 关键设计模式

1. **选项模式（Option Pattern）**: 广泛用于配置初始化，如 `framework/logger/option.go`
2. **仓库模式（Repository Pattern）**: `framework/dbrepo/` 提供通用DAO和查询构建器
3. **中间件模式**: `http/middleware/` 提供可插拔的Gin中间件
4. **工厂模式**: `framework/dbrepo/factory.go` 提供对象创建工厂

### 数据库访问

- **DAO层**: `framework/dbrepo/dao.go` 提供通用CRUD操作
- **查询构建器**: `framework/dbrepo/query_builder.go` 支持链式查询
- **分页**: `framework/pager/` 与 `framework/dbquery/` 集成
- **事务**: DAO支持事务操作

### HTTP服务

- **上下文管理**: `http/httpcontext/` 提供请求上下文，包含用户信息、追踪ID等
- **JWT认证**: 支持无状态和有状态（Redis存储）两种模式
- **中间件**: 包含CORS、限流、日志、XSS过滤等
- **RESTful响应**: `http/restful/` 提供标准化的API响应

### 错误处理

- 使用 `github.com/save95/xerror` 进行错误包装
- 数据库错误有专门的错误码定义 `framework/dberror/error_code.go`

## 开发规范

### 代码组织

1. **包结构**: 按功能模块划分，每个包有清晰的职责
2. **接口定义**: 关键功能提供接口，便于测试和扩展
3. **内部包**: 实现细节放在 `internal/` 目录下

### 测试

- 测试文件使用 `_test.go` 后缀
- 测试数据放在 `testdata/` 目录（如果存在）
- 注意：部分测试文件需要修复编译错误

### 配置管理

- 使用选项模式进行配置
- 配置结构体通常命名为 `Option`
- 配置函数使用 `func(*Option)` 类型

## 注意事项

1. **JWT中间件**: `http/middleware/internal/jwt/stateful_handler.go` 有函数比较警告
2. **数据库连接**: `framework/dbrepo/dao.go` 默认使用 "platform" 数据库名称，可能需要根据实际环境调整
3. **存储路径**: `storage/public.go` 使用相对路径，部署时需要注意

## 扩展开发

当添加新功能时：
1. 遵循现有的选项模式进行配置
2. 为关键功能定义接口
3. 将实现细节放在 `internal/` 目录
4. 添加相应的测试用例
5. 更新 `go.mod` 依赖