package kafkaproducer

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/IBM/sarama"
)

var saramaLoggerOnce sync.Once

// initSaramaLogger 设置 sarama 全局日志器，仅首次调用时生效，避免多实例覆盖
func initSaramaLogger(l *slog.Logger) {
	saramaLoggerOnce.Do(func() {
		sarama.Logger = newSaramaLogger(l)
	})
}

type saramaLogger struct {
	logger *slog.Logger
}

func newSaramaLogger(logger *slog.Logger) *saramaLogger {
	return &saramaLogger{
		logger: logger,
	}
}

func (fb *saramaLogger) Println(v ...interface{}) {
	fb.logger.Debug(fmt.Sprintf("[sarama] %v", v...), slog.String("component", "kafkaproducer"))
}

func (fb *saramaLogger) Printf(format string, v ...interface{}) {
	fb.logger.Debug(fmt.Sprintf("[sarama] "+format, v...), slog.String("component", "kafkaproducer"))
}

func (fb *saramaLogger) Print(v ...interface{}) {
	fb.logger.Debug(fmt.Sprintf("[sarama] %v", v...), slog.String("component", "kafkaproducer"))
}
