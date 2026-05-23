package kafkaproducer

import (
	"log/slog"
	"time"

	"github.com/IBM/sarama"
)

// WithLogger 设置日志器
func WithLogger(logger *slog.Logger) func(*producer) {
	return func(p *producer) {
		sarama.Logger = newSaramaLogger(logger)
		p.logger = logger
	}
}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) func(*producer) {
	return func(p *producer) {
		p.timeout = timeout
	}
}
