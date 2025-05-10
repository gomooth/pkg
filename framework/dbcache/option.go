package dbcache

import "time"

type option struct {
	autoRenew  bool // 自动延长缓存有效期
	expiration time.Duration
}

// WithAutoRenew 开启自动续期
func WithAutoRenew(autoRenew bool) func(*option) {
	return func(s *option) {
		s.autoRenew = autoRenew
	}
}

// WithExpiration 设置缓存时间，默认5分钟
func WithExpiration(expiration time.Duration) func(*option) {
	return func(s *option) {
		if expiration == 0 {
			expiration = 5 * time.Minute
		}
		s.expiration = expiration
	}
}
