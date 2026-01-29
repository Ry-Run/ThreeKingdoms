package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestError_Is_按错误码匹配(t *testing.T) {
	err := NewError(CodeUserNotFound, map[string]any{"userId": int64(1)}, nil)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("期望 errors.Is(err, ErrUserNotFound) == true, err=%v", err)
	}

	wrapped := fmt.Errorf("wrap: %w", err)
	if !errors.Is(wrapped, ErrUserNotFound) {
		t.Fatalf("期望 errors.Is(wrapped, ErrUserNotFound) == true, wrapped=%v", wrapped)
	}
}

func TestError_Unwrap_保留cause链(t *testing.T) {
	cause := errors.New("db timeout")
	err := NewError(CodeSystemUnavailable, nil, cause)

	if !errors.Is(err, ErrSystemUnavailable) {
		t.Fatalf("期望 errors.Is(err, ErrSystemUnavailable) == true, err=%v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("期望 errors.Is(err, cause) == true, err=%v", err)
	}

	if sp, ok := any(err).(interface{ Stack() []uintptr }); ok {
		if got := sp.Stack(); len(got) == 0 {
			t.Fatalf("期望带 cause 的领域错误捕获栈（发生/转换处），got=%v", got)
		}
	} else {
		t.Fatalf("期望领域错误实现 Stack() 用于溯源")
	}
}

func TestError_WithCause_业务错误不捕获栈(t *testing.T) {
	cause := errors.New("some infra error")
	err := ErrUserNotFound.WithCause(cause)
	if got := err.Stack(); got != nil {
		t.Fatalf("期望业务类错误不捕获栈，got=%v", got)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("期望 cause 链不丢，err=%v", err)
	}
}

func TestError_WithData_不污染原对象(t *testing.T) {
	base := ErrUserNotFound
	err := base.WithData("userId", int64(1))

	if base.Data() != nil {
		t.Fatalf("期望 base.Data() == nil（不应污染哨兵错误），base=%v", base)
	}
	if got := err.Data()["userId"]; got != int64(1) {
		t.Fatalf("期望 err.Data()[\"userId\"] == 1, got=%v", got)
	}

	m := map[string]any{"k": "v"}
	err2 := NewError(CodeUserDisabled, m, nil)
	m["k"] = "mutated"
	if got := err2.Data()["k"]; got != "v" {
		t.Fatalf("期望构造时复制 data，避免外部后续修改影响错误上下文；got=%v", got)
	}
}
