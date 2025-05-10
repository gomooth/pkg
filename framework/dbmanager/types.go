package dbmanager

import "gorm.io/gorm"

// IDatabaseManager 数据库连接管理器
type IDatabaseManager interface {
	// Register 注册数据库连接
	Register(name string, dbc *gorm.DB) error
	// Get 获取数据库连接
	Get(name string) (*gorm.DB, error)
}
