package app

import (
	"ThreeKingdoms/internal/gate/app/model"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"context"
)

type GateService struct {
	accountServiceClient AccountServiceClient
	playerServiceClient  PlayerServiceClient
}

const defaultPlayerWorldID int64 = 1

func checkBizResult(result *commonpb.BizResult) error {
	if result == nil {
		return ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	if !result.Ok {
		return newBizRejectedError(result.Reason, result.Message)
	}
	return nil
}

func NewGateService(accountServiceClient AccountServiceClient, playerServiceClient PlayerServiceClient) *GateService {
	return &GateService{
		accountServiceClient: accountServiceClient,
		playerServiceClient:  playerServiceClient,
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

func (g *GateService) EnterServer(ctx context.Context, reqDTO model.EnterServerReq, seq int64) (*model.EnterServerResp, error) {
	if g.playerServiceClient == nil {
		return nil, ErrUnavailable.WithReason(ReasonUpstreamUnavailable)
	}
	rpcResp, err := g.callPlayer(ctx, &playerpb.PlayerRequest{
		PlayerId: int64(reqDTO.Uid),
		WorldId:  defaultPlayerWorldID,
		Seq:      seq,
		Body: &playerpb.PlayerRequest_EnterServerRequest{
			EnterServerRequest: &playerpb.EnterServerRequest{
				PlayerId: int32(reqDTO.Uid),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	enterResp := rpcResp.GetEnterServerResponse()
	if enterResp == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}

	resp := &model.EnterServerResp{
		Time:  enterResp.Time,
		Token: enterResp.Token,
	}

	if enterResp.Role != nil {
		role := enterResp.Role
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

	if enterResp.Resource != nil {
		resource := enterResp.Resource
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

func (g *GateService) CreateRole(ctx context.Context, uid int, req *playerpb.CreateRoleRequest, seq int64) (*playerpb.CreateRoleResponse, error) {
	if req == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	rpcResp, err := g.callPlayer(ctx, &playerpb.PlayerRequest{
		PlayerId: int64(uid),
		WorldId:  defaultPlayerWorldID,
		Seq:      seq,
		Body: &playerpb.PlayerRequest_CreateRoleRequest{
			CreateRoleRequest: &playerpb.CreateRoleRequest{
				PlayerId: int64(uid),
				NickName: req.NickName,
				Sex:      req.Sex,
				Sid:      req.Sid,
				HeadId:   req.HeadId,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	body := rpcResp.GetCreateRoleResponse()
	if body == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	return body, nil
}

func (g *GateService) WorldMap(ctx context.Context, uid int, seq int64) (*playerpb.WorldMapResponse, error) {
	rpcResp, err := g.callPlayer(ctx, &playerpb.PlayerRequest{
		PlayerId: int64(uid),
		WorldId:  defaultPlayerWorldID,
		Seq:      seq,
		Body: &playerpb.PlayerRequest_WorldMapRequest{
			WorldMapRequest: &playerpb.WorldMapRequest{
				PlayerId: int64(uid),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	body := rpcResp.GetWorldMapResponse()
	if body == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	return body, nil
}

func (g *GateService) MyProperty(ctx context.Context, uid int, seq int64) (*playerpb.MyPropertyResponse, error) {
	rpcResp, err := g.callPlayer(ctx, &playerpb.PlayerRequest{
		PlayerId: int64(uid),
		WorldId:  defaultPlayerWorldID,
		Seq:      seq,
		Body: &playerpb.PlayerRequest_MyPropertyRequest{
			MyPropertyRequest: &playerpb.MyPropertyRequest{
				PlayerId: int32(uid),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	body := rpcResp.GetMyPropertyResponse()
	if body == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	return body, nil
}

func (g *GateService) callPlayer(ctx context.Context, req *playerpb.PlayerRequest) (*playerpb.PlayerResponse, error) {
	if g.playerServiceClient == nil {
		return nil, ErrUnavailable.WithReason(ReasonUpstreamUnavailable)
	}
	if req == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	rpcResp, err := g.playerServiceClient.Handle(ctx, req)
	if err != nil {
		return nil, wrapTechErr(err)
	}
	if rpcResp == nil {
		return nil, ErrInternalServer.WithReason(ReasonUpstreamBadResponse)
	}
	if err = checkBizResult(rpcResp.Result); err != nil {
		return nil, err
	}
	return rpcResp, nil
}
