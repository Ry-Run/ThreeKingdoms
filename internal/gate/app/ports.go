package app

import (
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"context"

	"google.golang.org/grpc"
)

type AccountServiceClient interface {
	// 登录
	Login(ctx context.Context, req *accountpb.LoginRequest, opts ...grpc.CallOption) (*accountpb.LoginReply, error)
	// 注册
	Register(ctx context.Context, req *accountpb.RegisterRequest, opts ...grpc.CallOption) (*accountpb.RegisterReply, error)
	// 进入游戏服
	EnterServer(ctx context.Context, in *accountpb.EnterServerRequest, opts ...grpc.CallOption) (*accountpb.EnterServerReply, error)
}
