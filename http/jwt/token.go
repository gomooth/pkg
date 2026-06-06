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
	leeway   time.Duration // 过期容差，IsExpired 时向前偏移

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

// TokenBuilder 用于链式构造 IToken 实例
type TokenBuilder struct {
	secret          []byte
	user            httpcontext.User
	issuer          string
	duration        time.Duration
	signingMethod   jwt.SigningMethod
	statefulHandler StatefulStore
	extend          map[string]string
}

// NewTokenBuilder 创建 Token 构建器。
// secret 为加密密钥，不能为空；user 为 token 关联的用户信息。
func NewTokenBuilder(secret []byte, user httpcontext.User) *TokenBuilder {
	return &TokenBuilder{
		secret: secret,
		user:   user,
	}
}

// WithIssuer 设置签发者，默认为 "gomooth/pkg"
func (b *TokenBuilder) WithIssuer(issuer string) *TokenBuilder {
	b.issuer = issuer
	return b
}

// WithExpiration 设置 Token 过期时间
func (b *TokenBuilder) WithExpiration(d time.Duration) *TokenBuilder {
	b.duration = d
	return b
}

// WithSigningMethod 设置签名方法，默认为 HS256
func (b *TokenBuilder) WithSigningMethod(m jwt.SigningMethod) *TokenBuilder {
	b.signingMethod = m
	return b
}

// WithStatefulStore 设置 Token 状态存储后端，设置后 token 为有状态
func (b *TokenBuilder) WithStatefulStore(store StatefulStore) *TokenBuilder {
	b.statefulHandler = store
	return b
}

// WithExtendData 设置扩展数据
func (b *TokenBuilder) WithExtendData(key, val string) *TokenBuilder {
	if b.extend == nil {
		b.extend = make(map[string]string)
	}
	b.extend[key] = val
	return b
}

// Build 构建 IToken 实例。若 secret 为空则返回错误。
func (b *TokenBuilder) Build() (IToken, error) {
	if len(b.secret) == 0 {
		return nil, xerror.NewXCode(xcode.ErrJWTSecretNotSet, "jwt: secret must not be empty")
	}

	isStateful := b.statefulHandler != nil
	c := &claims{
		Stateful: isStateful,
		Account:  b.user.Account,
		UserID:   b.user.ID,
		Name:     b.user.Name,
		Roles:    rolesFromUser(b.user),
		Extend:   b.user.Extend,
	}
	if len(b.extend) > 0 {
		if c.Extend == nil {
			c.Extend = make(map[string]string, len(b.extend))
		}
		for k, v := range b.extend {
			c.Extend[k] = v
		}
	}

	t := newTokenWith(c)
	t.secret = b.secret
	t.statefulHandler = b.statefulHandler

	if b.issuer != "" {
		t.SetIssuer(b.issuer)
	}
	if b.duration > 0 {
		t.SetDuration(b.duration)
	}
	if b.signingMethod != nil {
		t.SetSigningMethod(b.signingMethod)
	}

	return t, nil
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
// leeway > 0 时，过期判定会提前 leeway 时长，避免时钟偏移导致的误判
func (t *token) IsExpired() bool {
	if t.claims.ExpiresAt == nil {
		return true
	}
	return time.Now().After(t.claims.ExpiresAt.Time.Add(-t.leeway))
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
