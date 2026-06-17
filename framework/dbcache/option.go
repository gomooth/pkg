package dbcache

import "time"

// traceConfig OTel Span 配置
type traceConfig struct {
	methodSpan bool // 方法级 Span，默认 true
	buildSpan  bool // 构建级 Span，默认 false
}

type dbCacheOption struct {
	autoRenew      bool // 自动延长缓存有效期
	expiration     time.Duration
	renewThreshold float64       // 续期阈值比例，默认 0.2（剩余 20% TTL 时续期）
	codec          Codec         // 序列化编解码器，默认 JSON
	errorCacheTTL  time.Duration // 错误结果缓存时间，0 表示不缓存错误
	traceConfig    *traceConfig  // OTel Span 配置
}

// WithAutoRenew 开启自动续期
func WithAutoRenew(autoRenew bool) func(*dbCacheOption) {
	return func(s *dbCacheOption) {
		s.autoRenew = autoRenew
	}
}

// WithExpiration 设置缓存时间，默认5分钟
func WithExpiration(expiration time.Duration) func(*dbCacheOption) {
	return func(s *dbCacheOption) {
		if expiration == 0 {
			expiration = 5 * time.Minute
		}
		s.expiration = expiration
	}
}

// WithRenewThreshold 设置续期阈值比例。
// ratio 范围 (0, 1]，当缓存剩余 TTL <= expiration * ratio 时触发续期。
// 默认 0.2，即剩余 20% 有效期时续期。
func WithRenewThreshold(ratio float64) func(*dbCacheOption) {
	return func(s *dbCacheOption) {
		if ratio <= 0 {
			ratio = 0.2
		}
		if ratio > 1 {
			ratio = 1
		}
		s.renewThreshold = ratio
	}
}

// WithCodec 设置缓存序列化编解码器。
// 默认使用 JSON 编解码器，可替换为 msgpack 或 gob 等更高效的实现。
func WithCodec(c Codec) func(*dbCacheOption) {
	return func(s *dbCacheOption) {
		if c != nil {
			s.codec = c
		}
	}
}

// WithErrorCacheTTL 设置错误结果的短暂缓存时间。
// 当 query() 返回错误时，缓存一个占位值以防止错误风暴。
// 设为 0 禁用此功能（默认）。推荐值：30s。
func WithErrorCacheTTL(ttl time.Duration) func(*dbCacheOption) {
	return func(s *dbCacheOption) {
		s.errorCacheTTL = ttl
	}
}

// WithTraceMethodSpan 开启方法级 OTel Span（默认已开启，此选项用于显式控制）
func WithTraceMethodSpan() func(*dbCacheOption) {
	return func(o *dbCacheOption) {
		if o.traceConfig == nil {
			o.traceConfig = &traceConfig{methodSpan: true, buildSpan: false}
		}
		o.traceConfig.methodSpan = true
	}
}

// WithTraceBuildSpan 开启构建级 OTel Span（隐含同时开启方法级 Span）
func WithTraceBuildSpan() func(*dbCacheOption) {
	return func(o *dbCacheOption) {
		if o.traceConfig == nil {
			// 构建级 Span 隐含需要方法级 Span 作为父级，故 methodSpan 也设为 true
			o.traceConfig = &traceConfig{methodSpan: true, buildSpan: true}
		}
		o.traceConfig.buildSpan = true
	}
}
