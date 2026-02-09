package logx

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
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

type reasonProvider interface {
	Reason() string
}

type ErrorLog struct {
	Error      string
	Code       string
	Msg        string
	Reason     string
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
	var rp reasonProvider
	if errors.As(err, &rp) {
		out.Reason = rp.Reason()
	}
	var sp stackProvider
	if errors.As(err, &sp) {
		out.Origin, out.Stack = formatStack(sp.Stack(), 32)
	}
	out.CauseChain = buildCauseChain(err, 20)
	return out
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
