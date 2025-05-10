package jwt

import (
	"errors"
	"time"

	"github.com/gomooth/pkg/http/httpcontext"

	"github.com/golang-jwt/jwt/v5"
)

type token struct {
	claims *claims

	issuer   string        // 发行人
	issuedAt *time.Time    // 发行时间
	duration time.Duration // 有效时长
	secret   []byte        // 加密密钥

	statefulHandler StatefulStore // token 状态处理器
}

// NewToken 初始化 Token
// 默认发行人为 "go-pkg"，可以通过 WithIssuer 修改；
// 默认有效期为 24h，可以通过 WithDuration 设置有效时长
func NewToken(user httpcontext.User) IToken {
	var jwtRoles []string
	for i := range user.Roles {
		jwtRoles = append(jwtRoles, user.Roles[i].String())
	}

	return newTokenWith(&claims{
		Account: user.Account,
		UserID:  user.ID,
		Name:    user.Name,
		Roles:   jwtRoles,

		IP:     user.IP,
		Extend: user.Extend,
	})
}

// NewStatefulToken 初始化有状态的 Token
// 默认发行人为 "go-pkg"，可以通过 WithIssuer 修改；
// 默认有效期为 24h，可以通过 WithDuration 设置有效时长
func NewStatefulToken(user httpcontext.User, handler StatefulStore) IToken {
	var jwtRoles []string
	for i := range user.Roles {
		jwtRoles = append(jwtRoles, user.Roles[i].String())
	}

	t := newTokenWith(&claims{
		Stateful: true,

		Account: user.Account,
		UserID:  user.ID,
		Name:    user.Name,
		Roles:   jwtRoles,

		IP:     user.IP,
		Extend: user.Extend,
	})
	t.statefulHandler = handler

	return t
}

func newTokenWith(c *claims) *token {
	now := time.Now()

	c.Issuer = "gomooth-pkg"
	c.IssuedAt = jwt.NewNumericDate(now)

	d := 24 * time.Hour
	c.ExpiresAt = jwt.NewNumericDate(now.Add(d))

	return &token{
		claims:   c,
		issuer:   c.Issuer,
		issuedAt: &now,
		duration: d,
		secret:   jwtSecret,
	}
}

// SetIssuer 设置 token 发行人，默认为 "go-pkg"
func (t *token) SetIssuer(issuer string) IToken {
	t.issuer = issuer
	t.claims.Issuer = issuer
	return t
}

// SetDuration 设置 token 过期时长，默认为 24h
func (t *token) SetDuration(d time.Duration) IToken {
	t.duration = d
	t.claims.ExpiresAt = jwt.NewNumericDate(t.issuedAt.Add(d))
	return t
}

// SetSecret 设置 token 加密密钥，默认 "go-pkg.JwtSecret"
func (t *token) SetSecret(secret []byte) IToken {
	if len(secret) == 0 {
		secret = jwtSecret
	}

	t.secret = secret
	return t
}

// SetData 设置 token 扩展数据
func (t *token) SetData(key string, val string) IToken {
	if len(key) == 0 {
		return t
	}

	if t.claims.Extend == nil {
		t.claims.Extend = make(map[string]string, 0)
	}

	t.claims.Extend[key] = val

	return t
}

func (t *token) GetUser(fun httpcontext.ToRole) (*httpcontext.User, error) {
	roles, err := t.rolesBy(fun)
	if nil != err {
		return nil, err
	}

	return &httpcontext.User{
		ID:    t.claims.UserID,
		Name:  t.claims.Name,
		Roles: roles,

		IP:     t.claims.IP,
		Extend: t.claims.Extend,
	}, nil
}

// rolesBy 通过 fun 函数将 jwt 的用户角色转换
func (t *token) rolesBy(fun httpcontext.ToRole) ([]httpcontext.IRole, error) {
	var roles []httpcontext.IRole
	for _, v := range t.claims.Roles {
		r, err := fun(v)
		if err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}

	return roles, nil
}

// IsStateful 是否为有状态 jwt token
// 主要用于解决 jwt token 无状态，颁发后不可控。需要业务端注入处理函数
func (t *token) IsStateful() bool {
	return t.claims.Stateful
}

// IsExpired 是否过期
func (t *token) IsExpired() bool {
	return time.Now().Unix() > t.claims.ExpiresAt.Unix()
}

// Refresh 刷新 token
func (t *token) Refresh() {
	now := time.Now()
	t.claims.IssuedAt = jwt.NewNumericDate(now)
	t.claims.ExpiresAt = jwt.NewNumericDate(now.Add(t.duration))
}

// RefreshNear 自动刷新 token，如果当前时间临近过期时间
func (t *token) RefreshNear(d time.Duration) {
	if time.Now().Unix()+int64(d/time.Second) >= t.claims.ExpiresAt.Unix() {
		t.Refresh()
	}
}

// ToString 转成 token 字符串
func (t *token) ToString() (string, error) {
	//t.Refresh()
	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, t.claims)

	tokenStr, err := tokenClaims.SignedString(t.secret)
	if nil != err {
		return "", err
	}

	// 如果是有状态的
	if t.claims.Stateful {
		if t.statefulHandler == nil {
			return "", errors.New("jwt stateful saver error")
		}
		// 存储 token 状态
		if err := t.statefulHandler.Save(t.claims.UserID, tokenStr, t.claims.ExpiresAt.Unix()); nil != err {
			return "", err
		}
	}

	return tokenStr, nil
}
