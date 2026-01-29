package logx

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"ThreeKingdoms/modules/kit/tracex"

	"go.uber.org/zap"
)

type codeTextProvider interface {
	CodeText() string
}

type msgProvider interface {
	Msg() string
}

type dataProvider interface {
	Data() map[string]any
}

type stackProvider interface {
	Stack() []uintptr
}

type ErrorLog struct {
	Error      string
	Code       string
	Msg        string
	Data       map[string]any
	CauseChain []string
	Origin     string
	Stack      string
}

// BuildErrorLog 把“错误码/语义/上下文/cause链/发生处栈”提取成便于阅读的结构，用于接口层统一打印。
func BuildErrorLog(err error) ErrorLog {
	if err == nil {
		return ErrorLog{}
	}

	out := ErrorLog{
		Error: err.Error(),
	}

	var cp codeTextProvider
	if errors.As(err, &cp) {
		out.Code = cp.CodeText()
	}
	var mp msgProvider
	if errors.As(err, &mp) {
		out.Msg = mp.Msg()
	}
	var dp dataProvider
	if errors.As(err, &dp) {
		out.Data = dp.Data()
	}
	var sp stackProvider
	if errors.As(err, &sp) {
		out.Origin, out.Stack = formatStack(sp.Stack(), 32)
	}
	out.CauseChain = buildCauseChain(err, 20)
	return out
}

// ReportError 在接口层打印一次“易读”的错误日志：建议每个请求/消息只调用一次。
func ReportError(l *zap.Logger, msg string, err error, fields ...zap.Field) {
	ReportErrorContext(context.Background(), l, msg, err, fields...)
}

// ReportErrorContext 会把 trace/span 一起带入日志字段（如果 ctx 中存在）。
func ReportErrorContext(ctx context.Context, l *zap.Logger, msg string, err error, fields ...zap.Field) {
	if err == nil || l == nil {
		return
	}

	meta := BuildErrorLog(err)

	base := make([]zap.Field, 0, 10+len(fields))
	base = append(base, zap.String("error", meta.Error))
	if meta.Code != "" {
		base = append(base, zap.String("error_code", meta.Code))
	}
	if meta.Msg != "" {
		base = append(base, zap.String("error_msg", meta.Msg))
	}
	if meta.Data != nil {
		base = append(base, zap.Any("error_data", meta.Data))
	}
	if len(meta.CauseChain) != 0 {
		base = append(base, zap.Any("cause_chain", meta.CauseChain))
	}
	if meta.Origin != "" {
		base = append(base, zap.String("origin_caller", meta.Origin))
	}
	if meta.Stack != "" {
		base = append(base, zap.String("stack_origin", meta.Stack))
	}
	if tid, ok := tracex.TraceIDFrom(ctx); ok {
		base = append(base, zap.String("trace_id", tid))
	}
	if sid, ok := tracex.SpanIDFrom(ctx); ok {
		base = append(base, zap.String("span_id", sid))
	}
	base = append(base, fields...)

	l.Error(msg, base...)
}

func buildCauseChain(err error, maxDepth int) []string {
	if err == nil || maxDepth <= 0 {
		return nil
	}
	out := make([]string, 0, 4)
	cur := errors.Unwrap(err)
	for i := 0; i < maxDepth && cur != nil; i++ {
		out = append(out, fmt.Sprintf("%T: %v", cur, cur))
		cur = errors.Unwrap(cur)
	}
	return out
}

func formatStack(pcs []uintptr, maxFrames int) (originCaller string, stack string) {
	if len(pcs) == 0 || maxFrames <= 0 {
		return "", ""
	}
	frames := runtime.CallersFrames(pcs)
	var b strings.Builder
	for i := 0; i < maxFrames; i++ {
		f, more := frames.Next()
		if f.Function == "" && f.File == "" && f.Line == 0 {
			break
		}
		if originCaller == "" {
			originCaller = fmt.Sprintf("%s %s:%d", f.Function, f.File, f.Line)
		}
		b.WriteString(f.Function)
		b.WriteString(" ")
		b.WriteString(f.File)
		b.WriteString(":")
		b.WriteString(fmt.Sprintf("%d", f.Line))
		if !more {
			break
		}
		b.WriteString("\n")
	}
	return originCaller, b.String()
}
