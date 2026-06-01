package httpcontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// testRole 用于 user 测试
type userTestRole string

func (r userTestRole) String() string { return string(r) }

func TestUser_GetID(t *testing.T) {
	t.Run("normal user", func(t *testing.T) {
		u := &User{ID: 42}
		assert.Equal(t, uint(42), u.GetID())
	})
	t.Run("nil user returns 0", func(t *testing.T) {
		var u *User
		assert.Equal(t, uint(0), u.GetID())
	})
}

func TestUser_GetAccount(t *testing.T) {
	t.Run("normal user", func(t *testing.T) {
		u := &User{Account: "alice"}
		assert.Equal(t, "alice", u.GetAccount())
	})
	t.Run("nil user returns empty", func(t *testing.T) {
		var u *User
		assert.Equal(t, "", u.GetAccount())
	})
}

func TestUser_GetName(t *testing.T) {
	t.Run("normal user", func(t *testing.T) {
		u := &User{Name: "Alice"}
		assert.Equal(t, "Alice", u.GetName())
	})
	t.Run("nil user returns empty", func(t *testing.T) {
		var u *User
		assert.Equal(t, "", u.GetName())
	})
}

func TestUser_GetRoles(t *testing.T) {
	t.Run("normal user returns roles copy", func(t *testing.T) {
		roles := []IRole{userTestRole("admin"), userTestRole("user")}
		u := &User{Roles: roles}
		got := u.GetRoles()
		assert.Equal(t, roles, got)
		// 验证返回的是副本
		got[0] = userTestRole("hacked")
		assert.Equal(t, userTestRole("admin"), u.Roles[0])
	})
	t.Run("nil user returns empty slice", func(t *testing.T) {
		var u *User
		got := u.GetRoles()
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestUser_RolesRef(t *testing.T) {
	t.Run("normal user returns internal slice", func(t *testing.T) {
		roles := []IRole{userTestRole("admin")}
		u := &User{Roles: roles}
		ref := u.RolesRef()
		assert.Equal(t, roles, ref)
		// 验证是引用
		assert.Same(t, &roles[0], &ref[0])
	})
	t.Run("nil user returns nil", func(t *testing.T) {
		var u *User
		assert.Nil(t, u.RolesRef())
	})
}

func TestUser_Is(t *testing.T) {
	t.Run("nil user returns false", func(t *testing.T) {
		var u *User
		assert.False(t, u.Is(userTestRole("admin")))
	})
	t.Run("matching role returns true", func(t *testing.T) {
		u := &User{Roles: []IRole{userTestRole("admin"), userTestRole("user")}}
		assert.True(t, u.Is(userTestRole("admin")))
		assert.True(t, u.Is(userTestRole("user")))
	})
	t.Run("non-matching role returns false", func(t *testing.T) {
		u := &User{Roles: []IRole{userTestRole("user")}}
		assert.False(t, u.Is(userTestRole("admin")))
	})
	t.Run("empty roles returns false", func(t *testing.T) {
		u := &User{Roles: []IRole{}}
		assert.False(t, u.Is(userTestRole("admin")))
	})
}

func TestUser_GetIP(t *testing.T) {
	t.Run("normal user", func(t *testing.T) {
		u := &User{IP: "192.168.1.1"}
		assert.Equal(t, "192.168.1.1", u.GetIP())
	})
	t.Run("nil user returns empty", func(t *testing.T) {
		var u *User
		assert.Equal(t, "", u.GetIP())
	})
}

func TestUser_GetExtend(t *testing.T) {
	t.Run("normal extend returns copy", func(t *testing.T) {
		u := &User{Extend: map[string]string{"k1": "v1", "k2": "v2"}}
		got := u.GetExtend()
		assert.Equal(t, map[string]string{"k1": "v1", "k2": "v2"}, got)
		// 修改返回值不影响原始
		got["k1"] = "hacked"
		assert.Equal(t, "v1", u.Extend["k1"])
	})
	t.Run("nil user returns empty map", func(t *testing.T) {
		var u *User
		got := u.GetExtend()
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
	t.Run("nil extend returns empty map", func(t *testing.T) {
		u := &User{Extend: nil}
		got := u.GetExtend()
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestUser_GetExtendValue(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		u := &User{Extend: map[string]string{"email": "test@example.com"}}
		assert.Equal(t, "test@example.com", u.GetExtendValue("email"))
	})
	t.Run("non-existing key returns empty", func(t *testing.T) {
		u := &User{Extend: map[string]string{"email": "test@example.com"}}
		assert.Equal(t, "", u.GetExtendValue("phone"))
	})
	t.Run("nil user returns empty", func(t *testing.T) {
		var u *User
		assert.Equal(t, "", u.GetExtendValue("email"))
	})
}

func TestUser_Clone(t *testing.T) {
	t.Run("deep copy independence", func(t *testing.T) {
		u := User{
			ID:      1,
			Account: "alice",
			Name:    "Alice",
			Roles:   []IRole{userTestRole("admin")},
			IP:      "127.0.0.1",
			Extend:  map[string]string{"dept": "engineering"},
		}
		clone := u.Clone()

		// 值字段相等
		assert.Equal(t, u.ID, clone.ID)
		assert.Equal(t, u.Account, clone.Account)
		assert.Equal(t, u.Name, clone.Name)
		assert.Equal(t, u.IP, clone.IP)

		// 修改 clone 不影响原始
		clone.ID = 999
		clone.Account = "bob"
		clone.Roles[0] = userTestRole("hacked")
		clone.Extend["dept"] = "sales"

		assert.Equal(t, uint(1), u.ID)
		assert.Equal(t, "alice", u.Account)
		assert.Equal(t, userTestRole("admin"), u.Roles[0])
		assert.Equal(t, "engineering", u.Extend["dept"])
	})

	t.Run("nil extend clone", func(t *testing.T) {
		u := User{ID: 1, Extend: nil}
		clone := u.Clone()
		assert.Nil(t, clone.Extend)
	})
}
