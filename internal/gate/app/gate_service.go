package app

import (
	"ThreeKingdoms/internal/gate/app/model"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"context"
)

type GateService struct {
	accountServiceClient AccountServiceClient
}

func NewGateService(accountServiceClient AccountServiceClient) *GateService {
	return &GateService{
		accountServiceClient: accountServiceClient,
	}
}

func (g *GateService) Login(ctx context.Context, loginReqDTO model.LoginReq) (*model.LoginResp, error) {
	if g.accountServiceClient == nil {
		return nil, ErrUnavailable.WithReason(ReasonUpstreamUnavailable)
	}

	rpcReq := accountpb.LoginRequest{
		Username: loginReqDTO.Username,
		Password: loginReqDTO.Password,
		Ip:       loginReqDTO.Ip,
		Hardware: loginReqDTO.Hardware,
	}

	rpcResp, err := g.accountServiceClient.Login(ctx, &rpcReq)
	if err != nil {
		return nil, wrapTechErr(err)
	}
	if rpcResp == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	if !rpcResp.GetOk() {
		return nil, newBizRejectedError(rpcResp.GetReason(), rpcResp.GetMessage())
	}

	return &model.LoginResp{
		Username: rpcResp.Username,
		Session:  rpcResp.Session,
		UId:      int(rpcResp.Uid),
	}, nil
}

func (g *GateService) Register(ctx context.Context, registerReqDTO model.RegisterReq) error {
	if g.accountServiceClient == nil {
		return ErrUnavailable.WithReason(ReasonUpstreamUnavailable)
	}

	rpcReq := accountpb.RegisterRequest{
		Username: registerReqDTO.Username,
		Password: registerReqDTO.Password,
		Hardware: registerReqDTO.Hardware,
	}

	rpcResp, err := g.accountServiceClient.Register(ctx, &rpcReq)
	if err != nil {
		return wrapTechErr(err)
	}
	if rpcResp == nil {
		return ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	if !rpcResp.GetOk() {
		return newBizRejectedError(rpcResp.GetReason(), rpcResp.GetMessage())
	}
	return nil
}
