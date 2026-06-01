// Package cache 提供缓存抽象层，封装 gocache 实现泛型缓存读写。
//
// 支持自动续期、容量限制、singleflight 防击穿等能力，
// 并集成 OpenTelemetry 指标采集。
package cache
