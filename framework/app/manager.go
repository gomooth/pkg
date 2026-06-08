package app

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
)

type manager struct {
	apps            []IApp
	log             *slog.Logger
	shutdownTimeout time.Duration
	startupTimeout  time.Duration
}

var _ IManager = (*manager)(nil)

// NewManager 创建 APP 管理器
func NewManager(opts ...func(*manager)) IManager {
	m := &manager{
		apps:            make([]IApp, 0),
		shutdownTimeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.log == nil {
		m.log = logger.NewConsoleLogger()
	}

	return m
}

// Register 注册应用
func (m *manager) Register(app IApp) {
	m.apps = append(m.apps, app)
}

// Run 启动应用，返回启动或关闭错误
func (m *manager) Run(ctx context.Context) error {
	m.log.Info("Server startup...")

	// 启动 app
	for i := range m.apps {
		startCtx := ctx
		var startCancel context.CancelFunc
		if m.startupTimeout > 0 {
			startCtx, startCancel = context.WithTimeout(ctx, m.startupTimeout)
		}
		err := m.apps[i].Start(startCtx)
		if startCancel != nil {
			startCancel()
		}
		if err != nil {
			m.log.Error("Server app start failed", slog.String("component", "app"), slog.Int("index", i), slog.String("error", err.Error()))
			// 逆序关闭已启动的 app（使用 shutdownTimeout 防止阻塞）
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
			for j := i - 1; j >= 0; j-- {
				if err := m.apps[j].Shutdown(cleanupCtx); err != nil {
					m.log.Error("Server app shutdown failed", slog.String("component", "app"), slog.Int("index", j), slog.String("error", err.Error()))
				}
			}
			cleanupCancel()
			return xerror.WrapWithXCode(err, xcode.InternalServerError)
		}
	}

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 30 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 仅监听 SIGINT/SIGTERM：覆盖 kill 和 Ctrl+C 的常见场景。
	// SIGHUP 等自定义信号需在外部 signal.Notify 处理。
	<-quit
	m.log.Info("Server shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	var shutdownErrs []error
	for i := len(m.apps) - 1; i >= 0; i-- {
		if err := m.apps[i].Shutdown(shutdownCtx); err != nil {
			m.log.Error("Server app shutdown failed", slog.String("component", "app"), slog.Int("index", i), slog.String("error", err.Error()))
			shutdownErrs = append(shutdownErrs, err)
		}
	}

	if len(shutdownErrs) > 0 {
		m.log.Error("Server shutdown completed with errors", slog.String("component", "app"))
	}

	m.log.Info("Server exiting")
	return errors.Join(shutdownErrs...)
}

// MustRun 启动应用，失败时直接 os.Exit(1)
func (m *manager) MustRun(ctx context.Context) {
	if err := m.Run(ctx); err != nil {
		m.log.Error("Server start failed", slog.String("component", "app"), slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// IsHealthy 检查所有已注册应用的健康状态
func (m *manager) IsHealthy(ctx context.Context) error {
	var errs []error
	for _, app := range m.apps {
		if hc, ok := app.(HealthChecker); ok {
			if err := hc.HealthCheck(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
