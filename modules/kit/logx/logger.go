package logx

import (
	"context"

	"go.uber.org/zap"
)

// Logger 是跨服务可复用的最小日志接口。
//
// 约束：
// - 保持 API 极简，避免“自研日志框架”过度设计
// - 只承载业务需要的能力：结构化字段 + ctx 透传（trace/span 等）
type Logger interface {
	Info(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	WithContext(ctx context.Context) Logger
}
