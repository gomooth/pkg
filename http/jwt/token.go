package jwt

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/xerror"

	"github.com/golang-jwt/jwt/v5"
)

type token struct {
	claims *claims

	issuer   string        // 发行人
	issuedAt *time.Time    // 发行时间
	duration time.Duration // 有效时长
	secret   []byte        // 加密密钥

	signingMethod jwt.SigningMethod // 签名方法，默认 HS256

	statefulHandler StatefulStore // token 状态处理器
}

// rolesFromUser 辅助函数：从用户对象中提取角色列表
func rolesFromUser(user httpcontext.User) []string {
	jwtRoles := make([]string, 0, len(user.Roles))
	for i := range user.Roles {
		jwtRoles = append(jwtRoles, user.Roles[i].String())
	}
	return jwtRoles
}

// NewToken 初始化 Token
// secret 为加密密钥，不能为空，否则返回错误。
// 默认发行人为 "gomooth/pkg"，可以通过 SetIssuer 修改；
// 默认有效期为 24h，可以通过 SetDuration 设置有效时长。
func NewToken(secret []byte, user httpcontext.User) (IToken, error) {
	if len(secret) == 0 {
		return nil, xerror.NewXCode(xcode.ErrJWTSecretNotSet, "jwt: secret must not be empty")
	}

	t := newTokenWith(&claims{
		Account: user.Account,
		UserID:  user.ID,
		Name:    user.Name,
		Roles:   rolesFromUser(user),
		Extend:  user.Extend,
	})
	t.secret = secret
	return t, nil
}

// NewStatefulToken 初始化有状态的 Token
// secret 为加密密钥，不能为空，否则返回错误。
func NewStatefulToken(secret []byte, user httpcontext.User, handler StatefulStore) (IToken, error) {
	if len(secret) == 0 {
		return nil, xerror.NewXCode(xcode.ErrJWTSecretNotSet, "jwt: secret must not be empty")
	}

	t := newTokenWith(&claims{
		Stateful: true,
		Account:  user.Account,
		UserID:   user.ID,
		Name:     user.Name,
		Roles:    rolesFromUser(user),
		Extend:   user.Extend,
	})
	t.statefulHandler = handler
	t.secret = secret
	return t, nil
}

func newTokenWith(c *claims) *token {
	now := time.Now()

	c.Issuer = "gomooth/pkg"
	c.IssuedAt = jwt.NewNumericDate(now)

	d := 24 * time.Hour
	c.ExpiresAt = jwt.NewNumericDate(now.Add(d))

	return &token{
		claims:        c,
		issuer:        c.Issuer,
		issuedAt:      &now,
		duration:      d,
		signingMethod: jwt.SigningMethodHS256,
	}
}

// SetIssuer 设置 token 发行人，默认为 "gomooth/pkg"
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

// SetData 设置 token 扩展数据
func (t *token) SetData(key string, val string) IToken {
	if len(key) == 0 {
		return t
	}

	if t.claims.Extend == nil {
		t.claims.Extend = make(map[string]string)
	}

	t.claims.Extend[key] = val

	return t
}

// SetSigningMethod 设置签名方法，默认为 HS256。
// 使用非对称算法（如 RS256/ES256）时需通过此方法指定。
func (t *token) SetSigningMethod(m jwt.SigningMethod) IToken {
	if m != nil {
		t.signingMethod = m
	}
	return t
}

func (t *token) GetUser(fun httpcontext.ToRole) (*httpcontext.User, error) {
	roles, err := t.rolesBy(fun)
	if err != nil {
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
func (t *token) IsStateful() bool {
	return t.claims.Stateful
}

// IsExpired 是否过期
func (t *token) IsExpired() bool {
	if t.claims.ExpiresAt == nil {
		return true
	}
	return time.Now().After(t.claims.ExpiresAt.Time)
}

// Refresh 刷新 token
func (t *token) Refresh() {
	now := time.Now()
	t.claims.IssuedAt = jwt.NewNumericDate(now)
	t.claims.ExpiresAt = jwt.NewNumericDate(now.Add(t.duration))
}

// RefreshNear 自动刷新 token，如果当前时间临近过期时间
func (t *token) RefreshNear(d time.Duration) {
	if t.claims.ExpiresAt == nil {
		return
	}
	if time.Now().Add(d).After(t.claims.ExpiresAt.Time) {
		t.Refresh()
	}
}

// ToString 转成 token 字符串
func (t *token) ToString(ctx context.Context) (string, error) {
	if len(t.secret) == 0 {
		return "", xerror.NewXCode(xcode.ErrJWTSecretNotSet, "jwt: secret not set")
	}

	tokenClaims := jwt.NewWithClaims(t.signingMethod, t.claims)

	tokenStr, err := tokenClaims.SignedString(t.secret)
	if err != nil {
		return "", err
	}

	// 如果是有状态的
	if t.claims.Stateful {
		if t.statefulHandler == nil {
			return "", xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: stateful store not configured")
		}
		// 存储 token 状态
		if err := t.statefulHandler.Save(ctx, t.claims.UserID, tokenStr, t.claims.ExpiresAt.Unix()); err != nil {
			return "", err
		}
	}

	return tokenStr, nil
}
