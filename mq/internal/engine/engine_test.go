package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBase_HealthCheck(t *testing.T) {
	b := &Base{}
	b.State.Store(Running)
	assert.NoError(t, b.HealthCheck(context.Background()))

	b.State.Store(Idle)
	assert.Error(t, b.HealthCheck(context.Background()))
}

func TestBase_TryStart(t *testing.T) {
	b := &Base{}
	assert.True(t, b.TryStart())
	assert.Equal(t, int32(Running), b.State.Load())
	assert.False(t, b.TryStart()) // already running
}

func TestBase_RequestShutdown(t *testing.T) {
	b := &Base{}
	b.State.Store(Running)
	assert.True(t, b.RequestShutdown())
	assert.Equal(t, int32(ShuttingDown), b.State.Load())

	b2 := &Base{}
	assert.False(t, b2.RequestShutdown()) // not running
}

func TestBase_SafeGo_PanicRecovery(t *testing.T) {
	b := &Base{}
	var panicked atomic.Bool
	b.SafeGo("test", func() {
		panic("boom")
	}, func(r any) {
		panicked.Store(true)
	})
	assert.Eventually(t, func() bool { return panicked.Load() }, time.Second, 10*time.Millisecond)
}

func TestBase_SafeGo_NoPanic(t *testing.T) {
	b := &Base{}
	var executed atomic.Bool
	b.SafeGo("test", func() {
		executed.Store(true)
	}, nil)
	assert.Eventually(t, func() bool { return executed.Load() }, time.Second, 10*time.Millisecond)
}
