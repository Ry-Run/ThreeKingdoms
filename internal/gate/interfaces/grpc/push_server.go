package grpc

import (
	"ThreeKingdoms/internal/gate/interfaces/handler/ws/dto"
	gatepb "ThreeKingdoms/internal/shared/gen/gate"
	"ThreeKingdoms/internal/shared/session"
	"context"
)

type PushServer struct {
	gatepb.UnimplementedGatePushServiceServer
	sessMgr session.Manager
}

func NewPushServer(sessMgr session.Manager) *PushServer {
	return &PushServer{sessMgr: sessMgr}
}

func (s *PushServer) PushWorldBatch(ctx context.Context, req *gatepb.PushWorldBatchRequest) (*gatepb.PushWorldBatchReply, error) {
	if s == nil || s.sessMgr == nil || req == nil || len(req.Items) == 0 || req.MsgType == "" {
		return &gatepb.PushWorldBatchReply{Ok: true}, nil
	}
	for _, item := range req.Items {
		if item == nil || item.PlayerId <= 0 || item.Army == nil {
			continue
		}
		conn, ok := s.sessMgr.GetConn(int(item.PlayerId))
		if !ok || conn == nil {
			continue
		}
		conn.Push(req.MsgType, dto.NewArmy(item.Army))
	}
	return &gatepb.PushWorldBatchReply{Ok: true}, nil
}

var _ gatepb.GatePushServiceServer = (*PushServer)(nil)
