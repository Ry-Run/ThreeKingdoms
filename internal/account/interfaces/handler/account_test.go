package handler

import (
	"encoding/json"
	"testing"

	"ThreeKingdoms/internal/account/dto"
)

func TestLoginMsgDecode_能从map解析为LoginReq(t *testing.T) {
	msg := map[string]any{
		"username": "u1",
		"password": "p1",
		"ip":       "1.1.1.1",
		"hardware": "h1",
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var req dto.LoginReq
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.Username != "u1" || req.Password != "p1" || req.Ip != "1.1.1.1" || req.Hardware != "h1" {
		t.Fatalf("解析结果不符合预期: %+v", req)
	}
}
