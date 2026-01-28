package logs

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	glogger "gorm.io/gorm/logger"
)

type GormLogger struct {
	level         glogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger：把 logs 接到 GORM
func NewGormLogger(level glogger.LogLevel, slowThreshold time.Duration) glogger.Interface {
	return &GormLogger{
		level:         level,
		slowThreshold: slowThreshold,
	}
}

func (l *GormLogger) LogMode(level glogger.LogLevel) glogger.Interface {
	l.level = level
	return l
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.level >= glogger.Info {
		Info("gorm: "+msg, zap.Any("data", data))
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.level >= glogger.Warn {
		Warn("gorm: "+msg, zap.Any("data", data))
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.level >= glogger.Error {
		Error("gorm: "+msg, zap.Any("data", data))
	}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= glogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// 你也可以在 ctx 里取 traceID/requestID，打到字段里
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && !errors.Is(err, glogger.ErrRecordNotFound):
		// SQL 执行出错
		fields = append(fields, zap.Error(err))
		Error("gorm trace error", fields...)

	case l.slowThreshold > 0 && elapsed > l.slowThreshold:
		// 慢查询
		Warn("gorm slow query", fields...)

	default:
		// 正常 trace（按需：有些人生产不打这个）
		if l.level >= glogger.Info {
			Debug("gorm trace", fields...)
		}
	}
}
