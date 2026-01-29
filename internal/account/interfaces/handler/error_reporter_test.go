package handler

import (
	"errors"
	"testing"

	"ThreeKingdoms/internal/account/app"
)

func TestBuildErrorLog_包含语义与cause链与栈(t *testing.T) {
	cause := errors.New("db down")
	err := app.ErrInternalServer.WithData("method", "Login").WithCause(cause)

	meta := BuildErrorLog(err)
	if meta.Error == "" {
		t.Fatalf("期望 meta.Error 非空")
	}
	if meta.Code == "" {
		t.Fatalf("期望 meta.Code 非空")
	}
	if meta.Msg == "" {
		t.Fatalf("期望 meta.Msg 非空")
	}
	if meta.Data == nil || meta.Data["method"] != "Login" {
		t.Fatalf("期望 meta.Data 包含 method=Login, got=%v", meta.Data)
	}
	if len(meta.CauseChain) == 0 {
		t.Fatalf("期望 meta.CauseChain 非空")
	}
	if meta.Origin == "" {
		t.Fatalf("期望 meta.Origin 非空（错误发生/转换处 caller）")
	}
	if meta.Stack == "" {
		t.Fatalf("期望 meta.Stack 非空（错误发生/转换处栈）")
	}
}
