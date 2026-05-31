package internal

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/IBM/sarama"
)

var saramaLoggerOnce sync.Once

// InitSaramaLogger 初始化 sarama 全局日志器（仅首次调用生效）。
// Consumer 和 Producer 的构造函数都应调用此函数。
func InitSaramaLogger(l *slog.Logger) {
	saramaLoggerOnce.Do(func() {
		sarama.Logger = &saramaLogAdapter{l: l}
	})
}

// saramaLogAdapter 适配 slog.Logger 到 sarama.StdLogger 接口
type saramaLogAdapter struct {
	l *slog.Logger
}

func (a *saramaLogAdapter) Print(v ...interface{}) {
	a.l.Debug(fmt.Sprintf("[sarama] %v", v...), "component", "kafka")
}

func (a *saramaLogAdapter) Printf(format string, v ...interface{}) {
	a.l.Debug(fmt.Sprintf("[sarama] "+format, v...), "component", "kafka")
}

func (a *saramaLogAdapter) Println(v ...interface{}) {
	a.l.Debug(fmt.Sprintf("[sarama] %v", v...), "component", "kafka")
}
