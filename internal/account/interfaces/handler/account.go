package handler

import (
	"ThreeKingdoms/internal/account/app"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"ThreeKingdoms/modules/kit/logx"
	"ThreeKingdoms/modules/kit/tracex"
	"context"
)

type Account struct {
	userService *app.UserService
	log         logx.Logger
	// 获得默认 grpc 实现，返回客户端一个 Unimplemented 错误，同时避免编译期间没实现所有接口而报错
	accountpb.UnimplementedAccountServiceServer
}

func NewAccount(userRepo app.UserRepo, pwdEncrypter app.PwdEncrypter, log logx.Logger,
	lhRepo app.LoginHistoryRepo, llRepo app.LoginLastRepo, randSeq app.RandSeq) *Account {
	return &Account{
		userService: app.NewUserService(userRepo, pwdEncrypter, lhRepo, llRepo, randSeq),
		log:         log,
	}
}

func (a *Account) Login(ctx context.Context, req *accountpb.LoginRequest) (*accountpb.LoginReply, error) {
	ctx = tracex.WithSpanID(ctx, "account")

	resp, err := a.userService.Login(ctx, req)
	if err != nil {
		logx.ReportSysError(ctx, a.log, logx.NewSysLog("account account tech error", err))
		return nil, toRPCError(err)
	}
	if resp != nil && resp.Result != nil && !resp.Result.Ok {
		logx.ReportBizError(ctx, a.log, logx.NewBizLog("account account reject", resp.Result.Reason, resp.Result.Message))
	}
	return resp, nil
}

func (a *Account) Register(ctx context.Context, req *accountpb.RegisterRequest) (*accountpb.RegisterReply, error) {
	ctx = tracex.WithSpanID(ctx, "account")

	resp, err := a.userService.Register(ctx, req)
	if err != nil {
		logx.ReportSysError(ctx, a.log, logx.NewSysLog("account register tech error", err))
		return nil, toRPCError(err)
	}
	if resp != nil && resp.Result != nil && !resp.Result.Ok {
		logx.ReportBizError(ctx, a.log, logx.NewBizLog("account register reject", resp.Result.Reason, resp.Result.Message))
	}
	return resp, nil
}
