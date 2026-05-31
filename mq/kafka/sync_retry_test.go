package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
)

type mockConsumerGroupSession struct {
	ctx   context.Context
	marks []*sarama.ConsumerMessage
}

func newMockSession() *mockConsumerGroupSession {
	return &mockConsumerGroupSession{ctx: context.Background()}
}
func (m *mockConsumerGroupSession) Claims() map[string][]int32               { return nil }
func (m *mockConsumerGroupSession) MemberID() string                         { return "test-member" }
func (m *mockConsumerGroupSession) GenerationID() int32                      { return 1 }
func (m *mockConsumerGroupSession) MarkOffset(string, int32, int64, string)  {}
func (m *mockConsumerGroupSession) Commit()                                  {}
func (m *mockConsumerGroupSession) ResetOffset(string, int32, int64, string) {}
func (m *mockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	m.marks = append(m.marks, msg)
}
func (m *mockConsumerGroupSession) Context() context.Context { return m.ctx }
func (m *mockConsumerGroupSession) Close()                   {}

func TestSyncRetry_Success(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	if len(session.marks) != 1 {
		t.Errorf("expected 1 marked message, got %d", len(session.marks))
	}
}

func TestSyncRetry_RetryThenSuccess(t *testing.T) {
	attempt := 0
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		attempt++
		if attempt < 3 {
			return errors.New("fail")
		}
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	if len(session.marks) != 1 {
		t.Errorf("expected 1 marked message after retry success, got %d", len(session.marks))
	}
}

func TestSyncRetry_Exhausted(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return errors.New("always fail")
	})
	strategy := newSyncRetryStrategy("test-group", handler, 2,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	// 重试耗尽且无 DeadLetterHandler，应标记消息（exhaustedHandled）
	if len(session.marks) != 1 {
		t.Errorf("expected 1 marked message after exhausted, got %d", len(session.marks))
	}
}
