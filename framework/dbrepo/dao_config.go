package dbrepo

import (
	"context"

	"gorm.io/gorm"
)

// DBGetter 数据库连接获取函数类型
type DBGetter func(name string) (*gorm.DB, error)

// daoConfig DAO配置结构体
type daoConfig struct {
	ctx      context.Context
	db       *gorm.DB
	dbName   string
	dbGetter DBGetter
}

// Option DAO配置选项函数类型
type Option func(*daoConfig)

// WithContext 使用指定上下文
func WithContext(ctx context.Context) Option {
	return func(config *daoConfig) {
		config.ctx = ctx
	}
}

// WithDB 使用指定的数据库连接
func WithDB(db *gorm.DB) Option {
	return func(c *daoConfig) {
		c.db = db
		c.dbName = "" // 清除dbName，因为直接提供了db
	}
}

// WithDBName 使用指定名称的数据库连接
func WithDBName(dbName string, dbGetter DBGetter) Option {
	return func(c *daoConfig) {
		c.dbName = dbName
		c.dbGetter = dbGetter
		c.db = nil // 清除db，因为使用dbName
	}
}

// getDB 根据配置获取数据库连接
func getDB(config *daoConfig) *gorm.DB {
	db := config.db
	// 如果提供了dbName，使用默认的DBGetter获取连接
	if db == nil && config.dbName != "" && config.dbGetter != nil {
		ndb, err := config.dbGetter(config.dbName)
		if err != nil {
			panic(err)
		}
		db = ndb
	}

	if db == nil {
		panic("db not set")
	}

	if config.ctx != nil {
		db = db.WithContext(config.ctx)
	}
	return db
}
