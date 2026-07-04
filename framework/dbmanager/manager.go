package dbmanager

import (
	"fmt"
	"sync"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
	"gorm.io/gorm"
)

// memoryManager 基于 sync.Map 的内存数据库连接管理器
type memoryManager struct {
	connections sync.Map
}

var _ IDatabaseManager = (*memoryManager)(nil)

// NewMemoryManager 创建内存数据库连接管理器
func NewMemoryManager() IDatabaseManager {
	return &memoryManager{}
}

// Register 注册数据库连接
func (m *memoryManager) Register(name string, dbc *gorm.DB) error {
	if name == "" {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbmanager: connection name cannot be empty")
	}
	if dbc == nil {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbmanager: database connection cannot be nil")
	}
	m.connections.Store(name, dbc)
	return nil
}

// Get 获取数据库连接
func (m *memoryManager) Get(name string) (*gorm.DB, error) {
	if name == "" {
		return nil, xerror.NewXCode(xcode.ErrDBConnect, "dbmanager: connection name cannot be empty")
	}
	val, ok := m.connections.Load(name)
	if !ok {
		return nil, xerror.NewXCode(xcode.ErrDBConnect, fmt.Sprintf("dbmanager: connection %q not found", name))
	}
	db, ok := val.(*gorm.DB)
	if !ok {
		return nil, xerror.NewXCode(xcode.ErrDBConnect, fmt.Sprintf("dbmanager: connection %q has unexpected type", name))
	}
	return db, nil
}

// Unregister 注销数据库连接
func (m *memoryManager) Unregister(name string) error {
	if name == "" {
		return xerror.NewXCode(xcode.ErrDBConnect, "dbmanager: connection name cannot be empty")
	}
	m.connections.Delete(name)
	return nil
}

// CloseAll 关闭所有数据库连接
func (m *memoryManager) CloseAll() error {
	var firstErr error
	m.connections.Range(func(key, value any) bool {
		db, ok := value.(*gorm.DB)
		if !ok {
			return true
		}
		if sqlDB, err := db.DB(); err == nil {
			if closeErr := sqlDB.Close(); closeErr != nil && firstErr == nil {
				firstErr = closeErr
			}
		}
		m.connections.Delete(key)
		return true
	})
	return firstErr
}

// List 返回所有已注册的数据库连接名
func (m *memoryManager) List() []string {
	var names []string
	m.connections.Range(func(key, _ any) bool {
		if name, ok := key.(string); ok {
			names = append(names, name)
		}
		return true
	})
	return names
}
