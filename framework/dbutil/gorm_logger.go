package dbutil

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm/logger"
)

type dbWriter struct {
	log *slog.Logger
}

func (l *dbWriter) Printf(s string, vs ...interface{}) {
	l.log.Info(fmt.Sprintf(s, vs...))
}

func newWriter(l *slog.Logger) *dbWriter {
	return &dbWriter{log: l}
}

func newLogger(l *slog.Logger) logger.Interface {
	return logger.New(
		newWriter(l),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,       // Disable color
		},
	)
}

func newLoggerWith(l *slog.Logger, conf *logger.Config) logger.Interface {
	return logger.New(newWriter(l), *conf)
}
