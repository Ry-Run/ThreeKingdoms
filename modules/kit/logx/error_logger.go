package logx

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// BizLog 是业务拒绝日志的强类型输入，避免参数顺序误传。
type BizLog struct {
	Action  string
	Reason  string
	Message string
}

// SysLog 是技术错误日志的强类型输入，避免参数顺序误传。
type SysLog struct {
	Action string
	Err    error
}

func NewBizLog(action, reason, message string) BizLog {
	return BizLog{
		Action:  action,
		Reason:  reason,
		Message: message,
	}
}

func NewSysLog(action string, err error) SysLog {
	return SysLog{
		Action: action,
		Err:    err,
	}
}

// ReportAccessWithLoggerContext 记录访问日志：
// - biz_code == 0: INFO
// - biz_code  1~499: WARN
// - biz_code >= 500: ERROR
func ReportAccessWithLoggerContext(ctx context.Context, l Logger, action string, bizCode int, fields ...zap.Field) {
	if l == nil {
		return
	}
	base := []zap.Field{
		zap.String("log_type", "access"),
		zap.String("action", action),
		zap.Int("biz_code", bizCode),
	}
	base = append(base, fields...)
	withCtx := l.WithContext(ctx)
	switch {
	case bizCode == 0:
		withCtx.Info("access", base...)
	case bizCode >= 500:
		withCtx.Error("access", base...)
	default:
		withCtx.Warn("access", base...)
	}
}

// ReportBizWithLoggerContext 记录业务拒绝日志：INFO、err_type=biz、不带堆栈。
func ReportBizWithLoggerContext(ctx context.Context, l Logger, biz BizLog, fields ...zap.Field) {
	if l == nil {
		return
	}
	action := biz.Action
	if action == "" {
		action = "biz_reject"
	}
	reason := biz.Reason
	message := biz.Message

	base := []zap.Field{
		zap.String("err_type", "biz"),
		zap.String("action", action),
	}
	if reason != "" {
		base = append(base, zap.String("reason", reason))
	}
	if message != "" {
		base = append(base, zap.String("biz_message", message))
	}
	base = append(base, fields...)

	msg := action
	if reason != "" && message != "" {
		msg = fmt.Sprintf("%s, reason:%s, msg:%s", action, reason, message)
	} else if reason != "" {
		msg = fmt.Sprintf("%s, reason:%s", action, reason)
	} else if message != "" {
		msg = fmt.Sprintf("%s, msg:%s", action, message)
	}
	l.WithContext(ctx).Info(msg, base...)
}

// ReportSysErrorWithLoggerContext 记录技术错误日志：ERROR、err_type=sys，可附带栈信息。
func ReportSysErrorWithLoggerContext(ctx context.Context, l Logger, sys SysLog, fields ...zap.Field) {
	if sys.Err == nil || l == nil {
		return
	}
	action := sys.Action
	if action == "" {
		action = "sys_error"
	}
	err := sys.Err

	meta := BuildErrorLog(err)
	base := []zap.Field{
		zap.String("err_type", "sys"),
		zap.String("action", action),
	}
	if meta.Code != "" {
		base = append(base, zap.String("error_code", meta.Code))
	}
	if len(meta.CauseChain) != 0 {
		base = append(base, zap.Any("cause_chain", meta.CauseChain))
	}
	if len(meta.Data) != 0 {
		base = append(base, zap.Any("error_data", meta.Data))
	}
	if meta.Origin != "" {
		base = append(base, zap.String("origin_caller", meta.Origin))
	}
	if meta.Stack != "" {
		base = append(base, zap.String("stack_origin", meta.Stack))
	}
	base = append(base, fields...)

	finalMsg := action
	if meta.Reason != "" {
		finalMsg = fmt.Sprintf("%s, reason:%s, error:%s", action, meta.Reason, meta.Error)
	} else if meta.Msg != "" {
		finalMsg = fmt.Sprintf("%s, error:%s, msg:%s", action, meta.Error, meta.Msg)
	} else {
		finalMsg = fmt.Sprintf("%s, error:%s", action, meta.Error)
	}
	l.WithContext(ctx).Error(finalMsg, base...)
}
