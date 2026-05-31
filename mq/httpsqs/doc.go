// Package httpsqs 提供统一的 HTTPSQS 队列消费者实现。
//
// 消费者支持同步重试和再入队重试两种模式，通过可配置的退避策略控制重试节奏。
// 支持 per-queue 配置覆盖全局默认值（QueueOption）。
//
// Consumer 实现 app.IApp 接口，可通过 app.Manager 统一管理生命周期。
package httpsqs
