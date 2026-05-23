package kafkaproducer

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/framework/metrics"
	"github.com/gomooth/pkg/framework/xcode"

	"github.com/gomooth/xerror"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel/metric"
)

var kafkaProducerMeter = metrics.GetProvider().Meter("kafka")

var (
	kafkaProduceCounter = kafkaProducerMeter.Int64Counter("kafka.produce.count")
	kafkaProduceErrors  = kafkaProducerMeter.Int64Counter("kafka.produce.errors")
)

func recordKafkaProduce(ctx context.Context, topic string) {
	kafkaProduceCounter.Add(ctx, 1, metric.WithAttributes(metrics.Attr("topic", topic)))
}

func recordKafkaProduceError(ctx context.Context, topic string) {
	kafkaProduceErrors.Add(ctx, 1, metric.WithAttributes(metrics.Attr("topic", topic)))
}

type producer struct {
	brokers []string
	timeout time.Duration

	logger *slog.Logger

	saramaConfig  *sarama.Config
	mu            sync.Mutex
	producer      sarama.SyncProducer // 使用同步生产者，优先保证消息可靠性；高吞吐量场景可改用 sarama.AsyncProducer
	producerReady bool
}

func New(brokers []string, opts ...func(*producer)) IProducer {
	p := &producer{
		brokers: brokers,
		timeout: time.Second * 5,
		logger:  logger.NewConsoleLogger(),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (b *producer) Produce(ctx context.Context, topic string, message []byte) error {
	return b.ProduceWithSequence(ctx, topic, "", message)
}

func (b *producer) Produces(ctx context.Context, topic string, message ...[]byte) error {
	return b.ProduceWithSequence(ctx, topic, "", message...)
}

// ProduceWithSequence 保持顺序性的生产消息
// sequenceKey 顺序性特定KEY。如 kafka 为 partition key
func (b *producer) ProduceWithSequence(ctx context.Context, topic, sequenceKey string, messages ...[]byte) error {
	if len(messages) == 0 {
		return xerror.NewXCode(xcode.ErrMQPublish, "no message")
	}

	msgs := make([]*sarama.ProducerMessage, 0, len(messages))
	for _, message := range messages {
		msgs = append(msgs, &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(sequenceKey),
			Value: sarama.ByteEncoder(message),
		})
	}

	return b.sends(ctx, msgs...)
}

func (b *producer) Close() error {
	if b.producer != nil {
		if err := b.producer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (b *producer) getSaramaConfig() *sarama.Config {
	if b.saramaConfig != nil {
		return b.saramaConfig
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	conf := sarama.NewConfig()

	// client common config
	conf.Version = sarama.V3_6_0_0
	conf.ClientID = hostname
	conf.Net.DialTimeout = b.timeout

	// producer config
	conf.Producer.RequiredAcks = sarama.WaitForAll
	conf.Producer.Return.Successes = true                  //接收 producer 的 success
	conf.Producer.Return.Errors = true                     // 接收 producer 的 error
	conf.Producer.Compression = sarama.CompressionZSTD     // 压缩方式，如果kafka版本大于1.2，推荐使用zstd压缩
	conf.Producer.Flush.Messages = 10                      // 缓存条数
	conf.Producer.Flush.Frequency = 500 * time.Millisecond // 缓存时间
	conf.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	conf.Producer.Transaction.Retry.Backoff = 10

	// 日志收集（仅首次设置，避免多实例覆盖）
	initSaramaLogger(b.logger)

	return conf
}

func (b *producer) getProducer() (sarama.SyncProducer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.producerReady && b.producer != nil {
		return b.producer, nil
	}

	p, err := sarama.NewSyncProducer(b.brokers, b.getSaramaConfig())
	if err != nil {
		b.logger.Error("producer create failed", slog.String("component", "kafkaproducer"), slog.Any("error", err))
		return nil, err
	}

	b.producer = p
	b.producerReady = true
	return b.producer, nil
}

func (b *producer) sends(ctx context.Context, msgs ...*sarama.ProducerMessage) error {
	// 检查 context 是否已取消，避免无效的发送尝试
	if err := ctx.Err(); err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	p, err := b.getProducer()
	if err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	err = p.SendMessages(msgs)
	if err != nil {
		for _, msg := range msgs {
			recordKafkaProduceError(ctx, msg.Topic)
		}
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	// 记录发送成功指标
	for _, msg := range msgs {
		recordKafkaProduce(ctx, msg.Topic)
	}

	return nil
}
