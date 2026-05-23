# framework — 基础框架层

提供应用生命周期、缓存、数据库访问、日志、指标、重试、分页、校验等基础能力。

## 子包

### [app](./app/) — 应用生命周期管理

注册多个 `IApp` 服务，统一启动和优雅关闭。支持健康检查（`HealthChecker`）、启动/关闭超时配置。

```go
mgr := app.NewManager(app.WithLogger(log))
mgr.Register(&httpApp{}, &cronApp{})
if err := mgr.Run(ctx); err != nil {
    log.Fatal(err)
}
// 或直接退出
mgr.MustRun(ctx)
```

### [cache](./framework/cache/) — 泛型缓存

基于 `gocache` 的类型安全缓存，支持命名空间隔离、容量限制、自动续期、singleflight 防击穿。集成 OTel 指标（hit/miss/evict）。

```go
c := cache.New[string]("users", cacheManager,
    cache.WithMaxItems(1000),
    cache.WithAutoRenew(true),
)
val, err := c.Remember(ctx, "key", time.Minute, func() (string, error) {
    return fetchFromDB()
})
```

### [dbcache](./dbcache/) — 数据库查询结果缓存

将数据库查询结果缓存到 Redis。`IDBCache` 拆分为 `IQueryCache`（查询缓存）、`IKeyValueCache`（键值缓存）、`ICacheManager`（批量失效）三个子接口。

支持自动续期、错误短缓存（防错误风暴）、tag 失效、JSON/Gob/Msgpack 编解码。

```go
dc := dbcache.New[User, UserFilter]("users", cacheManager,
    dbcache.WithAutoRenew(3*time.Minute),
    dbcache.WithErrorCacheTTL(30*time.Second),
)
users, err := dc.Remember(ctx, filter, func() ([]User, error) {
    return dao.List(ctx, filter)
})

// 按 tag 批量失效
dc.Clear(ctx, dbcache.ClearWithTags("user:123"))
```

### [dberror](./dberror/) — 数据库错误识别

检测数据库约束冲突错误，支持 MySQL、PostgreSQL、SQLite。

```go
if dberror.IsDuplicateEntry(err) {
    // 处理重复记录
}
if dberror.IsForeignKeyViolation(err) {
    // 处理外键冲突
}
if dberror.IsNotNullViolation(err) {
    // 处理非空约束
}
```

### [dbmanager](./dbmanager/) — 数据库连接管理器

注册和获取多个数据库连接，`sync.Map` 保证并发安全。

```go
mgr := dbmanager.NewMemoryManager()
mgr.Register("platform", db)
db := mgr.Get("platform")
```

### [dbquery](./dbquery/) — 查询构建器

Filter/Sort/Page 三维正交分解，支持严格排序校验、默认排序、偏移量和游标分页。

```go
query, err := dbquery.Build(db,
    dbquery.WithFilter(filter),
    dbquery.WithSorts("-created_at,name"),
    dbquery.WithOffsetPage(0, 20),
    dbquery.WithStrictSort(true),
    dbquery.WithDefaultSort("id DESC"),
)
```

### [dbrepo](./dbrepo/) — 泛型 DAO

基于泛型的通用 CRUD 仓库。`ISearcher` 拆分为 `IListSearcher`（列表查询）和 `IAggSearcher`（聚合查询）两个子接口。

`IDAO` 提供 Create/First/Update/Delete/Save/Creates/Remove 方法，支持事务（`WithTx`）。`ISearcher` 提供 Paginate/ExistsBy/CountBy 便捷方法。

```go
dao, err := dbrepo.NewDAO[User](db)
user, err := dao.First(ctx, dbrepo.WithWhere("id = ?", 1))
err := dao.Create(ctx, &User{Name: "test"})

// 事务
err := dao.WithTx(tx).Create(ctx, &User{Name: "test"})

// 存在性检查（SELECT 1 LIMIT 1，非 COUNT(*)）
exists, err := searcher.ExistsBy(ctx, &filter)
```

### [dbutil](./dbutil/) — 数据库连接工具

创建 GORM 数据库连接，支持连接池配置、健康检查（`Ping`）、自动重连、多种数据库方言（MySQL/PostgreSQL/SQLite）。

```go
db, err := dbutil.ConnectWithContext(ctx, dbutil.Option{
    Dialect:          "mysql",
    DSN:              "user:pass@tcp(localhost:3306)/db",
    MaxOpenConns:     100,
    MaxIdleConns:     10,
    ConnMaxLifetime:  time.Hour,
})

// 自动重连
db, err := dbutil.ConnectWithReconnect(ctx, opt)
```

### [logger](./logger/) — 日志

基于 `slog` 的结构化日志，支持文件轮转（lumberjack）、控制台输出、OTel 链路追踪（trace_id/span_id 自动注入）、采样限流。

```go
log := logger.NewFileLogger("/var/log/app.log",
    logger.WithLevel(logger.LevelInfo),
    logger.WithFormat(logger.Json),
    logger.WithSampling(types.SamplingConfig{
        Enabled:    true,
        Threshold:  100,
        Interval:   time.Second,
    }),
    logger.WithOTelLoggerProvider(otelProvider),
)
logger.SetDefault(log)
```

### [metrics](./metrics/) — 指标

OTel 委托模式的指标提供者，支持 Counter/Histogram/Gauge，延迟绑定（未配置 OTel 时零开销）。

```go
m := metrics.GetProvider().Meter("my-service")
counter := m.Int64Counter("request.total")
histogram := m.Float64Histogram("request.duration")
gauge := m.Int64Gauge("active.connections")

counter.Add(ctx, 1, metric.WithAttributes(metrics.Attr("method", "GET")))
gauge.Record(ctx, 42, metric.WithAttributes(metrics.Attr("pool", "main")))
```

### [pager](./pager/) — 分页器

分页和排序工具，支持偏移量分页和游标分页。`Sorter` 支持 ASC/DESC/Custom 排序。

```go
sorts := pager.ParseSorts("-created_at,name")
// []Sorter{{Field: "created_at", Order: DESC}, {Field: "name", Order: ASC}}

size := pager.SanitizePageSize(9999) // MaxPageSize=500
```

### [retry](./retry/) — 重试

可配置的重试机制，支持固定延迟、线性退避、指数退避（含 jitter 防惊群）、谓词过滤、上下文取消。

```go
err := retry.Do(ctx, retry.Config{
    MaxAttempts: 3,
    Strategy: &retry.ExponentialDelay{
        Base:   time.Second,
        Max:    30 * time.Second,
        Jitter: true,
    },
    Predicate: func(err error) bool {
        return !errors.Is(err, io.EOF) // EOF 不重试
    },
}, func(attempt uint) error {
    return callExternalAPI()
})
```

### [validator](./validator/) — 结构体校验

封装 `go-playground/validator`，支持自定义校验规则。

```go
v := validator.NewStructValidator(data)
err := v.Validate()
```

### [xcode](./xcode/) — 错误码

分类定义错误码，自动映射 HTTP 状态。预定义通用/权限/数据库/缓存/MQ/存储等类别。

```go
// 定义自定义错误码
ErrOrderNotFound := xcode.DefineXCode(20, 1, "order not found")

// 使用
return xerror.NewXCode(ErrOrderNotFound, "order %d not found", orderID)
```
