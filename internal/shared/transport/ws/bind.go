package ws

import (
	"encoding/json"
	"errors"
)

// BindJSON 将 WsMsgReq.Body.Msg 反序列化到目标结构体。
func BindJSON(req *WsMsgReq, dst any) error {
	if req == nil || req.Body == nil {
		return errors.New("ws request body is nil")
	}
	raw, err := json.Marshal(req.Body.Msg)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}
