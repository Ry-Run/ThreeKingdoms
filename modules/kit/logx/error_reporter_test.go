package logx

import (
	"errors"
	"testing"

	"ThreeKingdoms/modules/kit/errx"
)

func TestBuildErrorLog_能提取语义与栈(t *testing.T) {
	cause := errors.New("db down")
	e := errx.NewSys("SYS_INTERNAL", "服务器内部错误").
		WithData("method", "Login").
		WithCause(cause)

	meta := BuildErrorLog(e)
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
	if meta.Origin == "" || meta.Stack == "" {
		t.Fatalf("期望 meta.Origin/meta.Stack 非空（错误发生/转换处栈） origin=%q stack=%q", meta.Origin, meta.Stack)
	}
}
