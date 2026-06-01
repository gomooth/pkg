package httpcontext

// IRole 角色接口，表示用户角色，需实现 String 方法
type IRole interface {
	String() string
}

// ToRole 转换成角色 IRole
type ToRole func(role string) (IRole, error)
