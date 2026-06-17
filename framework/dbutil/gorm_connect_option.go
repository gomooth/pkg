package dbutil

import "gorm.io/gorm"

// connectOption dbutil 连接选项的中间结构体
type connectOption struct {
	gormDialect gorm.Dialector
}

// ConnectOption dbutil 连接选项函数
type ConnectOption func(*connectOption)

// ConnectWithGORMDialector 设置 GORM 数据库方言，用于自定义数据库连接
func ConnectWithGORMDialector(dialect gorm.Dialector) ConnectOption {
	return func(opt *connectOption) {
		opt.gormDialect = dialect
	}
}
