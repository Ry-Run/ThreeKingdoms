package app

import (
	"errors"
	"fmt"
	"testing"
)

func TestError_Is_按错误码匹配(t *testing.T) {
	err := NewError(CodeInvalidCredentials, "用户名或密码错误")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("期望 errors.Is(err, ErrInvalidCredentials) == true, err=%v", err)
	}

	wrapped := fmt.Errorf("wrap: %w", err)
	if !errors.Is(wrapped, ErrInvalidCredentials) {
		t.Fatalf("期望 errors.Is(wrapped, ErrInvalidCredentials) == true, wrapped=%v", wrapped)
	}
}

func TestError_Unwrap_保留cause链(t *testing.T) {
	cause := errors.New("db down")
	err := Wrap(CodeInternalServer, "服务器内部错误", cause)

	if !errors.Is(err, ErrInternalServer) {
		t.Fatalf("期望 errors.Is(err, ErrInternalServer) == true, err=%v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("期望 errors.Is(err, cause) == true, err=%v", err)
	}

	if got := err.Stack(); len(got) == 0 {
		t.Fatalf("期望带 cause 的应用错误捕获栈（发生/转换处），got=%v", got)
	}
}

func TestError_WithData_不污染原对象(t *testing.T) {
	base := ErrInvalidCredentials
	err := base.WithData("method", "Login")

	if base.Data() != nil {
		t.Fatalf("期望 base.Data() == nil（不应污染哨兵错误），base=%v", base)
	}
	if got := err.Data()["method"]; got != "Login" {
		t.Fatalf("期望 err.Data()[\"method\"] == Login, got=%v", got)
	}
}

func TestError_WithCause_业务错误不捕获栈(t *testing.T) {
	cause := errors.New("some infra error")
	err := ErrInvalidCredentials.WithCause(cause)
	if got := err.Stack(); got != nil {
		t.Fatalf("期望业务类错误不捕获栈，got=%v", got)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("期望 cause 链不丢，err=%v", err)
	}
}
