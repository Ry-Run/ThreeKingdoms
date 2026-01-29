package security

import "testing"

func TestAward_缺少JWT_SECRET应失败(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := Award(1); err == nil {
		t.Fatalf("期望 JWT_SECRET 为空时 Award 返回错误")
	}
}

func TestAwardParse_正常签发并解析(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-123")

	token, err := Award(42)
	if err != nil {
		t.Fatalf("Award err=%v", err)
	}
	if token == "" {
		t.Fatalf("期望 token 非空")
	}

	_, claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken err=%v", err)
	}
	if claims == nil || claims.Uid != 42 {
		t.Fatalf("期望 claims.Uid==42, got=%v", claims)
	}
	t.Log(token)
}
