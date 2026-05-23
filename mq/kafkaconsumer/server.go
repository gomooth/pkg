package kafkaconsumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/app"
	"github.com/gomooth/pkg/framework/metrics"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/kafkaconsumer/internal"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/xerror"
	"go.opentelemetry.io/otel/metric"
)

var kafkaMeter = metrics.GetProvider().Meter("kafka")

var (
	kafkaConsumeCounter    = kafkaMeter.Int64Counter("kafka.consume.count")
	kafkaRetryCounter      = kafkaMeter.Int64Counter("kafka.retry.count")
	kafkaDeadLetterCounter = kafkaMeter.Int64Counter("kafka.dead_letter.count")
)

// recordKafkaConsume 记录消息消费成功
func recordKafkaConsume(ctx context.Context, topic string) {
	kafkaConsumeCounter.Add(ctx, 1, metric.WithAttributes(metrics.Attr("topic", topic)))
}

// recordKafkaRetry 记录消息重试
func recordKafkaRetry(ctx context.Context, topic string) {
	kafkaRetryCounter.Add(ctx, 1, metric.WithAttributes(metrics.Attr("topic", topic)))
}

// recordKafkaDeadLetter 记录消息进入死信队列
func recordKafkaDeadLetter(ctx context.Context, topic string) {
	kafkaDeadLetterCounter.Add(ctx, 1, metric.WithAttributes(metrics.Attr("topic", topic)))
}

type server struct {
	logger *slog.Logger

	addrs     []string
	consumers []IConsumer

	panicHandler  func(any)
	failedHandler func(ctx context.Context, group, topic string, msg []byte, err error)

	maxRetry                uint
	useDefaultFailedHandler bool
	backoff                 retry.BackoffStrategy

	// 新增：重试模式配置
	retryMode         internal.RetryMode
	retryWorkers      int
	retryMaxQueueSize int
	redisStore        internal.RedisRetryStore

	syncRetryMaxTotalTimeout time.Duration
	handlerTimeout           time.Duration

	consumer internal.IConsumer
}

func NewServer(addrs []string, opts ...func(*server)) IConsumeServer {
	svr := &server{
		addrs:     addrs,
		consumers: make([]IConsumer, 0),
		logger:    logger.NewConsoleLogger(),
		maxRetry:  0, // 默认不重试，由 Kafka 至少一次语义保证重新投递
	}

	for _, opt := range opts {
		opt(svr)
	}

	return svr
}

func (s *server) Register(group string, handler IHandler, topic string, topics ...string) {
	topics = append([]string{topic}, topics...)

	var dlHandler func(context.Context, string, []byte, error) error
	if dlh, ok := handler.(DeadLetterHandler); ok {
		dlHandler = dlh.OnDeadLetter
	}

	s.consumers = append(s.consumers, &consumer{
		group:             group,
		topics:            topics,
		handler:           handler,
		deadLetterHandler: dlHandler,
	})
}

func (s *server) Count() uint {
	return uint(len(s.consumers))
}

func (s *server) Start(ctx context.Context) error {
	if s.consumers == nil || len(s.consumers) == 0 {
		return xerror.New("no register consumer")
	}

	if s.logger == nil {
		s.logger = logger.NewConsoleLogger()
	}
	if s.failedHandler == nil && s.useDefaultFailedHandler {
		s.failedHandler = newDefaultFailedHandler(s.logger).Print
	}

	s.consumer = internal.NewDefaultConsumer(s.addrs)
	s.consumer.SetLogger(s.logger)
	s.consumer.SetMaxRetry(s.maxRetry)
	s.consumer.SetBackoff(s.backoff)
	s.consumer.SetPanicHandler(s.panicHandler)
	s.consumer.SetFailedHandler(s.failedHandler)
	s.consumer.SetRetryMode(s.retryMode)
	s.consumer.SetRetryWorkers(s.retryWorkers)
	s.consumer.SetRetryMaxQueueSize(s.retryMaxQueueSize)
	s.consumer.SetSyncRetryMaxTotalTimeout(s.syncRetryMaxTotalTimeout)
	s.consumer.SetHandlerTimeout(s.handlerTimeout)
	if s.redisStore != nil {
		s.consumer.SetRetryRedisStore(s.redisStore)
	}
	s.consumer.SetMetricsCallbacks(internal.MetricsCallbacks{
		OnConsume:    recordKafkaConsume,
		OnRetry:      recordKafkaRetry,
		OnDeadLetter: recordKafkaDeadLetter,
	})

	for _, item := range s.consumers {
		if err := s.consumer.RegisterHandler(item.Group(), item.Topics(), item.Handler(), item.DeadLetterHandler()); err != nil {
			return err
		}
	}

	s.consumer.Run(ctx)
	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	if s.consumer == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- s.consumer.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		s.logger.Warn("kafka consumer shutdown timed out, context cancelled", slog.String("component", "kafkaconsumer"), slog.String("error", ctx.Err().Error()))
		return ctx.Err()
	}
}

// HealthCheck 实现 app.HealthChecker 接口
func (s *server) HealthCheck(_ context.Context) error {
	if s.consumer == nil {
		return xerror.New("kafkaconsumer: consumer not initialized")
	}
	if !s.consumer.IsRunning() {
		return xerror.New("kafkaconsumer: consumer is not running")
	}
	return nil
}

var _ app.HealthChecker = (*server)(nil)
