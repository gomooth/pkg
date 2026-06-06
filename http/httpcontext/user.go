package httpcontext

import "slices"

// User 用户基本信息
type User struct {
	ID      uint
	Account string
	Name    string
	Roles   []IRole

	IP     string            // 用户登录IP
	Extend map[string]string // 扩展信息
}

// GetID 返回用户 ID
func (u *User) GetID() uint {
	if u == nil {
		return 0
	}
	return u.ID
}

// GetAccount 返回用户账号
func (u *User) GetAccount() string {
	if u == nil {
		return ""
	}
	return u.Account
}

// GetName 返回用户姓名
func (u *User) GetName() string {
	if u == nil {
		return ""
	}
	return u.Name
}

// GetRoles 返回用户角色列表的副本
func (u *User) GetRoles() []IRole {
	if u == nil {
		return make([]IRole, 0)
	}
	return slices.Clone(u.Roles)
}

// RolesRef 返回内部 Roles slice 的引用，供框架内部高性能只读场景使用。
// 调用方不可修改返回的 slice，否则会导致未定义行为。
func (u *User) RolesRef() []IRole {
	if u == nil {
		return nil
	}
	return u.Roles
}

// Is 判断用户是否拥有指定角色
func (u *User) Is(role IRole) bool {
	if u == nil {
		return false
	}
	roles := u.RolesRef()
	for i := range roles {
		if roles[i] == role {
			return true
		}
	}
	return false
}

// GetIP 返回用户登录 IP
func (u *User) GetIP() string {
	if u == nil {
		return ""
	}
	return u.IP
}

// GetExtend 返回用户扩展信息的副本
func (u *User) GetExtend() map[string]string {
	if u == nil || u.Extend == nil {
		return make(map[string]string)
	}
	cp := make(map[string]string, len(u.Extend))
	for k, v := range u.Extend {
		cp[k] = v
	}
	return cp
}

// GetExtendValue 返回用户扩展信息中指定字段的值
func (u *User) GetExtendValue(field string) string {
	extend := u.GetExtend()
	return extend[field]
}

// Clone 返回 User 的深拷贝，Roles 和 Extend 均为独立副本
func (u *User) Clone() User {
	roles := slices.Clone(u.Roles)

	var extend map[string]string
	if u.Extend != nil {
		extend = make(map[string]string, len(u.Extend))
		for k, v := range u.Extend {
			extend[k] = v
		}
	}

	return User{
		ID:      u.ID,
		Account: u.Account,
		Name:    u.Name,
		Roles:   roles,
		IP:      u.IP,
		Extend:  extend,
	}
}
