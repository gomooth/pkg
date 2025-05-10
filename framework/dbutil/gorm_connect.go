package dbutil

import (
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type dbFunc func() (*gorm.DB, error)

// 利用该结构减少并发锁竞争
var dbRelation sync.Map

// 获取db
func connectWithoutCache(dialect gorm.Dialector, option *Option) (*gorm.DB, error) {
	cnf := option.Config

	opt := &gorm.Config{}
	if cnf != nil {
		if !cnf.LogMode {
			opt.Logger = logger.Default.LogMode(logger.Silent)
		} else if option.Logger != nil {
			if option.LoggerConf != nil {
				opt.Logger = newLoggerWith(option.Logger, option.LoggerConf)
			} else {
				opt.Logger = newLogger(option.Logger)
			}
		}
	}

	// 连接 db
	db, err := gorm.Open(dialect, opt)
	if err != nil {
		return nil, errors.Wrap(err, "db连接异常")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "db连接异常")
	}

	if cnf != nil {
		if cnf.MaxIdle > 0 {
			sqlDB.SetMaxIdleConns(cnf.MaxIdle)
		}
		if cnf.MaxOpen > 0 {
			sqlDB.SetMaxOpenConns(cnf.MaxOpen)
		}
		if cnf.MaxLifeTime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(cnf.MaxLifeTime) * time.Second)
		}
	}

	return db, nil
}

// Connect 获取db
func Connect(option *Option) (*gorm.DB, error) {
	if option == nil {
		return nil, errors.New("db connect option empty")
	}
	if option.Name == "" {
		return nil, errors.New("the db config name invalid")
	}
	if option.Config == nil {
		return nil, errors.New("db config empty")
	}
	if len(option.Config.Driver) == 0 || len(option.Config.Dsn) == 0 ||
		!strings.Contains(option.Config.Dsn, ":") ||
		!strings.Contains(option.Config.Dsn, "@") {
		return nil, errors.New("db config invalid")
	}

	var (
		db  *gorm.DB
		err error

		// 用于只初始化一次
		wg sync.WaitGroup
	)
	wg.Add(1)
	fi, loaded := dbRelation.LoadOrStore(option.Name, dbFunc(func() (*gorm.DB, error) {
		// 阻塞直到初始化完成
		wg.Wait()
		return db, err
	}))

	// 已经存在，则直接调用即可
	if loaded {
		return fi.(dbFunc)()
	}

	// 配置转成方言
	dialect, err := toDialect(option.Config.Driver, option.Config.Dsn)
	if nil != err {
		return nil, err
	}

	// 未找到则需要初始化
	db, err = connectWithoutCache(dialect, option)

	// 真实的返回db函数，wg释放后
	f := dbFunc(func() (*gorm.DB, error) {
		return db, err
	})

	wg.Done()
	// 重置函数
	dbRelation.Store(option.Name, f)
	return db, err
}

// ConnectWith 通过方言获取db
func ConnectWith(dialect gorm.Dialector, option *Option) (*gorm.DB, error) {
	if option.Name == "" {
		return nil, errors.New("the db config name invalid")
	}

	var (
		db  *gorm.DB
		err error

		// 用于只初始化一次
		wg sync.WaitGroup
	)
	wg.Add(1)
	fi, loaded := dbRelation.LoadOrStore(option.Name, dbFunc(func() (*gorm.DB, error) {
		// 阻塞直到初始化完成
		wg.Wait()
		return db, err
	}))

	// 已经存在，则直接调用即可
	if loaded {
		return fi.(dbFunc)()
	}

	// 未找到则需要初始化
	db, err = connectWithoutCache(dialect, option)

	// 真实的返回db函数，wg释放后
	f := dbFunc(func() (*gorm.DB, error) {
		return db, err
	})

	wg.Done()
	// 重置函数
	dbRelation.Store(option.Name, f)
	return db, err
}
