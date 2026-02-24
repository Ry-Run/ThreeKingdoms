package grpc

import (
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DialAccountService 建立 account grpc 连接并返回 typed client。
func DialAccountService(accountService string) (*grpc.ClientConn, accountpb.AccountServiceClient, error) {
	// grpc Dial 拨号配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(UnaryClientTraceInterceptor()),
		grpc.WithChainStreamInterceptor(StreamClientTraceInterceptor()),
	}
	// grpc Dial 初始化网络模块: addr = resolver.Scheme() + ip + port
	// 1.创建 ClientConn（核心对象）
	// 2.根据 target 的 scheme 选 resolver
	// 3.初始化负载均衡器（balancer）
	// 4.启动 resolver（异步）
	// 5.把地址交给 balancer
	conn, err := grpc.NewClient(accountService, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("dial account service failed: %w", err)
	}
	return conn, accountpb.NewAccountServiceClient(conn), nil
}

// DialPlayerService 建立 player grpc 连接并返回 typed client。
func DialPlayerService(playerService string) (*grpc.ClientConn, playerpb.PlayerServiceClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(UnaryClientTraceInterceptor()),
		grpc.WithChainStreamInterceptor(StreamClientTraceInterceptor()),
	}
	conn, err := grpc.NewClient(playerService, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("dial player service failed: %w", err)
	}
	return conn, playerpb.NewPlayerServiceClient(conn), nil
}
