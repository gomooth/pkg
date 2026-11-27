package dbutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakePlugin 测试用假插件：记录 Initialize 是否被调用
type fakePlugin struct {
	name string
	init func(*gorm.DB) error
}

func (f *fakePlugin) Name() string { return f.name }

func (f *fakePlugin) Initialize(db *gorm.DB) error {
	if f.init != nil {
		return f.init(db)
	}
	return nil
}

// TestConnectWithGORMPluginRegistered 假插件经 Connect 注册后 Initialize 被调用
func TestConnectWithGORMPluginRegistered(t *testing.T) {
	called := false
	p := &fakePlugin{name: "fake", init: func(*gorm.DB) error { called = true; return nil }}
	_, err := Connect(&Option{Name: "t", Config: &ConnectConfig{Driver: "sqlite", Dsn: "file::memory:?cache=shared"}}, WithGORMPlugin(p))
	require.NoError(t, err)
	assert.True(t, called, "插件 Initialize 应被 gorm.Open 调用")
}

// TestConnectWithNilPluginSkipped nil 插件不 panic、不注册
func TestConnectWithNilPluginSkipped(t *testing.T) {
	db, err := Connect(&Option{Name: "t-nil-plugin", Config: &ConnectConfig{Driver: "sqlite", Dsn: "file::memory:?cache=shared"}}, WithGORMPlugin(nil))
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Empty(t, db.Plugins, "nil 插件不应注册进 gorm.Config.Plugins")
}
