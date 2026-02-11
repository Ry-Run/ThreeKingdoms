package app

import (
	"ThreeKingdoms/internal/gate/app/model"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	"context"
)

type GateService struct {
	accountServiceClient AccountServiceClient
}

func checkBizResult(result *commonpb.BizResult) error {
	if result == nil {
		return ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	if !result.Ok {
		return newBizRejectedError(result.Reason, result.Message)
	}
	return nil
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
	if err = checkBizResult(rpcResp.Result); err != nil {
		return nil, err
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
	if err = checkBizResult(rpcResp.Result); err != nil {
		return err
	}
	return nil
}

func (g *GateService) EnterServer(ctx context.Context, reqDTO model.EnterServerReq) (*model.EnterServerResp, error) {
	if g.accountServiceClient == nil {
		return nil, ErrUnavailable.WithReason(ReasonUpstreamUnavailable)
	}

	rpcReq := accountpb.EnterServerRequest{
		Uid: int32(reqDTO.Uid),
	}

	rpcResp, err := g.accountServiceClient.EnterServer(ctx, &rpcReq)
	if err != nil {
		return nil, wrapTechErr(err)
	}
	if rpcResp == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	if err = checkBizResult(rpcResp.Result); err != nil {
		return nil, err
	}

	resp := &model.EnterServerResp{
		Time:  rpcResp.Time,
		Token: rpcResp.Token,
	}

	if rpcResp.Role != nil {
		role := rpcResp.Role
		resp.Role = model.Role{
			RId:      int(role.Rid),
			UId:      int(role.Uid),
			NickName: role.NickName,
			Sex:      int8(role.Sex),
			Balance:  int(role.Balance),
			HeadId:   int16(role.HeadId),
			Profile:  role.Profile,
		}
	}

	if rpcResp.Resource != nil {
		resource := rpcResp.Resource
		resp.RoleRes = model.Resource{
			Wood:          int(resource.Wood),
			Iron:          int(resource.Iron),
			Stone:         int(resource.Stone),
			Grain:         int(resource.Grain),
			Gold:          int(resource.Gold),
			Decree:        int(resource.Decree),
			WoodYield:     int(resource.WoodYield),
			IronYield:     int(resource.IronYield),
			StoneYield:    int(resource.StoneYield),
			GrainYield:    int(resource.GrainYield),
			GoldYield:     int(resource.GoldYield),
			DepotCapacity: int(resource.DepotCapacity),
		}
	}

	return resp, nil
}
