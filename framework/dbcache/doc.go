// Package dbcache 提供数据库查询结果的缓存层，基于 Tag 失效机制实现精确的缓存管理。
//
// # 事务语义
//
// IDBCache 的查询方法（Paginate/List/Remember 等）在缓存未命中时通过 queryFn 回调
// 查询数据库。queryFn 使用 ISearcher 的原始 DB 连接，不参与事务。
// 如需事务内的一致性读，请直接使用 ISearcher.WithTx(tx) 而非通过 IDBCache。
package dbcache