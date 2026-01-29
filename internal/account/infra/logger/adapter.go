package logger

import (
	"ThreeKingdoms/modules/kit/logx"

	"go.uber.org/zap"
)

// ZapLoggerAdapter 是历史命名：实际复用 kit 的 zap 适配器，便于其他业务直接套用。
type ZapLoggerAdapter = logx.ZapLogger

func NewZapLoggerAdapter(l *zap.Logger) *ZapLoggerAdapter {
	return logx.NewZapLogger(l)
}
