package jwt

import (
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
)

// Option jwt 配置参数，通过 NewOption 构造，With* 函数配置可选参数
type Option struct {
	roleConvert           httpcontext.ToRole
	refreshDuration       time.Duration
	secret                []byte
	silentMode            bool
	allowQueryStringToken bool
	queryStringTokenPaths []string
	leeway                time.Duration
	legacySecrets         [][]byte
	signingMethods        []string
}

// NewOption 创建 jwt 配置参数。
// secret 为加密密钥，不能为空；roleConvert 为角色转化函数，不能为 nil。
// 可选参数通过 With* 函数配置。
func NewOption(secret []byte, roleConvert httpcontext.ToRole, opts ...func(*Option)) *Option {
	o := &Option{
		secret:      secret,
		roleConvert: roleConvert,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithRefreshDuration 设置过期自动刷新临界时长。零则表示不自动刷新
func WithRefreshDuration(d time.Duration) func(*Option) {
	return func(o *Option) {
		o.refreshDuration = d
	}
}

// WithSilentMode 设置是否开启静默模式。
// true-开启：鉴权失败，不注入用户信息；false-关闭。鉴权失败阻断，并抛出错误
func WithSilentMode(enabled bool) func(*Option) {
	return func(o *Option) {
		o.silentMode = enabled
	}
}

// WithAllowQueryStringToken 设置是否允许通过 URL Query String 传递 token。
// paths 限定允许的路径白名单，为空时表示允许所有路径（全局开放，会打印安全警告日志）。
func WithAllowQueryStringToken(enabled bool, paths ...string) func(*Option) {
	return func(o *Option) {
		o.allowQueryStringToken = enabled
		o.queryStringTokenPaths = paths
	}
}

// WithLeeway 为 JWT 时间比较留出容差。默认 0（无容差）。推荐分布式环境设置 30s。
func WithLeeway(d time.Duration) func(*Option) {
	return func(o *Option) {
		o.leeway = d
	}
}

// WithLegacySecrets 设置旧版验证密钥，用于密钥轮换场景。
// 解析 token 时先尝试 Secret，失败后依次尝试 LegacySecrets。
func WithLegacySecrets(secrets ...[]byte) func(*Option) {
	return func(o *Option) {
		o.legacySecrets = secrets
	}
}

// WithSigningMethods 设置允许的 JWT 签名算法，默认 ["HS256","HS384","HS512"]。
func WithSigningMethods(methods ...string) func(*Option) {
	return func(o *Option) {
		o.signingMethods = methods
	}
}

// getter 方法，供同模块内的其他包访问

func (o *Option) RoleConvert() httpcontext.ToRole     { return o.roleConvert }
func (o *Option) RefreshDuration() time.Duration      { return o.refreshDuration }
func (o *Option) Secret() []byte {
	if o.secret == nil {
		return nil
	}
	cp := make([]byte, len(o.secret))
	copy(cp, o.secret)
	return cp
}
func (o *Option) SilentMode() bool                    { return o.silentMode }
func (o *Option) AllowQueryStringToken() bool         { return o.allowQueryStringToken }
func (o *Option) QueryStringTokenPaths() []string     { return o.queryStringTokenPaths }
func (o *Option) Leeway() time.Duration               { return o.leeway }
func (o *Option) LegacySecrets() [][]byte {
	if o.legacySecrets == nil {
		return nil
	}
	cp := make([][]byte, len(o.legacySecrets))
	for i, s := range o.legacySecrets {
		cp[i] = make([]byte, len(s))
		copy(cp[i], s)
	}
	return cp
}
func (o *Option) SigningMethods() []string            { return o.signingMethods }
