package dbutil

import (
	"github.com/save95/xlog"
	"gorm.io/gorm/logger"
)

// Option 连接操作配置
type Option struct {
	Name       string         // 连接别名
	Config     *ConnectConfig // 连接配置
	Logger     xlog.XLog      // 日志
	LoggerConf *logger.Config // 日志配置
}

// ConnectConfig 数据库链接配置
type ConnectConfig struct {
	Dsn         string // 连接
	Driver      string // 数据库类型
	MaxIdle     int    // 最大空闲连接数
	MaxOpen     int    // 最大连接数
	LogMode     bool   // 是否打印SQL
	MaxLifeTime int    // 连接存活时间
}
