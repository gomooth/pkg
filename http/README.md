# http — HTTP 服务

提供 JWT 认证、Gin 中间件、RESTful 响应、XSS 防护等 HTTP 服务基础能力。

## 子包

### [httpcontext](./httpcontext/) — 请求上下文

管理 HTTP 请求上下文，包含用户信息、角色权限、追踪 ID 等。使用自定义 `ctxKey` 类型避免上下文键冲突。

```go
ctx, err := httpcontext.MustParse(c.Request.Context())
ctx.SetUser(httpcontext.User{ID: 1, Account: "admin"})
```

### [httpmodel](./httpmodel/) — HTTP 数据模型

通用 HTTP 请求/响应模型：

| 类型 | 说明 |
|------|------|
| `ResponseModel` | 标准响应字段（ID、创建/更新时间） |
| `SearchRequest` | 偏移量分页搜索（Start/Limit/Sort） |
| `CursorSearchRequest` | 游标分页搜索（After/Limit） |
| `DayStatRequest` | 按日统计（起止日期 + 日期范围计算） |

### [jwt](./jwt/) — JWT 认证

支持无状态和有状态（Redis 存储）双模式。密钥通过 `NewOption` 显式传入（最低 16 字节），支持 Leeway（分布式时钟偏移容忍）、Legacy Secrets（密钥轮换）、HashFunc（防时序攻击）。

```go
// 创建配置（显式传入密钥）
opt := jwt.NewOption([]byte("your-secret-key-32-bytes-long!!"), roleConvert,
    jwt.WithLeeway(30*time.Second),
    jwt.WithLegacySecrets([]byte("old-secret")),
)

// 生成 Token
token, err := jwt.NewToken(opt.Secret, user)

// 有状态存储（Redis）
store := jwtstore.NewSingleRedisStore(redisClient,
    jwtstore.WithSingleHashFunc(jwt.DefaultHashFunc),
)
token, err := jwt.NewStatefulToken(opt.Secret, user, store)
```

### [middleware](./middleware/) — Gin 中间件

| 中间件 | 函数 | 说明 |
|--------|------|------|
| JWT | `JWTWith()` | 无状态 Token 校验 |
| JWT 有状态 | `JWTStatefulWith()` | Redis 存储 Token，支持吊销 |
| CORS | `CORS()` | 跨域资源共享 |
| 限流 | `RateLimiter()` / `IPRateLimit()` | 请求速率限制 |
| HTTP 日志 | `HttpLogger()` | 请求/响应日志（含敏感字段脱敏） |
| HTTP 打印 | `HttpPrinter()` | 调试用完整打印（不脱敏） |
| HTTP 缓存 | `HttpCache()` | Redis 响应缓存（按路由策略） |
| XSS | `XSSFilter()` | 请求体 XSS 过滤 |
| RESTful | `RESTFul()` | RESTful 响应格式化 |
| Session | `Session()` / `SessionWithSecretFromEnv()` | Cookie Session |

```go
r.Use(middleware.CORS())
r.Use(middleware.HttpLogger(middleware.HttpLoggerOption{
    Logger:    log,
    OnlyError: false,
}))
r.Use(middleware.IPRateLimit(limiter))

// JWT 中间件
authorized := r.Group("/")
authorized.Use(middleware.JWTWith(opt))

// HTTP 缓存（按路由配置）
r.Use(middleware.HttpCache(
    middleware.WithHttpCacheRedisStore(redisClient),
    middleware.WithHttpCacheGlobalDuration(5*time.Minute),
    middleware.WithHttpCacheRoutePolicy("/api/users", true),
))
```

**HttpLogger 敏感字段脱敏**：默认开启，自动脱敏 `password`、`token`、`secret` 等字段和 `Authorization`、`Cookie` 等 Header。可通过 `RedactEnabled` 和 `SensitiveFields` 配置。

### [restful](./restful/) — RESTful 响应

标准化 API 响应格式，`IResponse` 接口提供 Retrieve/Post/Put/Delete/Paginate 方法。支持游标分页、多语言错误消息、XCode 错误码透传。

```go
resp := restful.NewResponse(c,
    restful.WithResponseShowXCode(xcode.Unauthorized),
    restful.WithResponseDebugError(true),
)
resp.ListWithPagination(total, users)
```

### [xss](./xss/) — XSS 防护

基于 `bluemonday` 的输入过滤策略，支持三种级别：

| 策略 | 说明 |
|------|------|
| `PolicyNone` | 不过滤 |
| `PolicyStrict` | 严格过滤（管理端输入） |
| `PolicyUGC` | 用户生成内容过滤（保留安全标签） |

提供工厂函数复用 Policy 实例，避免重复创建：

```go
strictPolicy := xss.DefaultStrictPolicy()
ugcPolicy := xss.DefaultUGCPolicy()
filtered := ugcPolicy.Sanitize(input)
```
