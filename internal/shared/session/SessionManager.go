package session

import (
	"ThreeKingdoms/internal/shared/transport/ws"
	"sync"
)

type Manager interface {
	Bind(uid int, token string, conn ws.WSConn)
	UnbindConn(conn ws.WSConn)
	UnbindUID(uid int)
	GetConn(uid int) (ws.WSConn, bool)
	GetUID(conn ws.WSConn) (int, bool)
}

type SessMgr struct {
	sync.RWMutex
	uid2token map[int]string
	uid2conn  map[int]ws.WSConn
	conn2uid  map[ws.WSConn]int
	watched   map[ws.WSConn]struct{}
}

func NewSessMgr() Manager {
	return &SessMgr{
		uid2token: make(map[int]string),
		uid2conn:  make(map[int]ws.WSConn),
		conn2uid:  make(map[ws.WSConn]int),
		watched:   make(map[ws.WSConn]struct{}),
	}
}

func (s *SessMgr) Bind(uid int, token string, conn ws.WSConn) {
	if conn == nil {
		return
	}
	s.Lock()
	defer s.Unlock()

	// 为每条连接只启动一次 watcher：连接关闭后自动解绑，避免 conn2uid 逐步膨胀
	if _, ok := s.watched[conn]; !ok {
		s.watched[conn] = struct{}{}
		go s.watchConnDone(conn)
	}

	oldConn := s.uid2conn[uid]
	// 踢掉原来的那个
	if oldConn != nil && oldConn != conn {
		oldConn.Push("robLogin", nil)
		oldConn.Close()
	}
	s.uid2conn[uid] = conn
	s.conn2uid[conn] = uid
	s.uid2token[uid] = token
}

func (s *SessMgr) watchConnDone(conn ws.WSConn) {
	<-conn.Done()
	s.UnbindConn(conn)
}

func (s *SessMgr) UnbindConn(conn ws.WSConn) {
	s.Lock()
	defer s.Unlock()
	uid := s.conn2uid[conn]
	delete(s.watched, conn)
	delete(s.conn2uid, conn)
	if s.uid2conn[uid] == conn {
		delete(s.uid2conn, uid)
	}
}

func (s *SessMgr) UnbindUID(uid int) {
	s.Lock()
	defer s.Unlock()
	conn, ok := s.uid2conn[uid]
	if ok {
		delete(s.watched, conn)
		delete(s.conn2uid, conn)
	}
	delete(s.uid2conn, uid)
}

func (s *SessMgr) GetConn(uid int) (ws.WSConn, bool) {
	s.RLock()
	defer s.RUnlock()
	conn, ok := s.uid2conn[uid]
	return conn, ok
}

func (s *SessMgr) GetUID(conn ws.WSConn) (int, bool) {
	s.RLock()
	defer s.RUnlock()
	uid, ok := s.conn2uid[conn]
	return uid, ok
}
