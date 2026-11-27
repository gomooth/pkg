package dbutil

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/logger"
)

// captured 记录最近一次到达 slog handler 的 ctx、级别与消息
type captured struct {
	ctx   context.Context
	level slog.Level
	msg   string
}

// ctxCaptureHandler 包装 slog.NewTextHandler，Handle 时捕获 ctx、级别与消息
type ctxCaptureHandler struct {
	next slog.Handler
	got  *captured
}

func (h *ctxCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *ctxCaptureHandler) Handle(ctx context.Context, r slog.Record) error {
	h.got.ctx = ctx
	h.got.level = r.Level
	h.got.msg = r.Message
	return h.next.Handle(ctx, r)
}

func (h *ctxCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *ctxCaptureHandler) WithGroup(_ string) slog.Handler { return h }

// captureLogger 构造内存 slog.Logger，Handle 里记录 ctx
func captureLogger(got *captured) *slog.Logger {
	return slog.New(&ctxCaptureHandler{
		next: slog.NewTextHandler(&bytes.Buffer{}, nil),
		got:  got,
	})
}

// TestGormLoggerTracePassesContext 断言 Trace 的 ctx 原样到达 slog handler，
// 覆盖 gorm 级别到 slog 级别的映射（慢 SQL→Warn、错误→Error、其余→Info），
// 并断言消息文本格式与调用方 file:line 归因（不得指向桥接层自身）
func TestGormLoggerTracePassesContext(t *testing.T) {
	type spanKey struct{}
	ctx := context.WithValue(context.Background(), spanKey{}, "v")

	tests := []struct {
		name      string
		elapsed   time.Duration
		err       error
		wantLevel slog.Level
		wantText  []string
	}{
		{"slow sql maps to warn", 2 * time.Second, nil, slog.LevelWarn,
			[]string{"SLOW SQL >= 1s", "[rows:1]", "SELECT 1"}},
		{"error maps to error", time.Millisecond, errors.New("boom"), slog.LevelError,
			[]string{"boom", "[rows:1]", "SELECT 1"}},
		{"ignored record not found falls to info", time.Millisecond, logger.ErrRecordNotFound, slog.LevelInfo,
			[]string{"[rows:1]", "SELECT 1"}},
		{"plain sql maps to info", time.Millisecond, nil, slog.LevelInfo,
			[]string{"[rows:1]", "SELECT 1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &captured{ctx: context.Background()}
			gl := newLogger(captureLogger(got))
			gl.Trace(ctx, time.Now().Add(-tt.elapsed), func() (string, int64) {
				return "SELECT 1", 1
			}, tt.err)
			assert.Equal(t, "v", got.ctx.Value(spanKey{}), "Trace 的 ctx 必须传到 slog handler")
			assert.Equal(t, tt.wantLevel, got.level, "Trace 的 gorm 级别应映射到对应 slog 级别")
			assert.Regexp(t, `\[\d+\.\d{3}ms\] \[rows:1\] SELECT 1$`, got.msg, "消息应保留现状格式 [%.3fms] [rows:%v] sql")
			for _, want := range tt.wantText {
				assert.Contains(t, got.msg, want)
			}
			// 调用方归因：消息前缀必须是业务调用方的 file:line，不能是桥接层自身
			prefix, _, ok := strings.Cut(got.msg, "\n[")
			assert.True(t, ok, "消息应以调用方 file:line 前缀开头")
			assert.Regexp(t, `\.go:\d+`, prefix, "前缀应为 file:line 形态")
			assert.NotContains(t, prefix, "gorm_logger.go", "file:line 必须指向业务调用方，而非桥接层自身")
		})
	}
}

// TestGormLoggerInfoErrorKeepContext 断言 Info/Warn/Error 的 ctx 原样到达 slog handler
func TestGormLoggerInfoErrorKeepContext(t *testing.T) {
	type spanKey struct{}
	ctx := context.WithValue(context.Background(), spanKey{}, "v")

	tests := []struct {
		name      string
		call      func(l logger.Interface)
		wantLevel slog.Level
		wantMsg   string
	}{
		{"info", func(l logger.Interface) { l.Info(ctx, "info message") }, slog.LevelInfo, "info message"},
		{"warn", func(l logger.Interface) { l.Warn(ctx, "warn message") }, slog.LevelWarn, "warn message"},
		{"error", func(l logger.Interface) { l.Error(ctx, "error message") }, slog.LevelError, "error message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &captured{ctx: context.Background()}
			gl := newLogger(captureLogger(got))
			tt.call(gl)
			assert.Equal(t, "v", got.ctx.Value(spanKey{}), "%s 的 ctx 必须传到 slog handler", tt.name)
			assert.Equal(t, tt.wantLevel, got.level, "%s 应映射到对应 slog 级别", tt.name)
			assert.Equal(t, tt.wantMsg, got.msg, "%s 消息应为 fmt.Sprintf(msg, data...) 形态", tt.name)
		})
	}
}
