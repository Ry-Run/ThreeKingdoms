package actors

import (
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/player/service"
	"ThreeKingdoms/internal/shared/actor/messages"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"context"
	"time"

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
	resp, err := service.PS.EnterServer(p.Entity())
	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}
	ctx.Respond(resp)
}

func (h *PlayerHandler) HandleCreateRole(ctx actor.Context, p *PlayerActor, request *playerpb.CreateRoleRequest) {
	if request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}

	role := entity.RoleState{
		Headid:    int16(request.HeadId),
		Sex:       int8(request.Sex),
		NickName:  request.NickName,
		CreatedAt: time.Now(),
	}

	p.Entity().SetProfile(role)

	err := p.dc.FlushSync(context.TODO())

	if err != nil {
		ctx.Respond(fail("request parameter error"))
		return
	}

	response := ok()
	response.Body = &playerpb.PlayerResponse_CreateRoleResponse{
		CreateRoleResponse: &playerpb.CreateRoleResponse{
			Role: &playerpb.Role{
				NickName: role.NickName,
				Sex:      int32(role.Sex),
				Balance:  int32(role.Balance),
				HeadId:   int32(role.Headid),
				Profile:  role.Profile,
			},
		},
	}
	ctx.Respond(response)
}

func (h *PlayerHandler) HandleWorldMapRequest(ctx actor.Context, p *PlayerActor, request *playerpb.WorldMapRequest) {
	f := ctx.RequestFuture(p.worldPID, messages.HWWorldMap{
		WorldBaseMessage: messages.WorldBaseMessage{
			WorldId:  int(*p.WorldId),
			PlayerId: int(*p.PlayerId),
		},
	}, time.Millisecond*500)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		var worldMap []*playerpb.Cell
		wHWorldMap, ok := res.(messages.WHWorldMap)
		if !ok {
			return
		}
		cells := wHWorldMap.WorldMap
		worldMap = make([]*playerpb.Cell, 0, len(cells))
		for _, v := range cells {
			pbCell := playerpb.Cell{
				Type:     int32(v.Type),
				Name:     v.Name,
				Level:    int32(v.Level),
				Defender: int64(v.Defender),
				Durable:  int64(v.Durable),
				Grain:    int64(v.Grain),
				Iron:     int64(v.Iron),
				Stone:    int64(v.Stone),
				Wood:     int64(v.Wood),
			}
			worldMap = append(worldMap, &pbCell)
		}
		ctx.Respond(playerpb.WorldMapResponse{
			Map: worldMap,
		})
	})
}

func (h *PlayerHandler) HandleMyPropertyRequest(ctx actor.Context, p *PlayerActor, request *playerpb.MyPropertyRequest) {
	resp := service.PS.MyProperty(p.Entity())
	if resp == nil {
		ctx.Respond(fail("myProperty response is nil"))
		return
	}

	f := ctx.RequestFuture(p.worldPID, messages.HWMyCities{
		WorldBaseMessage: messages.WorldBaseMessage{
			WorldId:  int(*p.WorldId),
			PlayerId: int(*p.PlayerId),
		},
	}, 500*time.Millisecond)

	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			// world 查询失败时降级返回 player 私域数据，避免整包失败。
			ctx.Respond(resp)
			return
		}
		worldCities, ok := res.(messages.WHMyCities)
		if !ok {
			ctx.Respond(resp)
			return
		}

		body := resp.GetMyPropertyResponse()
		if body == nil {
			ctx.Respond(resp)
			return
		}
		body.Cities = make([]*playerpb.City, 0, len(worldCities.Cities))
		for _, c := range worldCities.Cities {
			body.Cities = append(body.Cities, &playerpb.City{
				Name:       c.Name,
				UnionId:    int32(c.UnionId),
				UnionName:  c.UnionName,
				ParentId:   int32(c.ParentId),
				X:          int32(c.X),
				Y:          int32(c.Y),
				IsMain:     c.IsMain,
				Level:      int32(c.Level),
				CurDurable: int32(c.CurDurable),
				MaxDurable: int32(c.MaxDurable),
				OccupyTime: c.OccupyTime,
			})
		}
		ctx.Respond(resp)
	})
}

func (h *PlayerHandler) HandleMyGeneralsRequest(ctx actor.Context, p *PlayerActor, request *playerpb.MyGeneralsRequest) {
	resp, err := service.PS.MyGenerals(request)
	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}
	ctx.Respond(resp)
}
