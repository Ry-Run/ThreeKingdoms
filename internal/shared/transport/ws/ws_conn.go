package ws

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
	// Done 用于感知连接生命周期结束（连接关闭时该 channel 会被关闭）
	Done() <-chan struct{}
}

type Handshake struct {
	Key string `json:"key"`
}

type Heartbeat struct {
	CTime int64 `json:"ctime"`
	STime int64 `json:"stime"`
}

const (
	HandshakeMsg = "handshake"
	SecretKey    = "secretKey"
	ConnKeyUID   = "uid"
	HeartbeatMsg = "heartbeat"
)
