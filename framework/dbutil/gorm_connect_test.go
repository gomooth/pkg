package dbutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
)

func TestConnectWithContext_CachesConnection(t *testing.T) {
	opt := &Option{
		Name: "test-cache",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db1, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	db2, err := ConnectWithContext(context.Background(), opt)
	assert.NoError(t, err)
	assert.Same(t, db1, db2, "should return cached connection")

	// 清理
	assert.NoError(t, Close("test-cache"))
}

func TestConnectWithContext_ReconnectsOnBrokenConnection(t *testing.T) {
	opt := &Option{
		Name: "test-reconnect",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db1, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	// 模拟连接断开：关闭底层 sql.DB
	sqlDB, err := db1.DB()
	assert.NoError(t, err)
	sqlDB.Close()

	// ConnectWithContext 应检测到不健康并重建连接
	db2, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db2)

	// 新连接应该可用
	var result int
	err = db2.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	// 清理
	Close("test-reconnect")
}

func TestConnectWithReconnect(t *testing.T) {
	opt := &Option{
		Name: "test-force-reconnect",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db1, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	// Force reconnect
	db2, err := ConnectWithReconnect(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db2)

	// 新连接可用
	var result int
	err = db2.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	// 清理
	Close("test-force-reconnect")
}

func TestClose_RemovesConnection(t *testing.T) {
	opt := &Option{
		Name: "test-close",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	_, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)

	assert.NoError(t, Close("test-close"))
	assert.Error(t, Close("test-close"), "second close should fail: connection not found")
}
