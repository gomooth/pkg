package dbutil

import "gorm.io/gorm"

// connectOption dbutil 连接选项的中间结构体
type connectOption struct {
	gormDialect gorm.Dialector
	plugins     []gorm.Plugin
}

// ConnectOption dbutil 连接选项函数
type ConnectOption func(*connectOption)

// ConnectWithGORMDialector 设置 GORM 数据库方言，用于自定义数据库连接
func ConnectWithGORMDialector(dialect gorm.Dialector) ConnectOption {
	return func(opt *connectOption) {
		opt.gormDialect = dialect
	}
}

// WithGORMPlugin 注入自定义 GORM 插件，连接建立时注册进 gorm.Config.Plugins，
// 其 Initialize 由 gorm.Open 自动调用（时序上晚于 Dialector.Initialize）。
// p 为 nil 时为 no-op，便于调用侧按功能开关直接传值。
func WithGORMPlugin(p gorm.Plugin) ConnectOption {
	return func(opt *connectOption) {
		if p == nil {
			return
		}
		opt.plugins = append(opt.plugins, p)
	}
}
