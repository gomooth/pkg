package httpsqsconsumer

import "github.com/save95/xlog"

// WithLogger 设置日志器
func WithLogger(log xlog.XLogger) func(*consumer) {
	return func(s *consumer) {
		s.log = log
	}
}

// WithHandler 设置消费者处理器
func WithHandler(handler IHandler) func(*consumer) {
	return func(s *consumer) {
		s.handler = handler
	}
}

// WithMaxRetry 设置重试次数
func WithMaxRetry(count uint) func(*consumer) {
	return func(s *consumer) {
		s.maxRetry = count
	}
}
