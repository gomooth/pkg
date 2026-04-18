package dbutil

import "gorm.io/gorm"

type connectOption struct {
	gormDialect gorm.Dialector
}

func ConnectWithGORMDialector(dialect gorm.Dialector) func(*connectOption) {
	return func(opt *connectOption) {
		opt.gormDialect = dialect
	}
}
