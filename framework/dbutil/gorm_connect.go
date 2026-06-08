package dbutil

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 利用该结构减少并发锁竞争
var dbRelation sync.Map

type dbHolder struct {
	mu           sync.Mutex
	db           *gorm.DB
	err          error
	ready        bool
	reconnecting bool         // 标记正在重连
	reconnectCh  chan struct{} // 重连完成信号
}

// isHealthy 检查连接是否可用。
// 注意：此方法不再要求调用方持有锁，Ping 网络操作在锁外执行。
func (h *dbHolder) isHealthy() bool {
	h.mu.Lock()
	if !h.ready || h.db == nil {
		h.mu.Unlock()
		return false
	}
	db := h.db
	h.mu.Unlock()

	sqlDB, err := db.DB()
	if err != nil {
		return false
	}
	return sqlDB.Ping() == nil
}

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
		return nil, xerror.WrapWithXCode(err, xcode.ErrDBConnect)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrDBConnect)
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

// validateConnectOption 校验连接参数
func validateConnectOption(option *Option) error {
	if option == nil {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbutil: connect option is empty")
	}
	if option.Name == "" {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbutil: config name is invalid")
	}
	if option.Config == nil {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbutil: config is empty")
	}
	if len(option.Config.Driver) == 0 || len(option.Config.Dsn) == 0 {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbutil: config is invalid")
	}
	return nil
}

// tryFastPath 快速路径：缓存命中且健康。
// 调用方必须已确认 holder.ready 为 true（但不在锁内）。
// 若健康则返回 (db, true)；否则重置状态并返回 (nil, false)。
func tryFastPath(holder *dbHolder) (*gorm.DB, bool) {
	if holder.isHealthy() {
		holder.mu.Lock()
		db := holder.db
		holder.mu.Unlock()
		return db, true
	}
	// Ping 失败，重新加锁重置状态
	holder.mu.Lock()
	if holder.ready {
		holder.ready = false
		holder.db = nil
		holder.err = nil
	}
	holder.mu.Unlock()
	return nil, false
}

// connectOrWait 等待其他协程完成重连或由当前协程执行重连。
// 调用时必须持有 holder.mu 锁，本函数内部负责加解锁。
func connectOrWait(ctx context.Context, holder *dbHolder, option *Option, opt *connectOption) (*gorm.DB, error) {
	// 如果另一个协程正在重连，等待其完成
	if holder.reconnecting {
		ch := holder.reconnectCh
		holder.mu.Unlock()
		select {
		case <-ch:
			// 重连已完成
		case <-ctx.Done():
			return nil, xerror.WrapWithXCode(ctx.Err(), xcode.ErrDBConnect)
		}
		holder.mu.Lock()
		if holder.ready {
			db := holder.db
			holder.mu.Unlock()
			return db, nil
		}
		holder.mu.Unlock()
		return nil, holder.err
	}

	// 标记正在重连
	holder.reconnecting = true
	holder.reconnectCh = make(chan struct{})
	holder.mu.Unlock()

	// 重连完成后清理：先在锁内重置标志，再解锁，最后关闭通道
	defer func() {
		holder.mu.Lock()
		holder.reconnecting = false
		holder.mu.Unlock()
		close(holder.reconnectCh)
	}()

	dialect := opt.gormDialect
	if dialect == nil {
		v, err := toDialect(option.Config.Driver, option.Config.Dsn)
		if err != nil {
			holder.err = err
			return nil, err
		}
		dialect = v
	}

	// 再次检查 context（可能在等待锁时被取消）
	if err := ctx.Err(); err != nil {
		holder.err = err
		return nil, xerror.WrapWithXCode(err, xcode.ErrDBConnect)
	}

	holder.db, holder.err = connectWithoutCache(dialect, option)
	if holder.err == nil {
		holder.ready = true
	}

	return holder.db, holder.err
}

// ConnectWithContext 通过 context 获取数据库连接
func ConnectWithContext(ctx context.Context, option *Option, optBuilders ...func(*connectOption)) (*gorm.DB, error) {
	if err := validateConnectOption(option); err != nil {
		return nil, err
	}

	// 检查 context 是否已取消
	if err := ctx.Err(); err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrDBConnect)
	}

	opt := new(connectOption)
	for _, builder := range optBuilders {
		builder(opt)
	}

	actual, _ := dbRelation.LoadOrStore(option.Name, &dbHolder{})
	holder := actual.(*dbHolder)

	holder.mu.Lock()

	if holder.ready {
		holder.mu.Unlock()
		if db, ok := tryFastPath(holder); ok {
			return db, nil
		}
		holder.mu.Lock()
	}
	return connectOrWait(ctx, holder, option, opt)
}

// Connect 获取数据库连接
func Connect(option *Option, optBuilders ...func(*connectOption)) (*gorm.DB, error) {
	return ConnectWithContext(context.Background(), option, optBuilders...)
}

// ConnectWithReconnect 强制重建数据库连接（即使缓存连接存在）。
// 当检测到连接断开时可使用此函数。
func ConnectWithReconnect(ctx context.Context, option *Option, optBuilders ...func(*connectOption)) (*gorm.DB, error) {
	if option == nil {
		return nil, xerror.NewXCode(xcode.ErrDBConnect, "dbutil: connect option is empty")
	}

	// 先清理旧连接
	if err := Close(option.Name); err != nil {
		slog.Warn("dbutil: failed to close old connection before reconnect", slog.String("component", "dbutil"), slog.String("name", option.Name), slog.String("error", err.Error()))
	}

	return ConnectWithContext(ctx, option, optBuilders...)
}

// ConnectWith 通过方言获取db
// Deprecated Use Connect(option, ConnectWithGORMDialector(dialect))
func ConnectWith(dialect gorm.Dialector, option *Option) (*gorm.DB, error) {
	return Connect(option, ConnectWithGORMDialector(dialect))
}

// Close 关闭并移除指定名称的数据库连接
func Close(name string) error {
	val, ok := dbRelation.LoadAndDelete(name)
	if !ok {
		return xerror.NewXCode(xcode.ErrDBConnect, fmt.Sprintf("dbutil: connection %q not found", name))
	}

	holder := val.(*dbHolder)
	holder.mu.Lock()
	defer holder.mu.Unlock()

	if holder.db != nil {
		sqlDB, err := holder.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// CloseAll 关闭并移除所有数据库连接
func CloseAll() error {
	var firstErr error
	dbRelation.Range(func(key, value any) bool {
		holder := value.(*dbHolder)
		holder.mu.Lock()
		if holder.db != nil {
			sqlDB, err := holder.db.DB()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err == nil {
				if closeErr := sqlDB.Close(); closeErr != nil && firstErr == nil {
					firstErr = closeErr
				}
			}
		}
		holder.mu.Unlock()
		dbRelation.Delete(key)
		return true
	})
	return firstErr
}
