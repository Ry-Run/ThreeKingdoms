package net

type ReqBody struct {
	Seq   int64  `json:"seq"`
	Name  string `json:"name"`
	Msg   any    `json:"msg"`
	Proxy string `json:"proxy"`
}

type RespBody struct {
	Seq  int64  `json:"seq"`
	Name string `json:"name"`
	Code int    `json:"code"`
	Msg  any    `json:"msg"`
}

type WsMsgReq struct {
	Body *ReqBody
	Conn WSConn
}

type WsMsgResp struct {
	Body *RespBody
}

// 理解为 request请求 请求会有参数 请求中放参数 取参数
type WSConn interface {
	SetProperty(key string, value any)
	GetProperty(key string) any
	RemoveProperty(key string)
	Addr() string
	Push(name string, data any)
	Close()
}

type Handshake struct {
	Key string `json:"key"`
}

const HandshakeMsg = "handshake"

const SecretKey = "secretKey"
