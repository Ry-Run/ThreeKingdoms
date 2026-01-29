package errx

import (
	"errors"
	"testing"
)

func TestError_Is_只按code比较语义(t *testing.T) {
	e1 := NewBiz("BIZ_X", "x").WithData("k", "v").WithCause(errors.New("cause1"))
	e2 := NewBiz("BIZ_X", "x2").WithData("k2", "v2").WithCause(errors.New("cause2"))
	if !errors.Is(e1, e2) {
		t.Fatalf("期望 errors.Is(e1, e2)==true（只按 code 判断语义），e1=%v e2=%v", e1, e2)
	}
}

func TestError_业务错误不捕获栈_但保留cause链(t *testing.T) {
	cause := errors.New("db down")
	err := NewBiz("BIZ_LOGIN_FAIL", "用户名或密码错误").WithCause(cause)
	if got := err.Stack(); got != nil {
		t.Fatalf("期望业务错误不捕获栈，got=%v", got)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("期望 cause 链不丢，err=%v", err)
	}
}

func TestError_系统错误捕获一次栈_且不重复捕获(t *testing.T) {
	cause := errors.New("io timeout")
	sys := NewSys("SYS_DB_UNAVAILABLE", "系统不可用").WithCause(cause)
	if got := sys.Stack(); len(got) == 0 {
		t.Fatalf("期望系统错误捕获栈（发生/转换处），got=%v", got)
	}

	// 再包一层系统错误：如果下层已有栈，上层不应重复捕获
	sys2 := NewSys("SYS_GATEWAY_ERROR", "网关异常").WithCause(sys)
	if got := sys2.Stack(); got != nil {
		t.Fatalf("期望上层系统错误不重复捕获栈（cause 链里已有栈），got=%v", got)
	}
}

func TestError_Data_防止外部map污染(t *testing.T) {
	m := map[string]any{"k": "v"}
	err := NewBiz("BIZ_X", "").WithDataMap(m)
	m["k"] = "mutated"
	if got := err.Data()["k"]; got != "v" {
		t.Fatalf("期望构造时复制 data，避免外部后续修改影响错误上下文；got=%v", got)
	}
}
