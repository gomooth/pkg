package internal

import (
	"math"
	"time"

	"github.com/IBM/sarama"
	"github.com/pkg/errors"
	"github.com/save95/xlog"
)

type groupHandler struct {
	consumerGroup string

	retryCount    uint
	maxRetry      uint // 最大重试次数
	handler       func(topic string, msg []byte) error
	failedHandler func(consumerGroup, topic string, msg []byte, err error)

	logger xlog.XLogger
}

type groupHandlerConf struct {
	Logger xlog.XLogger

	Handler       func(topic string, msg []byte) error
	FailedHandler func(consumerGroup, topic string, msg []byte, err error)

	MaxRetry uint // 最大重试次数
}

func newConsumerGroupHandler(cg string, conf *groupHandlerConf) sarama.ConsumerGroupHandler {
	failedHandler := conf.FailedHandler

	return &groupHandler{
		consumerGroup: cg,
		handler:       conf.Handler,
		failedHandler: failedHandler,
		logger:        conf.Logger,
		maxRetry:      conf.MaxRetry,
	}
}

func (c *groupHandler) Setup(sarama.ConsumerGroupSession) error {
	//// 没有日志，则初始化一个默认日志处理
	//if nil == c.logger {
	//	l := console.NewLogger()
	//	l.SetLevel(int(xlog.DebugLevel))
	//	c.logger = l
	//	fmt.Println("not set logger, use default consoleLogger")
	//}

	return nil
}

func (c *groupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				c.logger.Debug("message channel was closed")
				return nil
			}

			c.logger.Debugf(
				"message claimed: cg=%q, topic=%q, time=%v, partition=%d, offset=%d",
				c.consumerGroup, msg.Topic, msg.Timestamp, msg.Partition, msg.Offset,
			)

			if err := c.handler(msg.Topic, msg.Value); err != nil {
				// 重试次数
				if c.maxRetry != 0 {
					if c.retryCount <= c.maxRetry {
						time.Sleep(time.Duration(10*math.Pow(2, float64(c.retryCount))) * time.Second)
						c.retryCount++
						return errors.Wrapf(err, "event handle failed, wait to retry(%d)", c.retryCount)
					}
				}

				// 未定义失败处理函数，直接抛出错误，阻塞
				if c.failedHandler == nil {
					return errors.Wrapf(err, "event handle failed，data: %s", msg.Value)
				}

				c.failedHandler(c.consumerGroup, msg.Topic, msg.Value, err)
				c.retryCount = 0
			}

			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			// Should return when `session.Context()` is done.
			// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
			// https://github.com/IBM/sarama/issues/1192
			return nil
		}
	}
}
