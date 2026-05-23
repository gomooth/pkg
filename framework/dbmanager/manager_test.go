package dbmanager

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewMemoryManager(t *testing.T) {
	manager := NewMemoryManager()
	assert.Implements(t, (*IDatabaseManager)(nil), manager)
}

func TestRegisterAndGet(t *testing.T) {
	manager := NewMemoryManager()

	// 创建测试数据库连接
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 测试注册后能正常获取
	err = manager.Register("test_db", db)
	assert.NoError(t, err)

	retrievedDB, err := manager.Get("test_db")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedDB)
	assert.Equal(t, db, retrievedDB)
}

func TestGetNotFound(t *testing.T) {
	manager := NewMemoryManager()

	// 测试获取不存在的数据库连接
	_, err := manager.Get("nonexistent_db")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection \"nonexistent_db\" not found")
}

func TestRegisterEmptyName(t *testing.T) {
	manager := NewMemoryManager()

	// 创建测试数据库连接
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 测试注册空名称
	err = manager.Register("", db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection name cannot be empty")
}

func TestRegisterNilDB(t *testing.T) {
	manager := NewMemoryManager()

	// 测试注册nil数据库连接
	err := manager.Register("test_db", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection cannot be nil")
}

func TestRegisterDuplicate(t *testing.T) {
	manager := NewMemoryManager()

	// 创建第一个数据库连接
	db1, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 创建第二个数据库连接
	db2, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 第一次注册
	err = manager.Register("test_db", db1)
	assert.NoError(t, err)

	// 重复注册同名数据库（应该不报错）
	err = manager.Register("test_db", db2)
	assert.NoError(t, err)

	// 获取的应该是最新注册的数据库
	retrievedDB, err := manager.Get("test_db")
	assert.NoError(t, err)
	assert.Equal(t, db2, retrievedDB)
	assert.NotEqual(t, db1, retrievedDB)
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewMemoryManager()
	const numGoroutines = 100
	const numDBs = 5
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	// 创建测试数据库
	dbs := make([]*gorm.DB, numDBs)
	for i := 0; i < numDBs; i++ {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		assert.NoError(t, err)
		dbs[i] = db
	}

	// 启动100个goroutine并发执行Register和Get
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			dbIndex := id % numDBs
			dbName := fmt.Sprintf("db_%d", dbIndex)

			// 执行Register和Get操作
			err := manager.Register(dbName, dbs[dbIndex])
			if err != nil {
				errChan <- err
				return
			}

			retrievedDB, err := manager.Get(dbName)
			if err != nil {
				errChan <- err
				return
			}

			// 确保获取到正确的数据库连接
			if retrievedDB != dbs[dbIndex] {
				errChan <- fmt.Errorf("expected database %d, got different one", dbIndex)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误发生
	for err := range errChan {
		assert.NoError(t, err)
	}
}

// TestGetEmptyName 测试获取空名称的数据库连接
func TestGetEmptyName(t *testing.T) {
	manager := NewMemoryManager()

	_, err := manager.Get("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection name cannot be empty")
}

// TestTypeConversion 测试类型转换
func TestTypeConversion(t *testing.T) {
	manager := NewMemoryManager()

	// 创建一个非*gorm.DB类型的值
	wrongType := "not a database"

	// 使用unsafe方式存储，模拟类型错误
	m := manager.(*memoryManager)
	m.connections.Store("wrong_type", wrongType)

	// 测试获取时类型错误
	_, err := manager.Get("wrong_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has unexpected type")
}

// BenchmarkRegister 性能测试
func BenchmarkRegister(b *testing.B) {
	manager := NewMemoryManager()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			manager.Register(fmt.Sprintf("db_%d", i), db)
			i++
		}
	})
}

// BenchmarkGet 性能测试
func BenchmarkGet(b *testing.B) {
	manager := NewMemoryManager()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}

	// 预先注册100个数据库连接
	for i := 0; i < 100; i++ {
		manager.Register(fmt.Sprintf("db_%d", i), db)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			manager.Get(fmt.Sprintf("db_%d", i%100))
			i++
		}
	})
}
