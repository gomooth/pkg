package app

import "context"

// IApp 应用约定
type IApp interface {
	// Start 启动
	Start(ctx context.Context) error
	// Shutdown 关闭
	Shutdown(ctx context.Context) error
}

// IManager 应用生命周期管理器，负责注册、启动、关闭和健康检查
type IManager interface {
	// Register 注册应用
	Register(app IApp)
	// Run 启动应用，返回启动或关闭错误
	Run(ctx context.Context) error
	// MustRun 启动应用，失败时直接 os.Exit(1)
	MustRun(ctx context.Context)
	// IsHealthy 检查所有已注册应用的健康状态。
	// 返回 nil 表示全部健康；返回第一个非 nil 错误。
	// 未实现 HealthChecker 的 IApp 视为健康。
	IsHealthy(ctx context.Context) error
}

// HealthChecker 可选接口，实现此接口的 IApp 将被 Manager 自动纳入健康检查。
// 未实现此接口的 IApp 默认视为健康。
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}
