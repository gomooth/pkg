package dbutil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

// slogGormLogger 直接实现 gorm logger.Interface 的 slog 桥接：
// 分级规则与输出文本沿用 gorm 默认 logger（非彩色模式），但所有方法都把
// ctx 原样传给 slog 的 *Context 方法，使 ctx 直达 slog handler
// （后续 trace_id 注入Injector 的生效前提）。Colorful 配置项被忽略，
// 与 gorm 官方 slog 桥接（logger.NewSlogLogger）的行为一致。
type slogGormLogger struct {
	log  *slog.Logger
	conf logger.Config
}

var _ logger.Interface = (*slogGormLogger)(nil)

// LogMode 返回调整日志级别后的副本，语义同 gorm 默认 logger
func (l *slogGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.conf.LogLevel = level
	return &newlogger
}

func (l *slogGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.conf.LogLevel >= logger.Info {
		l.log.InfoContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *slogGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.conf.LogLevel >= logger.Warn {
		l.log.WarnContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *slogGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.conf.LogLevel >= logger.Error {
		l.log.ErrorContext(ctx, fmt.Sprintf(msg, data...))
	}
}

// Trace 打印 SQL 日志：错误（遵循 IgnoreRecordNotFoundError）走 Error、
// 慢 SQL 走 Warn、其余走 Info，文本格式与 gorm 默认 logger 一致
func (l *slogGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.conf.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.conf.LogLevel >= logger.Error && (!errors.Is(err, logger.ErrRecordNotFound) || !l.conf.IgnoreRecordNotFoundError):
		sql, rows := fc()
		l.log.ErrorContext(ctx, fmt.Sprintf("%s %s\n[%.3fms] [rows:%v] %s",
			fileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rowsField(rows), sql))
	case elapsed > l.conf.SlowThreshold && l.conf.SlowThreshold != 0 && l.conf.LogLevel >= logger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.conf.SlowThreshold)
		l.log.WarnContext(ctx, fmt.Sprintf("%s %s\n[%.3fms] [rows:%v] %s",
			fileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rowsField(rows), sql))
	case l.conf.LogLevel == logger.Info:
		sql, rows := fc()
		l.log.InfoContext(ctx, fmt.Sprintf("%s\n[%.3fms] [rows:%v] %s",
			fileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rowsField(rows), sql))
	}
}

// rowsField 与 gorm 默认 logger 一致：rows 为 -1（未知）时打印 "-"
func rowsField(rows int64) interface{} {
	if rows == -1 {
		return "-"
	}
	return rows
}

// bridgeFile 本文件路径，用于在栈扫描中跳过桥接层自身帧
var bridgeFile = func() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}()

// fileWithLineNum 返回 SQL 日志业务调用方的 file:line。
// 不能直接用 gorm 的 utils.FileWithLineNum：从桥接层调用时其栈扫描
// 只过滤 gorm 模块路径的帧，会把桥接自身帧当作调用方返回。这里自行
// 扫描运行时栈，跳过 gorm 模块帧（gorm.io/gorm*）与本文件帧，返回
// 第一个其余帧——即业务调用方；.gen.go 生成文件的帧同样跳过，
// 与 gorm 默认 logger 的过滤规则保持一致。
func fileWithLineNum() string {
	// gorm 默认 logger 从 Trace 调用者起扫描 13 帧；本函数从自身起扫描，
	// 多经过 2 个桥接帧，窗口取 15 以覆盖同样的深度
	pcs := make([]uintptr, 15)
	n := runtime.Callers(1, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for i := 0; i < n; i++ {
		frame, _ := frames.Next()
		if frame.File == "" || frame.File == bridgeFile ||
			strings.HasSuffix(frame.File, ".gen.go") || isGormFrame(frame.Function) {
			continue
		}
		return frame.File + ":" + strconv.Itoa(frame.Line)
	}
	return ""
}

// isGormFrame 判断栈帧函数是否位于 gorm 核心模块（gorm.io/gorm 及其子包）
func isGormFrame(fn string) bool {
	return strings.HasPrefix(fn, "gorm.io/gorm/") || strings.HasPrefix(fn, "gorm.io/gorm.")
}

// ParamsFilter 保持与 gorm 默认 logger 一致的 Config 语义：
// ParameterizedQueries 开启时从日志中剥离绑定参数
func (l *slogGormLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.conf.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

func newLogger(l *slog.Logger) logger.Interface {
	return &slogGormLogger{
		log: l,
		conf: logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,       // Disable color
		},
	}
}

func newLoggerWith(l *slog.Logger, conf *logger.Config) logger.Interface {
	return &slogGormLogger{log: l, conf: *conf}
}
