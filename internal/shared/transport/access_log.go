package transport

import (
	"ThreeKingdoms/modules/kit/logx"
	"ThreeKingdoms/modules/kit/tracex"
	"context"
	"time"

	"go.uber.org/zap"
)

// AccessLog 是请求级日志上下文，覆盖 WS/HTTP 两种协议。
type AccessLog struct {
	BizCode     BizCode
	ErrorReason string
	startTime   time.Time
	action      string
}

type accessLogKey struct{}

// NewContext 创建带 AccessLog 的新 context（以 background 为父 context）。
func NewContext(action string) context.Context {
	return NewContextWithParent(context.Background(), action)
}

// NewContextWithParent 创建带 AccessLog 的新 context（保留父 context 的取消/超时信号）。
func NewContextWithParent(parent context.Context, action string) context.Context {
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	if action == "" {
		action = "unknown"
	}
	if traceID := tracex.NewTraceID(); traceID != "" {
		ctx = tracex.WithTraceID(ctx, traceID)
	}
	ctx = tracex.WithSpanID(ctx, "gate")

	al := &AccessLog{
		BizCode:   BizCode(SystemError),
		startTime: time.Now(),
		action:    action,
	}
	return context.WithValue(ctx, accessLogKey{}, al)
}

// FromContext 从 context 读取 AccessLog。
func FromContext(ctx context.Context) *AccessLog {
	if ctx == nil {
		return nil
	}
	al, _ := ctx.Value(accessLogKey{}).(*AccessLog)
	return al
}

// SetBizCode 设置业务码。
func SetBizCode(ctx context.Context, code BizCode) {
	if al := FromContext(ctx); al != nil {
		al.BizCode = code
	}
}

// SetErrorReason 设置 access 日志错误原因（失败场景）。
func SetErrorReason(ctx context.Context, reason string) {
	if reason == "" {
		return
	}
	if al := FromContext(ctx); al != nil {
		al.ErrorReason = reason
	}
}

// WriteAccessLog 输出访问日志（建议在中间件 defer 调用）。
func WriteAccessLog(ctx context.Context, log logx.Logger) {
	al := FromContext(ctx)
	if al == nil || log == nil {
		return
	}

	fields := []zap.Field{
		zap.Duration("latency", time.Since(al.startTime)),
	}
	if al.BizCode == BizCode(OK) {
		fields = append(fields, zap.String("result", "success"))
	} else {
		fields = append(fields, zap.String("result", "failure"))
		if al.ErrorReason != "" {
			fields = append(fields, zap.String("error_reason", al.ErrorReason))
		}
	}
	logx.ReportAccessWithLoggerContext(ctx, log, al.action, int(al.BizCode), fields...)
}
