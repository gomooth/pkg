package dbutil

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// --- Connect ---

func TestConnect_Success(t *testing.T) {
	opt := &Option{
		Name: "test-connect-basic",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db, err := Connect(opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	Close("test-connect-basic")
}

func TestConnect_NilOption(t *testing.T) {
	db, err := Connect(nil)
	assert.Nil(t, db)
	assert.Error(t, err)
}

func TestConnect_EmptyName(t *testing.T) {
	opt := &Option{
		Name: "",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}
	db, err := Connect(opt)
	assert.Nil(t, db)
	assert.Error(t, err)
}

func TestConnect_NilConfig(t *testing.T) {
	opt := &Option{
		Name:   "test-nil-config",
		Config: nil,
	}
	db, err := Connect(opt)
	assert.Nil(t, db)
	assert.Error(t, err)
}

func TestConnect_EmptyDriverOrDSN(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsn    string
	}{
		{"empty driver", "", ":memory:"},
		{"empty dsn", "sqlite", ""},
		{"both empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := &Option{
				Name: "test-invalid",
				Config: &ConnectConfig{
					Driver: tt.driver,
					Dsn:    tt.dsn,
				},
			}
			db, err := Connect(opt)
			assert.Nil(t, db)
			assert.Error(t, err)
		})
	}
}

func TestConnect_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opt := &Option{
		Name: "test-cancelled",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}
	db, err := ConnectWithContext(ctx, opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.Nil(t, db)
	assert.Error(t, err)
}

// --- ConnectWith ---

func TestConnectWith(t *testing.T) {
	opt := &Option{
		Name: "test-connectwith",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}
	dialect := sqlite.Open(":memory:")
	db, err := ConnectWith(dialect, opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	Close("test-connectwith")
}

func TestConnectWith_NilOption(t *testing.T) {
	dialect := sqlite.Open(":memory:")
	db, err := ConnectWith(dialect, nil)
	assert.Nil(t, db)
	assert.Error(t, err)
}

// --- CloseAll ---

func TestCloseAll(t *testing.T) {
	// Create two connections
	opt1 := &Option{
		Name: "test-closeall-1",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}
	opt2 := &Option{
		Name: "test-closeall-2",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db1, err := Connect(opt1, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	db2, err := Connect(opt2, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db2)

	// CloseAll should close both
	err = CloseAll()
	assert.NoError(t, err)

	// After CloseAll, individual Close should fail
	assert.Error(t, Close("test-closeall-1"))
	assert.Error(t, Close("test-closeall-2"))
}

func TestCloseAll_Empty(t *testing.T) {
	// CloseAll on empty map should not error
	err := CloseAll()
	assert.NoError(t, err)
}

// --- toDialect ---

func TestToDialect_MySQL(t *testing.T) {
	d, err := toDialect("mysql", "user:pass@tcp(localhost:3306)/dbname")
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestToDialect_TiDB(t *testing.T) {
	d, err := toDialect("tidb", "user:pass@tcp(localhost:4000)/dbname")
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestToDialect_SQLite(t *testing.T) {
	d, err := toDialect("sqlite", ":memory:")
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestToDialect_SQLite3(t *testing.T) {
	d, err := toDialect("sqlite3", ":memory:")
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestToDialect_Postgres(t *testing.T) {
	d, err := toDialect("postgres", "host=localhost port=5432 user=postgres dbname=test sslmode=disable")
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestToDialect_CaseInsensitive(t *testing.T) {
	d, err := toDialect("MySQL", "user:pass@tcp(localhost:3306)/dbname")
	assert.NoError(t, err)
	assert.NotNil(t, d)

	d, err = toDialect("SQLITE", ":memory:")
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestToDialect_Unsupported(t *testing.T) {
	d, err := toDialect("oracle", "oracle-dsn")
	assert.Nil(t, d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dialect convert undefined")
}

// --- Logger functions ---

func TestNewWriter_Printf(t *testing.T) {
	log := slog.Default()
	w := newWriter(log)
	// Should not panic
	w.Printf("test message: %s=%d", "key", 42)
}

func TestNewLogger(t *testing.T) {
	log := slog.Default()
	l := newLogger(log)
	assert.NotNil(t, l)
}

func TestNewLoggerWith(t *testing.T) {
	log := slog.Default()
	conf := &logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
	}
	l := newLoggerWith(log, conf)
	assert.NotNil(t, l)
}

// --- connectWithoutCache ---

func TestConnectWithoutCache_Basic(t *testing.T) {
	opt := &Option{
		Name: "test-without-cache",
		Config: &ConnectConfig{
			Driver:  "sqlite",
			Dsn:     ":memory:",
			MaxIdle: 5,
			MaxOpen: 10,
		},
	}

	db, err := connectWithoutCache(sqlite.Open(":memory:"), opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestConnectWithoutCache_WithMaxLifeTime(t *testing.T) {
	opt := &Option{
		Name: "test-lifetime",
		Config: &ConnectConfig{
			Driver:      "sqlite",
			Dsn:         ":memory:",
			MaxLifeTime: 300,
		},
	}

	db, err := connectWithoutCache(sqlite.Open(":memory:"), opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	sqlDB, err := db.DB()
	assert.NoError(t, err)
	sqlDB.Close()
}

func TestConnectWithoutCache_LogModeOff(t *testing.T) {
	opt := &Option{
		Name: "test-logoff",
		Config: &ConnectConfig{
			Driver:  "sqlite",
			Dsn:     ":memory:",
			LogMode: false,
		},
	}

	db, err := connectWithoutCache(sqlite.Open(":memory:"), opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestConnectWithoutCache_LogModeOnWithLogger(t *testing.T) {
	opt := &Option{
		Name:   "test-logon",
		Logger: slog.Default(),
		Config: &ConnectConfig{
			Driver:  "sqlite",
			Dsn:     ":memory:",
			LogMode: true,
		},
	}

	db, err := connectWithoutCache(sqlite.Open(":memory:"), opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestConnectWithoutCache_LogModeOnWithLoggerAndConf(t *testing.T) {
	opt := &Option{
		Name:       "test-logon-conf",
		Logger:     slog.Default(),
		LoggerConf: &logger.Config{},
		Config: &ConnectConfig{
			Driver:  "sqlite",
			Dsn:     ":memory:",
			LogMode: true,
		},
	}

	db, err := connectWithoutCache(sqlite.Open(":memory:"), opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestConnectWithoutCache_NilConfig(t *testing.T) {
	opt := &Option{
		Name:   "test-nil-cnf",
		Config: nil,
	}

	db, err := connectWithoutCache(sqlite.Open(":memory:"), opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestConnectWithoutCache_InvalidDialector(t *testing.T) {
	opt := &Option{
		Name: "test-invalid-dialect",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	// Use an invalid dialector that will fail on Open
	db, err := connectWithoutCache(&badDialector{}, opt)
	assert.Error(t, err)
	assert.Nil(t, db)
}

// badDialector is a dialector that always fails on Initialize
type badDialector struct{}

func (b *badDialector) Name() string                                                { return "bad" }
func (b *badDialector) Initialize(db *gorm.DB) error                                { return fmt.Errorf("forced error") }
func (b *badDialector) Migrator(db *gorm.DB) gorm.Migrator                          { return nil }
func (b *badDialector) DataTypeOf(field *schema.Field) string                       { return "" }
func (b *badDialector) DefaultValueOf(field *schema.Field) clause.Expression        { return clause.Expr{} }
func (b *badDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {}
func (b *badDialector) QuoteTo(writer clause.Writer, column string)                 {}
func (b *badDialector) Explain(sql string, vars ...interface{}) string              { return "" }

// --- ConnectWithReconnect ---

func TestConnectWithReconnect_NilOption(t *testing.T) {
	db, err := ConnectWithReconnect(context.Background(), nil)
	assert.Nil(t, db)
	assert.Error(t, err)
}

func TestConnectWithReconnect_ForceReconnect(t *testing.T) {
	opt := &Option{
		Name: "test-force-reconnect-2",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db1, err := Connect(opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	// Force reconnect should succeed even if connection exists
	db2, err := ConnectWithReconnect(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db2)

	var result int
	err = db2.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	Close("test-force-reconnect-2")
}

// --- Close edge cases ---

func TestClose_NotFound(t *testing.T) {
	err := Close("nonexistent-connection")
	assert.Error(t, err)
}

// --- Integration: Connect with toDialect ---

func TestConnect_WithSQLite_Dialect(t *testing.T) {
	opt := &Option{
		Name: "test-sqlite-dialect",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	db, err := Connect(opt)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	Close("test-sqlite-dialect")
}

func TestConnect_WithInvalidDriver(t *testing.T) {
	opt := &Option{
		Name: "test-invalid-driver",
		Config: &ConnectConfig{
			Driver: "oracle",
			Dsn:    "invalid-dsn",
		},
	}

	db, err := Connect(opt)
	assert.Error(t, err)
	assert.Nil(t, db)
}

// --- dbWriter.Printf direct test ---

func TestDbWriter_Printf(t *testing.T) {
	w := &dbWriter{log: slog.Default()}
	w.Printf("hello %s, number %d", "world", 123)
}

// --- ConnectWithContext: second context cancellation after lock ---

func TestConnectWithContext_ContextCancelledAfterFirstCheck(t *testing.T) {
	// This tests the second ctx.Err() check (line 147-149)
	// We can't easily control the timing, but we can test with a cancelled context
	// that is cancelled between the first check and the second check.
	// Since we can't easily orchestrate that timing, we test the basic cancelled case.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opt := &Option{
		Name: "test-ctx-cancel-2",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}
	db, err := ConnectWithContext(ctx, opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.Nil(t, db)
	assert.Error(t, err)
}

// --- CloseAll with holder that has nil db ---

func TestCloseAll_NilDB(t *testing.T) {
	// Store a holder with nil db into dbRelation
	dbRelation.Store("test-nil-db", &dbHolder{ready: false, db: nil})

	err := CloseAll()
	assert.NoError(t, err)
}

// --- isHealthy coverage ---

func TestIsHealthy_ReadyButNilDB(t *testing.T) {
	h := &dbHolder{ready: true, db: nil}
	assert.False(t, h.isHealthy())
}

func TestIsHealthy_NotReady(t *testing.T) {
	h := &dbHolder{ready: false, db: nil}
	assert.False(t, h.isHealthy())
}

func TestIsHealthy_WithWorkingDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	h := &dbHolder{ready: true, db: db}
	assert.True(t, h.isHealthy())

	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestIsHealthy_WithClosedDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.Close()

	h := &dbHolder{ready: true, db: db}
	assert.False(t, h.isHealthy())
}

// --- ConnectWithContext: cached healthy connection path ---

func TestConnectWithContext_CachedHealthyConnection(t *testing.T) {
	opt := &Option{
		Name: "test-cached-healthy",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	// First connect creates the connection
	db1, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	// Second connect should return the cached (healthy) connection via isHealthy
	db2, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.Same(t, db1, db2)

	Close("test-cached-healthy")
}

// --- ConnectWithContext: broken cached connection triggers reconnect ---

func TestConnectWithContext_BrokenCacheReconnects(t *testing.T) {
	opt := &Option{
		Name: "test-broken-cache",
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	// First connect
	db1, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	// Break the connection by closing the underlying sql.DB
	sqlDB, err := db1.DB()
	assert.NoError(t, err)
	sqlDB.Close()

	// Reconnect should detect broken connection and create a new one
	db2, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db2)

	// The new connection should be functional
	var result int
	err = db2.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	Close("test-broken-cache")
}

// --- ConnectWithReconnect: Close returns warning when old connection has issues ---

func TestConnectWithReconnect_CloseWarning(t *testing.T) {
	// Store a broken holder directly so Close will encounter issues
	name := "test-reconnect-warning"
	holder := &dbHolder{ready: true, db: nil}
	dbRelation.Store(name, holder)

	opt := &Option{
		Name: name,
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}

	// ConnectWithReconnect should warn but still proceed (Close fails because db is nil in holder but ready=true)
	db, err := ConnectWithReconnect(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	assert.NoError(t, err)
	assert.NotNil(t, db)

	Close(name)
}

// --- Close: holder with nil db (ready=true but db=nil) ---

func TestClose_HolderWithNilDB(t *testing.T) {
	name := "test-close-nil-db"
	dbRelation.Store(name, &dbHolder{ready: true, db: nil})

	err := Close(name)
	assert.NoError(t, err)
}

// --- CloseAll: holder with DB() returning error ---

func TestCloseAll_DBError(t *testing.T) {
	// Create a real connection, close its sql.DB, then test CloseAll
	// This exercises the error paths in CloseAll
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	sqlDB, _ := db.DB()
	// Close the underlying connection to make DB() still work but Close() might fail
	sqlDB.Close()

	name := "test-closeall-err"
	dbRelation.Store(name, &dbHolder{ready: true, db: db})

	// CloseAll should still complete (might return error from closed db)
	CloseAll()
}
