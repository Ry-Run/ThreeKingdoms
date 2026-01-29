package logx

import (
	"context"

	"ThreeKingdoms/modules/kit/tracex"

	"go.uber.org/zap"
)

// ZapLogger 是 zap 的适配器，实现 logx.Logger，便于各服务复用。
type ZapLogger struct {
	logger *zap.Logger
}

func NewZapLogger(l *zap.Logger) *ZapLogger {
	if l == nil {
		return &ZapLogger{logger: zap.NewNop()}
	}
	return &ZapLogger{logger: l}
}

func (z *ZapLogger) WithContext(ctx context.Context) Logger {
	if z == nil {
		return NewZapLogger(nil)
	}
	l := z.logger
	if tid, ok := tracex.TraceIDFrom(ctx); ok {
		l = l.With(zap.String("trace_id", tid))
	}
	if sid, ok := tracex.SpanIDFrom(ctx); ok {
		l = l.With(zap.String("span_id", sid))
	}
	return &ZapLogger{logger: l}
}

func (z *ZapLogger) Info(msg string, fields ...zap.Field) {
	z.logger.Info(msg, fields...)
}

func (z *ZapLogger) Error(msg string, fields ...zap.Field) {
	z.logger.Error(msg, fields...)
}

func (z *ZapLogger) Debug(msg string, fields ...zap.Field) {
	z.logger.Debug(msg, fields...)
}

func (z *ZapLogger) Warn(msg string, fields ...zap.Field) {
	z.logger.Warn(msg, fields...)
}
