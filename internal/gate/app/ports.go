package app

import (
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"context"

	"google.golang.org/grpc"
)

type AccountServiceClient interface {
	// 登录
	Login(ctx context.Context, req *accountpb.LoginRequest, opts ...grpc.CallOption) (*accountpb.LoginReply, error)
	// 注册
	Register(ctx context.Context, req *accountpb.RegisterRequest, opts ...grpc.CallOption) (*accountpb.RegisterReply, error)
}

type PlayerServiceClient interface {
	// Handle 玩家服务统一入口（内部再分发到 player/world 相关 handler）
	Handle(ctx context.Context, req *playerpb.PlayerRequest, opts ...grpc.CallOption) (*playerpb.PlayerResponse, error)
}
