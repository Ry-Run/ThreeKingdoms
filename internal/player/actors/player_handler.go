package actors

import (
	"ThreeKingdoms/internal/player/entity"
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
	resp, err := PS.EnterServer(p)
	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}

	player := p.Entity()
	f := ctx.RequestFuture(
		p.worldPID,
		messages.HWCreateCity{
			WorldBaseMessage: messages.WorldBaseMessage{
				PlayerId: int(*p.PlayerId),
				WorldId:  0,
			},
			NickName: player.Profile().NickName(),
		},
		5*time.Second,
	)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}

		if WHPosition, ok := res.(messages.WHCreateCity); ok {
			ctx.Logger().Info("position", "x", WHPosition.X, "y", WHPosition.Y)
		} else {
			ctx.Logger().Info("position", entity.ErrCreateCity)
		}

		ctx.Respond(resp)
	})
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
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}
		var worldMap []*playerpb.Cell
		wHWorldMap, worldOK := res.(messages.WHWorldMap)
		if !worldOK {
			ctx.Respond(fail("world map response invalid"))
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
		response := ok()
		response.Body = &playerpb.PlayerResponse_WorldMapResponse{
			WorldMapResponse: &playerpb.WorldMapResponse{Map: worldMap},
		}
		ctx.Respond(response)
	})
}

func (h *PlayerHandler) HandleMyPropertyRequest(ctx actor.Context, p *PlayerActor, request *playerpb.MyPropertyRequest) {
	resp := PS.MyProperty(p.Entity())
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

func (h *PlayerHandler) HandlePosTagListRequest(ctx actor.Context, p *PlayerActor, request *playerpb.PosTagListRequest) {
	attribute := p.Entity().Attribute()
	tags := make([]*playerpb.PosTag, 0, attribute.LenPosTags())
	attribute.ForEachPosTags(func(i int, v entity.PosTagState) {
		tags = append(tags, &playerpb.PosTag{
			Name: v.Name,
			X:    int32(v.X),
			Y:    int32(v.Y),
		})
	})

	ctx.Respond(&playerpb.PosTagListResponse{PosTags: tags})
}

func (h *PlayerHandler) HandleMyGeneralsRequest(ctx actor.Context, p *PlayerActor, request *playerpb.MyGeneralsRequest) {
	resp, err := PS.GetGenerals(p.Entity())
	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}
	ctx.Respond(resp)
}
