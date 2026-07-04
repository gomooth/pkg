package dbmanager

import "gorm.io/gorm"

// IDatabaseManager 数据库连接管理器
type IDatabaseManager interface {
	// Register 注册数据库连接
	Register(name string, dbc *gorm.DB) error
	// Unregister 注销数据库连接
	Unregister(name string) error
	// Get 获取数据库连接
	Get(name string) (*gorm.DB, error)
	// CloseAll 关闭所有数据库连接
	CloseAll() error
	// List 返回所有已注册的数据库连接名
	List() []string
}
