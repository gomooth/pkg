package dbutil

import "gorm.io/gorm"

type connectOption struct {
	gormDialect gorm.Dialector
}

// ConnectWithGORMDialector 设置 GORM 数据库方言，用于自定义数据库连接
func ConnectWithGORMDialector(dialect gorm.Dialector) func(*connectOption) {
	return func(opt *connectOption) {
		opt.gormDialect = dialect
	}
}
