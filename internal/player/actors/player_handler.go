package actors

import (
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"

	"github.com/asynkron/protoactor-go/actor"
)

type PlayerHandler struct {
}

// 全局实例
var PH = &PlayerHandler{}

func (h *PlayerHandler) HandleEnterServerRequest(ctx actor.Context, p *PlayerActor, request *playerpb.EnterServerRequest) {
	if request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	//playerId := request.GetPlayerId()

	//playerID := p.entity.ID()
	//profile := p.entity.Profile()
	//resource := p.entity.Resource()
	//
	//token, err := security.Award(int(playerID))
	//if err != nil {
	//	ctx.Respond(&messages.FailResp{Code: transport.SessionInvalid, Message: "生成 token 失败"})
	//	return
	//}
	ctx.Respond(ok())
}

func (h *PlayerHandler) HandleMyPropertyRequest(ctx actor.Context, p *PlayerActor, request *playerpb.MyPropertyRequest) {
	if request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	ctx.Respond(ok())
}

func (h *PlayerHandler) HandleMyGeneralsRequest(ctx actor.Context, p *PlayerActor, request *playerpb.MyGeneralsRequest) {
	if request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	ctx.Respond(ok())
}

func ok() *playerpb.PlayerResponse {
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{
			Ok: true,
		},
	}
}

func fail(reason string) *playerpb.PlayerResponse {
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{
			Ok:      false,
			Reason:  reason,
			Message: reason,
		},
	}
}
