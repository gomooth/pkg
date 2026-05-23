package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

type healthyApp struct{}

func (h *healthyApp) Start(_ context.Context) error       { return nil }
func (h *healthyApp) Shutdown(_ context.Context) error    { return nil }
func (h *healthyApp) HealthCheck(_ context.Context) error { return nil }

type unhealthyApp struct{}

func (u *unhealthyApp) Start(_ context.Context) error       { return nil }
func (u *unhealthyApp) Shutdown(_ context.Context) error    { return nil }
func (u *unhealthyApp) HealthCheck(_ context.Context) error { return errors.New("unhealthy") }

type noHealthApp struct{}

func (n *noHealthApp) Start(_ context.Context) error    { return nil }
func (n *noHealthApp) Shutdown(_ context.Context) error { return nil }

func TestManager_IsHealthy_AllHealthy(t *testing.T) {
	m := NewManager()
	m.Register(&healthyApp{})
	m.Register(&healthyApp{})

	if err := m.IsHealthy(context.Background()); err != nil {
		t.Errorf("expected healthy, got: %v", err)
	}
}

func TestManager_IsHealthy_Unhealthy(t *testing.T) {
	m := NewManager()
	m.Register(&healthyApp{})
	m.Register(&unhealthyApp{})

	err := m.IsHealthy(context.Background())
	if err == nil {
		t.Error("expected unhealthy error, got nil")
	}
}

func TestManager_IsHealthy_NoHealthChecker(t *testing.T) {
	m := NewManager()
	m.Register(&noHealthApp{})

	if err := m.IsHealthy(context.Background()); err != nil {
		t.Errorf("apps without HealthChecker should be considered healthy, got: %v", err)
	}
}

func TestManager_IsHealthy_Mixed(t *testing.T) {
	m := NewManager()
	m.Register(&noHealthApp{})
	m.Register(&unhealthyApp{})
	m.Register(&healthyApp{})

	err := m.IsHealthy(context.Background())
	if err == nil {
		t.Error("expected unhealthy error from unhealthyApp, got nil")
	}
}

type slowStartApp struct {
	startDur time.Duration
}

func (s *slowStartApp) Start(ctx context.Context) error {
	select {
	case <-time.After(s.startDur):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *slowStartApp) Shutdown(_ context.Context) error { return nil }

func TestManager_StartupTimeout(t *testing.T) {
	m := NewManager(WithStartupTimeout(50 * time.Millisecond))
	m.Register(&slowStartApp{startDur: 200 * time.Millisecond})

	err := m.Run(context.Background())
	if err == nil {
		t.Error("expected startup timeout error, got nil")
	}
}

func TestWithStartupTimeout(t *testing.T) {
	m := NewManager(WithStartupTimeout(10 * time.Second))
	mgr := m.(*manager)
	if mgr.startupTimeout != 10*time.Second {
		t.Errorf("expected 10s startup timeout, got %v", mgr.startupTimeout)
	}
}

func TestWithStartupTimeout_Zero(t *testing.T) {
	m := NewManager(WithStartupTimeout(0))
	mgr := m.(*manager)
	if mgr.startupTimeout != 0 {
		t.Errorf("expected 0 startup timeout, got %v", mgr.startupTimeout)
	}
}
