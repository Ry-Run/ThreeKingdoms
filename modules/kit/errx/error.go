package errx

import (
	"errors"
	"fmt"
	"runtime"
)

// Code 表示错误码（对外语义的稳定标识）。
type Code string

type kind uint8

const (
	kindBiz kind = iota
	kindSys
)

// Reason 是错误原因的最小接口，只暴露 reason code。
type Reason interface {
	ReasonCode() string
}

// Error 是通用错误模型：
// - code/msg：对外语义
// - data：业务/应用上下文（禁止外部修改，内部会复制）
// - cause：原始错误链（仅用于溯源，不参与对外语义）
// - stack：只在“系统类错误”第一次 wrap/转换处捕获一次，用于溯源定位
type Error struct {
	code  Code
	msg   string
	data  map[string]any
	cause error
	stack []uintptr
	kind  kind
}

func NewBiz(code Code, msg string) *Error {
	return &Error{
		code: code,
		msg:  msg,
		kind: kindBiz,
	}
}

func NewSys(code Code, msg string) *Error {
	return &Error{
		code: code,
		msg:  msg,
		kind: kindSys,
	}
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.msg == "" {
		if e.cause == nil {
			return string(e.code)
		}
		return fmt.Sprintf("%s: %v", e.code, e.cause)
	}
	if e.cause == nil {
		return fmt.Sprintf("%s: %s", e.code, e.msg)
	}
	return fmt.Sprintf("%s: %s: %v", e.code, e.msg, e.cause)
}

// Unwrap 让 errors.Is / errors.As 可以沿着 cause 链溯源。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 让 errors.Is 仅按错误码判断“语义是否相同”，忽略 msg/data/cause。
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	t, ok := target.(*Error)
	if !ok || t == nil {
		return false
	}
	return e.code == t.code
}

func (e *Error) Code() Code {
	if e == nil {
		return ""
	}
	return e.code
}

func (e *Error) CodeText() string {
	if e == nil {
		return ""
	}
	return string(e.code)
}

func (e *Error) Msg() string {
	if e == nil {
		return ""
	}
	return e.msg
}

// Data 返回 data 的拷贝，避免外部修改影响错误上下文。
func (e *Error) Data() map[string]any {
	if e == nil || e.data == nil {
		return nil
	}
	return cloneAnyMap(e.data)
}

// Reason 返回约定的字符串原因码（存储在 data.reason）。
func (e *Error) Reason() string {
	if e == nil || e.data == nil {
		return ""
	}
	v, ok := e.data["reason"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// Stack 返回“错误最早发生/被转换那一刻”的调用栈（只对系统类错误生效，且只捕获一次）。
func (e *Error) Stack() []uintptr {
	if e == nil || len(e.stack) == 0 {
		return nil
	}
	out := make([]uintptr, len(e.stack))
	copy(out, e.stack)
	return out
}

func (e *Error) WithData(key string, value any) *Error {
	next := &Error{
		code:  e.code,
		msg:   e.msg,
		data:  cloneAnyMap(e.data),
		cause: e.cause,
		stack: cloneStack(e.stack),
		kind:  e.kind,
	}
	if next.data == nil {
		next.data = make(map[string]any, 1)
	}
	next.data[key] = value
	return next
}

// WithReason 是 WithData("reason", reason.ReasonCode()) 的语义化快捷方法。
func (e *Error) WithReason(reason Reason) *Error {
	if reason == nil {
		return e.WithData("reason", "")
	}
	return e.WithData("reason", reason.ReasonCode())
}

func (e *Error) WithDataMap(data map[string]any) *Error {
	next := &Error{
		code:  e.code,
		msg:   e.msg,
		data:  cloneAnyMap(e.data),
		cause: e.cause,
		stack: cloneStack(e.stack),
		kind:  e.kind,
	}
	if len(data) == 0 {
		return next
	}
	if next.data == nil {
		next.data = make(map[string]any, len(data))
	}
	for k, v := range data {
		next.data[k] = v
	}
	return next
}

func (e *Error) WithCause(cause error) *Error {
	next := &Error{
		code:  e.code,
		msg:   e.msg,
		data:  cloneAnyMap(e.data),
		cause: cause,
		stack: cloneStack(e.stack),
		kind:  e.kind,
	}
	// 只在系统类错误首次挂 cause 时捕获一次；如果下层已有栈，则不上浮重复捕获。
	if next.kind == kindSys && cause != nil && len(next.stack) == 0 && !hasStackInChain(cause) {
		next.stack = captureStack(3)
	}
	return next
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStack(in []uintptr) []uintptr {
	if len(in) == 0 {
		return nil
	}
	out := make([]uintptr, len(in))
	copy(out, in)
	return out
}

func captureStack(skip int) []uintptr {
	const maxDepth = 64
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip, pcs)
	if n <= 0 {
		return nil
	}
	return pcs[:n]
}

func hasStackInChain(err error) bool {
	const maxDepth = 32
	for i := 0; i < maxDepth && err != nil; i++ {
		if sp, ok := err.(interface{ Stack() []uintptr }); ok && len(sp.Stack()) != 0 {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
