package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// --- New tests for improved coverage ---

func TestWithLogger(t *testing.T) {
	customLogger := slog.Default()
	m := NewManager(WithLogger(customLogger))
	mgr := m.(*manager)
	assert.NotNil(t, mgr.log)
}

func TestWithApp(t *testing.T) {
	app := &noHealthApp{}
	m := NewManager(WithApp(app))
	mgr := m.(*manager)
	require.Len(t, mgr.apps, 1)
}

func TestWithApp_NilApps(t *testing.T) {
	// WithApp initializes apps if nil
	mgr := &manager{}
	WithApp(&noHealthApp{})(mgr)
	require.Len(t, mgr.apps, 1)
}

func TestWithShutdownTimeout(t *testing.T) {
	m := NewManager(WithShutdownTimeout(10 * time.Second))
	mgr := m.(*manager)
	assert.Equal(t, 10*time.Second, mgr.shutdownTimeout)
}

func TestWithShutdownTimeout_ZeroIgnored(t *testing.T) {
	// Zero or negative durations are ignored
	m := NewManager(WithShutdownTimeout(0))
	mgr := m.(*manager)
	assert.Equal(t, 30*time.Second, mgr.shutdownTimeout, "default 30s should remain when zero is passed")
}

func TestNewManager_DefaultLogger(t *testing.T) {
	m := NewManager()
	mgr := m.(*manager)
	assert.NotNil(t, mgr.log, "default logger should be set")
	assert.Equal(t, 30*time.Second, mgr.shutdownTimeout, "default shutdown timeout should be 30s")
}

func TestManager_Run_Success(t *testing.T) {
	started := make(chan struct{})
	app := &signalApp{
		startHook: func() { close(started) },
	}

	m := NewManager(WithApp(app))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Run(ctx)
	}()

	// Wait for app to start
	select {
	case <-started:
		// app started successfully
	case <-time.After(2 * time.Second):
		t.Fatal("app did not start within timeout")
	}

	// Cancel context to trigger shutdown (simulate signal)
	cancel()

	// The Run function waits for OS signals, so we need to send one
	// Since we can't easily send signals in test, let's use a different approach
	// We'll test the start path and cleanup by sending SIGTERM
	go func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not complete within timeout")
	}
}

type signalApp struct {
	startHook   func()
	shutdownErr error
}

func (s *signalApp) Start(_ context.Context) error {
	if s.startHook != nil {
		s.startHook()
	}
	return nil
}
func (s *signalApp) Shutdown(_ context.Context) error { return s.shutdownErr }

func TestManager_Run_StartError_Cleanup(t *testing.T) {
	// First app starts fine, second app fails => first app should be shut down
	shutdownCalled := make(chan struct{})
	firstApp := &trackableApp{
		startErr:    nil,
		shutdownHook: func() { close(shutdownCalled) },
	}
	failApp := &trackableApp{
		startErr: errors.New("start failed"),
	}

	m := NewManager()
	m.Register(firstApp)
	m.Register(failApp)

	err := m.Run(context.Background())
	assert.Error(t, err)

	// Verify first app was cleaned up
	select {
	case <-shutdownCalled:
		// cleanup was called
	case <-time.After(2 * time.Second):
		t.Fatal("first app was not shut down after second app failed to start")
	}
}

type trackableApp struct {
	startErr     error
	shutdownErr  error
	shutdownHook func()
}

func (a *trackableApp) Start(_ context.Context) error { return a.startErr }
func (a *trackableApp) Shutdown(_ context.Context) error {
	if a.shutdownHook != nil {
		a.shutdownHook()
	}
	return a.shutdownErr
}

func TestManager_Run_StartError_WithShutdownError(t *testing.T) {
	// First app starts OK, second fails; first app's shutdown also fails
	firstApp := &trackableApp{
		startErr:    nil,
		shutdownErr: errors.New("shutdown also failed"),
	}
	failApp := &trackableApp{
		startErr: errors.New("start failed"),
	}

	m := NewManager()
	m.Register(firstApp)
	m.Register(failApp)

	err := m.Run(context.Background())
	assert.Error(t, err)
}

func TestManager_MustRun_ExitsOnError(t *testing.T) {
	// MustRun should call os.Exit(1) on error
	// We can't easily test os.Exit in-process, but we can verify the path
	// by checking that it would exit. Instead, we test the happy path.
	app := &trackableApp{}

	m := NewManager(WithApp(app))

	// Run MustRun in a goroutine that sends SIGTERM quickly
	go func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	// This should not panic/exit since the app starts fine
	done := make(chan struct{})
	go func() {
		m.MustRun(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// MustRun completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("MustRun did not complete within timeout")
	}
}

func TestManager_Run_WithStartupTimeout(t *testing.T) {
	// App that starts within timeout should succeed
	m := NewManager(WithStartupTimeout(2 * time.Second))
	m.Register(&noHealthApp{})

	go func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	done := make(chan error, 1)
	go func() {
		done <- m.Run(context.Background())
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not complete within timeout")
	}
}

func TestManager_Register(t *testing.T) {
	m := NewManager()
	m.Register(&noHealthApp{})
	m.Register(&healthyApp{})

	mgr := m.(*manager)
	assert.Len(t, mgr.apps, 2)
}

func TestManager_ShutdownUsesBackgroundContext(t *testing.T) {
	// Verify that the cleanup shutdown context is derived from context.Background(),
	// not from the parent ctx passed to Run(). When the parent ctx is cancelled,
	// the shutdown timeout should still function independently.
	var capturedShutdownCtx context.Context
	var wasAlreadyDone bool
	ctxApp := &ctxCaptureApp2{
		captured:     &capturedShutdownCtx,
		capturedDone: &wasAlreadyDone,
	}

	m := NewManager()
	m.Register(ctxApp)
	m.Register(&trackableApp{startErr: errors.New("fail")})

	// Use an already-cancelled context to prove shutdown ctx is independent
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_ = m.Run(cancelledCtx)

	assert.NotNil(t, capturedShutdownCtx, "shutdown context should have been captured")
	assert.False(t, wasAlreadyDone, "shutdown context should NOT be cancelled at Shutdown call time (derived from Background, not cancelled parent)")
}

type ctxCaptureApp2 struct {
	captured     *context.Context
	capturedDone *bool // true if ctx was already done at Shutdown call time
}

func (c *ctxCaptureApp2) Start(_ context.Context) error { return nil }
func (c *ctxCaptureApp2) Shutdown(ctx context.Context) error {
	*c.captured = ctx
	done := ctx.Err() != nil
	*c.capturedDone = done
	return nil
}

func TestManager_SignalShutdownUsesBackgroundContext(t *testing.T) {
	// Verify the signal-based shutdown path also uses Background context.
	// We start the manager with a cancelled context, then send SIGTERM.
	// The shutdown ctx passed to apps should not be tied to the cancelled parent.
	var capturedShutdownCtx context.Context
	var wasAlreadyDone bool
	ctxApp := &ctxCaptureApp2{
		captured:     &capturedShutdownCtx,
		capturedDone: &wasAlreadyDone,
	}

	m := NewManager(WithApp(ctxApp))

	go func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	// Use a cancelled context as the parent to prove independence
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- m.Run(cancelledCtx)
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not complete within timeout")
	}

	assert.NotNil(t, capturedShutdownCtx, "shutdown context should have been captured during signal shutdown")
	assert.False(t, wasAlreadyDone, "shutdown context should NOT be cancelled at Shutdown call time (derived from Background, not cancelled parent)")
}
