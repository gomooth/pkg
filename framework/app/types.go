package app

// IApp 应用约定
type IApp interface {
	// Start 启动
	Start() error
	// Shutdown 关闭
	Shutdown() error
}

type IManager interface {
	// Register 注册应用
	Register(app IApp)
	// Run 启动应用
	Run()
}
