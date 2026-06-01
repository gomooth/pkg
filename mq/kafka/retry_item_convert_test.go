package kafka

import (
	"testing"
	"time"

	"github.com/gomooth/pkg/mq/kafka/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToInternalRetryItem(t *testing.T) {
	now := time.Now()
	publicItem := &RetryItem{
		Topic:         "test-topic",
		Partition:     1,
		Offset:        100,
		Key:           []byte("key1"),
		Value:         []byte("value1"),
		Headers:       []HeaderKV{{Key: "h1", Value: []byte("v1")}},
		Attempt:       3,
		NextRetryAt:   now,
		ConsumerGroup: "test-group",
	}

	internalItem := toInternalRetryItem(publicItem)
	assert.Equal(t, "test-topic", internalItem.Topic)
	assert.Equal(t, int32(1), internalItem.Partition)
	assert.Equal(t, int64(100), internalItem.Offset)
	assert.Equal(t, []byte("key1"), internalItem.Key)
	assert.Equal(t, []byte("value1"), internalItem.Value)
	assert.Equal(t, uint(3), internalItem.Attempt)
	assert.Equal(t, now, internalItem.NextRetryAt)
	assert.Equal(t, "test-group", internalItem.ConsumerGroup)
	require.Len(t, internalItem.Headers, 1)
	assert.Equal(t, []byte("h1"), internalItem.Headers[0].Key)
	assert.Equal(t, []byte("v1"), internalItem.Headers[0].Value)
}

func TestToInternalRetryItem_Nil(t *testing.T) {
	result := toInternalRetryItem(nil)
	assert.Nil(t, result)
}

func TestToInternalRetryItem_EmptyHeaders(t *testing.T) {
	publicItem := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    1,
	}
	internalItem := toInternalRetryItem(publicItem)
	assert.Empty(t, internalItem.Headers)
}

func TestToPublicRetryItem(t *testing.T) {
	now := time.Now()
	internalItem := &internal.RetryItem{
		Topic:         "test-topic",
		Partition:     2,
		Offset:        200,
		Key:           []byte("key2"),
		Value:         []byte("value2"),
		Headers:       []internal.HeaderKV{{Key: []byte("h2"), Value: []byte("v2")}},
		Attempt:       5,
		NextRetryAt:   now,
		ConsumerGroup: "my-group",
	}

	publicItem := toPublicRetryItem(internalItem)
	assert.Equal(t, "test-topic", publicItem.Topic)
	assert.Equal(t, int32(2), publicItem.Partition)
	assert.Equal(t, int64(200), publicItem.Offset)
	assert.Equal(t, []byte("key2"), publicItem.Key)
	assert.Equal(t, []byte("value2"), publicItem.Value)
	assert.Equal(t, 5, publicItem.Attempt)
	assert.Equal(t, now, publicItem.NextRetryAt)
	assert.Equal(t, "my-group", publicItem.ConsumerGroup)
	require.Len(t, publicItem.Headers, 1)
	assert.Equal(t, "h2", publicItem.Headers[0].Key)
	assert.Equal(t, []byte("v2"), publicItem.Headers[0].Value)
}

func TestToPublicRetryItem_Nil(t *testing.T) {
	result := toPublicRetryItem(nil)
	assert.Nil(t, result)
}

func TestToPublicRetryItems(t *testing.T) {
	items := []*internal.RetryItem{
		{Topic: "t1", Partition: 0, Offset: 1},
		{Topic: "t2", Partition: 1, Offset: 2},
	}
	result := toPublicRetryItems(items)
	assert.Len(t, result, 2)
	assert.Equal(t, "t1", result[0].Topic)
	assert.Equal(t, "t2", result[1].Topic)
}

func TestToPublicRetryItems_Empty(t *testing.T) {
	result := toPublicRetryItems([]*internal.RetryItem{})
	assert.Empty(t, result)
}

func TestToPublicRetryItems_NilHeaders(t *testing.T) {
	internalItem := &internal.RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    1,
	}
	publicItem := toPublicRetryItem(internalItem)
	assert.Empty(t, publicItem.Headers)
}
