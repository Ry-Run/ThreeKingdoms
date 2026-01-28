package ws

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-think/openssl"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/security"
	"ThreeKingdoms/internal/shared/utils"
)

type WsServer struct {
	conn     *websocket.Conn
	router   *Router
	outChan  chan *WsMsgResp
	Seq      int64
	property map[string]any
	sync.RWMutex
}

func NewWsServer(wsConn *websocket.Conn) *WsServer {
	return &WsServer{
		conn:     wsConn,
		outChan:  make(chan *WsMsgResp, 1000),
		property: make(map[string]any),
		Seq:      0,
	}
}

func (s *WsServer) Router(router *Router) {
	s.router = router
}

func (s *WsServer) SetProperty(key string, value any) {
	s.Lock()
	defer s.Unlock()
	s.property[key] = value
}

func (s *WsServer) GetProperty(key string) any {
	s.RLock()
	defer s.RUnlock()
	return s.property[key]
}

func (s *WsServer) RemoveProperty(key string) {
	s.Lock()
	defer s.Unlock()
	delete(s.property, key)
}

func (s *WsServer) Addr() string {
	return s.conn.RemoteAddr().String()
}

func (s *WsServer) Push(name string, data any) {
	rsp := WsMsgResp{
		Body: &RespBody{
			Seq:  s.Seq,
			Name: name,
			Msg:  data,
		},
	}
	s.outChan <- &rsp
}

func (s *WsServer) Run() {
	go s.readMsgLoop()
	go s.writeMsgLoop()
}

func (s *WsServer) readMsgLoop() {
	defer func() {
		if err := recover(); err != nil {
			e := fmt.Sprintf("%v", err)
			logs.Error("ws readMsgLoop error", zap.String("err", e))
		}
		s.Close()
	}()
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			logs.Error("ws_server read msg", zap.Error(err))
			continue
		}

		// 前端发送的是压缩加密过的json
		//1. 解压缩
		secretData, err := security.UnZip(data)
		if err != nil {
			logs.Error("ws_server readMsgLoop unzip", zap.Error(err))
			continue
		}

		// 2.获取密匙
		secretKey := s.GetProperty(SecretKey)
		if secretKey == nil {
			logs.Error("ws_server readMsgLoop not found secretKey", zap.String(SecretKey, string(secretData)))
			continue
		}

		// 3.解密数据
		key := secretKey.(string)
		decryptedData, err := security.AesCBCDecrypt(data, []byte(key), []byte(key), openssl.ZEROS_PADDING)
		if err != nil {
			logs.Error("ws_server readMsgLoop decrypt error", zap.Error(err))
			// 出错后，发起握手
			s.handshake()
			continue
		}

		// 4.转为 json
		body := ReqBody{}
		err = json.Unmarshal(decryptedData, &body)
		if err != nil {
			logs.Error("ws_server readMsgLoop unmarshal json error", zap.Error(err))
			continue
		}

		logs.Info("ws_server read msg", zap.Any("data", body))

		// 5.分发消息
		req := WsMsgReq{Body: &body, Conn: s}
		resp := WsMsgResp{Body: &RespBody{Seq: s.Seq, Name: body.Name, Msg: body.Msg}}
		s.router.Dispatch(req, &resp)
	}
}

func (s *WsServer) writeMsgLoop() {
	for {
		select {
		case msg, ok := <-s.outChan:
			if ok {
				logs.Info("ws_server write msg", zap.Any("msg", msg))
				s.write(msg)
			}
		}
	}
}

func (s *WsServer) Close() {
	_ = s.conn.Close()
}

func (s *WsServer) write(msg *WsMsgResp) {
	// 转成 json
	marshal, err := json.Marshal(msg.Body)
	if err != nil {
		logs.Error("ws_server write marshal json error", zap.Error(err))
		return
	}

	// 获取密匙
	secretKey := s.GetProperty(SecretKey)
	if secretKey == nil {
		logs.Error("ws_server write not found secretKey", zap.Any("msg", msg))
		return
	}

	// 加密
	key := secretKey.(string)
	encryptedData, err := security.AesCBCEncrypt(marshal, []byte(key), []byte(key), openssl.ZEROS_PADDING)
	if err != nil {
		logs.Error("ws_server write decrypt error", zap.Error(err))
		return
	}

	// 压缩
	zip, err := security.Zip(encryptedData)
	if err != nil {
		logs.Error("ws_server write zip error", zap.Error(err))
	}

	err = s.conn.WriteMessage(websocket.TextMessage, zip)
}

func (s *WsServer) handshake() {
	secretKey := ""
	key := s.GetProperty(SecretKey)
	if key == nil {
		secretKey = utils.RandSeq(16)
	} else {
		secretKey = key.(string)
	}

	handshake := &Handshake{Key: secretKey}
	body := &RespBody{Name: HandshakeMsg, Msg: handshake}

	data, err := json.Marshal(body)
	if err != nil {
		logs.Error("ws_server handshake marshal json error", zap.Error(err))
	}

	if secretKey != "" {
		s.SetProperty(SecretKey, secretKey)
	} else {
		s.RemoveProperty(SecretKey)
	}

	s.conn.WriteMessage(websocket.BinaryMessage, data)
}
