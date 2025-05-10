package app

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gomooth/pkg/framework/logger"

	"github.com/save95/xlog"
)

type manager struct {
	apps []IApp
	log  xlog.XLog
}

// NewManager 创建 APP 管理器
func NewManager(opts ...func(*manager)) IManager {
	m := &manager{
		apps: make([]IApp, 0),
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

// Run 启动应用
func (m *manager) Run() {
	m.log.Info("Server startup...")

	// 启动 app
	for i := range m.apps {
		if err := m.apps[i].Start(); err != nil {
			m.log.Errorf("Server app start failed: err=%+v", err)
			os.Exit(1)
		}
	}

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	m.log.Info("Server shutting down...")

	// 关闭
	for i := range m.apps {
		if err := m.apps[i].Shutdown(); err != nil {
			m.log.Errorf("Server app shutdown failed: %+v", err)
			os.Exit(1)
		}
	}

	m.log.Info("Server exiting")
}
