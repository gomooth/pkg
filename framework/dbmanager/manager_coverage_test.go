package dbmanager

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMemoryManager_Unregister_DoesNotClose 验证 Unregister 仅移除管理器中的引用，
// 不关闭底层连接
func TestMemoryManager_Unregister_DoesNotClose(t *testing.T) {
	manager := NewMemoryManager()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = manager.Register("test_db", db)
	assert.NoError(t, err)

	// Unregister 不应关闭底层连接
	err = manager.Unregister("test_db")
	assert.NoError(t, err)

	// 连接应从管理器中移除
	_, err = manager.Get("test_db")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection \"test_db\" not found")

	// 底层 sql.DB 应仍然可用
	sqlDB, err := db.DB()
	assert.NoError(t, err)
	assert.NoError(t, sqlDB.Ping())
	sqlDB.Close()
}

// TestMemoryManager_Unregister_EmptyName 验证 Unregister 空名称返回错误
func TestMemoryManager_Unregister_EmptyName(t *testing.T) {
	manager := NewMemoryManager()

	err := manager.Unregister("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection name cannot be empty")
}

// TestMemoryManager_Unregister_NotExist 验证 Unregister 不存在的连接不报错
func TestMemoryManager_Unregister_NotExist(t *testing.T) {
	manager := NewMemoryManager()

	err := manager.Unregister("nonexistent")
	assert.NoError(t, err)
}

// TestMemoryManager_CloseAll_Success 验证 CloseAll 正常关闭所有连接
func TestMemoryManager_CloseAll_Success(t *testing.T) {
	manager := NewMemoryManager()

	db1, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	db2, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = manager.Register("db1", db1)
	assert.NoError(t, err)
	err = manager.Register("db2", db2)
	assert.NoError(t, err)

	err = manager.CloseAll()
	assert.NoError(t, err)

	// 所有连接应已从管理器中移除
	_, err = manager.Get("db1")
	assert.Error(t, err)
	_, err = manager.Get("db2")
	assert.Error(t, err)
}

// TestMemoryManager_CloseAll_PartialError 验证 CloseAll 遇到非 *gorm.DB 条目时仍继续处理其余连接
func TestMemoryManager_CloseAll_PartialError(t *testing.T) {
	manager := NewMemoryManager()
	m := manager.(*memoryManager)

	// 正常连接
	goodDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = manager.Register("good_db", goodDB)
	assert.NoError(t, err)

	// 在 sync.Map 中直接插入非 *gorm.DB 类型，模拟类型断言失败
	m.connections.Store("bad_entry", 42)

	// CloseAll 不应因 bad_entry 类型断言失败而中断
	err = manager.CloseAll()
	assert.NoError(t, err) // 非数据库类型被跳过，不产生错误

	// good_db 应被正常关闭并从管理器中移除
	_, getErr := manager.Get("good_db")
	assert.Error(t, getErr)
}

// TestMemoryManager_CloseAll_Empty 验证 CloseAll 在无连接时不报错
func TestMemoryManager_CloseAll_Empty(t *testing.T) {
	manager := NewMemoryManager()

	err := manager.CloseAll()
	assert.NoError(t, err)
}

// TestMemoryManager_CloseAll_UnexpectedType 验证 CloseAll 遇到非 *gorm.DB 类型时跳过
func TestMemoryManager_CloseAll_UnexpectedType(t *testing.T) {
	manager := NewMemoryManager()
	m := manager.(*memoryManager)

	// 存入非 *gorm.DB 类型
	m.connections.Store("wrong_type", "not a database")

	// 正常连接
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = manager.Register("good_db", db)
	assert.NoError(t, err)

	// CloseAll 不应因类型断言失败而 panic
	err = manager.CloseAll()
	assert.NoError(t, err)
}

// TestMemoryManager_CloseAll_ClosedDB 验证 CloseAll 中 db.DB() 返回错误时仍继续处理
func TestMemoryManager_CloseAll_ClosedDB(t *testing.T) {
	manager := NewMemoryManager()

	// 创建一个连接后关闭底层 sql.DB，使后续 DB() 调用可能返回错误
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.Close()

	err = manager.Register("closed_db", db)
	assert.NoError(t, err)

	// 另一个正常连接
	goodDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = manager.Register("good_db", goodDB)
	assert.NoError(t, err)

	// CloseAll 不应 panic，且继续处理其他连接
	err = manager.CloseAll()
	// 无论 closed_db 是否报错，good_db 应被正常关闭并移除
	_, getErr := manager.Get("good_db")
	assert.Error(t, getErr)
}

// TestMemoryManager_ConcurrentRegisterUnregister 并发执行 Register / Unregister / Get
func TestMemoryManager_ConcurrentRegisterUnregister(t *testing.T) {
	manager := NewMemoryManager()
	const numGoroutines = 50
	var wg sync.WaitGroup

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	errChan := make(chan error, numGoroutines*3)

	// 并发 Register
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent_db_%d", id)
			if err := manager.Register(name, db); err != nil {
				errChan <- err
			}
		}(i)
	}

	// 并发 Unregister
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent_db_%d", id)
			if err := manager.Unregister(name); err != nil {
				errChan <- err
			}
		}(i)
	}

	// 并发 Get
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent_db_%d", id)
			_, _ = manager.Get(name) // 可能找到也可能找不到，都正常
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		assert.NoError(t, err)
	}
}

// TestMemoryManager_EmptyName_Error 验证 Register 空名称返回错误
func TestMemoryManager_EmptyName_Error(t *testing.T) {
	manager := NewMemoryManager()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = manager.Register("", db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection name cannot be empty")
}

// TestMemoryManager_NilConnection_Error 验证 Register nil 连接返回错误
func TestMemoryManager_NilConnection_Error(t *testing.T) {
	manager := NewMemoryManager()

	err := manager.Register("nil_db", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection cannot be nil")
}

// TestMemoryManager_UnexpectedType_Error 验证 Get 时类型断言失败返回错误
func TestMemoryManager_UnexpectedType_Error(t *testing.T) {
	manager := NewMemoryManager()
	m := manager.(*memoryManager)

	m.connections.Store("bad_type", 12345)

	_, err := manager.Get("bad_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has unexpected type")
}
