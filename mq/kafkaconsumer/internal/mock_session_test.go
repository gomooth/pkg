package internal

import (
	"context"

	"github.com/IBM/sarama"
)

type mockSession struct {
	ctx context.Context
}

func (m *mockSession) Context() context.Context                                 { return m.ctx }
func (m *mockSession) Claims() map[string][]int32                               { return nil }
func (m *mockSession) MemberID() string                                         { return "" }
func (m *mockSession) GenerationID() int32                                      { return 0 }
func (m *mockSession) MarkOffset(_ string, _ int32, _ int64, _ string)          {}
func (m *mockSession) Commit()                                                  {}
func (m *mockSession) MarkMessage(_ *sarama.ConsumerMessage, _ string)          {}
func (m *mockSession) ResetOffset(_ string, _ int32, _ int64, _ string)         {}
func (m *mockSession) MarkPartitionOffset(_ string, _ int32, _ int64, _ string) {}
