package kafkaproducer

import (
	"time"

	"github.com/save95/xlog"
)

// WithLogger 设置日志器
func WithLogger(logger xlog.XLogger) func(*producer) {
	return func(p *producer) {
		p.logger = logger
	}
}

//func WithBrokers(brokers []string) func(*producer) {
//	return func(p *producer) {
//		p.brokers = brokers
//	}
//}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) func(*producer) {
	return func(p *producer) {
		p.timeout = timeout
	}
}
