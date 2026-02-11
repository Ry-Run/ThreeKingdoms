package ws

import (
	"ThreeKingdoms/modules/kit/logx"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-think/openssl"
	"github.com/go-viper/mapstructure/v2"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

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
	done      chan struct{}
	closeOnce sync.Once
	log       logx.Logger
}

func NewWsServer(wsConn *websocket.Conn, l logx.Logger) *WsServer {
	return &WsServer{
		conn:     wsConn,
		outChan:  make(chan *WsMsgResp, 1000),
		property: make(map[string]any),
		Seq:      0,
		done:     make(chan struct{}),
		log:      l,
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
			Seq:  0,
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
			s.log.Error("ws readMsgLoop error", zap.String("err", e))
		}
		s.Close()
	}()
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			s.log.Error("ws_server read msg", zap.Error(err))
			return
		}

		// 前端发送的是压缩加密过的json
		//1. 解压缩
		secretData, err := security.UnZip(data)
		if err != nil {
			s.log.Error("ws_server readMsgLoop unzip", zap.Error(err))
			continue
		}

		// 2.获取密匙
		secretKey := s.GetProperty(SecretKey)
		if secretKey == nil {
			s.log.Error("ws_server readMsgLoop not found secretKey", zap.String(SecretKey, string(secretData)))
			continue
		}

		// 3.解密数据
		key := secretKey.(string)
		decryptedData, err := security.AesCBCDecrypt(secretData, []byte(key), []byte(key), openssl.ZEROS_PADDING)
		if err != nil {
			s.log.Error("ws_server readMsgLoop decrypt error", zap.Error(err))
			// 出错后，发起握手
			s.handshake()
			continue
		}

		// 4.转为 json
		reqBody := ReqBody{}
		err = json.Unmarshal(decryptedData, &reqBody)
		if err != nil {
			s.log.Error("ws_server readMsgLoop unmarshal json error", zap.Error(err))
			continue
		}

		// 5.分发消息
		req := WsMsgReq{Body: &reqBody, Conn: s}
		// req 和 resp 的 Seq 必须一致
		resp := WsMsgResp{Body: &RespBody{Seq: req.Body.Seq, Name: reqBody.Name, Msg: reqBody.Msg}}
		if reqBody.Name == HeartbeatMsg {
			// 回复客户端心跳，心跳放服务端合适，目前只能满足客户端的条件
			h := &Heartbeat{}
			mapstructure.Decode(reqBody.Msg, h)
			h.STime = time.Now().UnixNano() / 1e6
			resp.Body.Msg = h
		} else {
			s.log.Info("ws_server read msg", zap.Any("data", reqBody))
			s.router.Dispatch(&req, &resp)
		}

		s.Push(reqBody.Name, &resp)
	}
}

func (s *WsServer) writeMsgLoop() {
	for {
		select {
		case msg, ok := <-s.outChan:
			if ok {
				if msg.Body.Name != HeartbeatMsg {
					s.log.Info("ws_server write msg", zap.Any("msg", msg))
				}
				s.write(msg)
			}
		case <-s.done:
			return
		}
	}
}

func (s *WsServer) Close() {
	s.closeOnce.Do(func() {
		_ = s.conn.Close()
		close(s.done)
	})
}

func (s *WsServer) Done() <-chan struct{} {
	return s.done
}

func (s *WsServer) write(msg *WsMsgResp) {
	// 转成 json
	marshal, err := json.Marshal(msg.Body)
	if err != nil {
		s.log.Error("ws_server write marshal json error", zap.Error(err))
		return
	}

	// 获取密匙
	secretKey := s.GetProperty(SecretKey)
	if secretKey == nil {
		s.log.Error("ws_server write not found secretKey", zap.Any("msg", msg))
		return
	}

	// 加密
	key := secretKey.(string)
	encryptedData, err := security.AesCBCEncrypt(marshal, []byte(key), []byte(key), openssl.ZEROS_PADDING)
	if err != nil {
		s.log.Error("ws_server write decrypt error", zap.Error(err))
		return
	}

	// 压缩
	zip, err := security.Zip(encryptedData)
	if err != nil {
		s.log.Error("ws_server write zip error", zap.Error(err))
	}

	// 压缩后的密文是二进制字节流，必须走 BinaryMessage，不能走 TextMessage
	if err := s.conn.WriteMessage(websocket.BinaryMessage, zip); err != nil {
		s.log.Error("ws_server write error", zap.Error(err))
	}
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
		s.log.Error("ws_server handshake marshal json error", zap.Error(err))
	}

	if secretKey != "" {
		s.SetProperty(SecretKey, secretKey)
	} else {
		s.RemoveProperty(SecretKey)
	}

	// 压缩
	zipData, err := security.Zip(data)

	if err := s.conn.WriteMessage(websocket.BinaryMessage, zipData); err != nil {
		s.log.Error("ws_server handshake write error", zap.Error(err))
	}
}
