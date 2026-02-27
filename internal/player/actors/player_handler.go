package actors

import (
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/building"
	_map "ThreeKingdoms/internal/shared/gameconfig/map"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type PlayerHandler struct {
}

// type PlayerID = entity.PlayerID
// type WorldID = entity.WorldID
type CityID = entity.CityID

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
	if resp == nil {
		ctx.Respond(fail("enterServer response is nil"))
		return
	}
	if result := resp.GetResult(); result != nil && !result.GetOk() {
		ctx.Respond(resp)
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
			ctx.Logger().Info("position", "err", entity.ErrCreateCity)
		}

		ctx.Respond(resp)
	})
}

func (h *PlayerHandler) HandleCreateRole(ctx actor.Context, p *PlayerActor, request *playerpb.CreateRoleRequest) {
	if request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	resp, err := PS.CreateRole(p, request)
	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}
	ctx.Respond(resp)
}

func (h *PlayerHandler) HandleBuildingConfRequest(ctx actor.Context, p *PlayerActor, request *playerpb.BuildingConfRequest) {
	buildingConf := building.BuildingConf
	buildingCfgs := make([]*playerpb.BuildingCfg, 0, len(buildingConf.Cfgs))
	for _, v := range buildingConf.Cfgs {
		pbCell := playerpb.BuildingCfg{
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
		buildingCfgs = append(buildingCfgs, &pbCell)
	}
	response := ok()
	response.Body = &playerpb.PlayerResponse_BuildingConfResponse{
		BuildingConfResponse: &playerpb.BuildingConfResponse{
			Cfgs: buildingCfgs,
		},
	}
	ctx.Respond(response)
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
				PlayerId:   int32(*p.PlayerId),
				CityId:     c.CityId,
				Name:       c.Name,
				UnionId:    int32(c.UnionId),
				UnionName:  c.UnionName,
				ParentId:   int32(c.ParentId),
				X:          int32(c.Pos.X),
				Y:          int32(c.Pos.Y),
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

func (h *PlayerHandler) HandleArmyListRequest(ctx actor.Context, p *PlayerActor, request *playerpb.ArmyListRequest) {
	armies, _ := p.Entity().GetArmies(CityID(request.CityId))
	pbArmies := make([]*playerpb.Army, 0, len(armies))
	for _, v := range armies {
		pbArmies = append(pbArmies, ToPBArmy(v))
	}
	ctx.Respond(&playerpb.ArmyListResponse{
		CityId: request.CityId,
		Armies: pbArmies,
	})
}

func (h *PlayerHandler) HandleWarReportRequest(ctx actor.Context, p *PlayerActor, request *playerpb.WarReportRequest) {
	player := p.Entity()
	warReports := make([]*playerpb.WarReport, 0, player.LenWarReports())
	player.ForEachWarReports(func(i int, v entity.WarReportState) {
		warReports = append(warReports, ToPBWarReport(v))
	})
	ctx.Respond(&playerpb.WarReportResponse{
		WarReports: warReports,
	})
}

func (h *PlayerHandler) HandleSkillListRequest(ctx actor.Context, p *PlayerActor, request *playerpb.SkillListRequest) {
	player := p.Entity()
	skills := make([]*playerpb.Skill, 0, player.LenSkills())
	player.ForEachSkills(func(i int, v entity.SkillState) {
		skills = append(skills, ToPBSkill(v))
	})
	ctx.Respond(&playerpb.SkillListResponse{
		Skills: skills,
	})
}

func (h *PlayerHandler) HandleScanBlockRequest(ctx actor.Context, p *PlayerActor, request *playerpb.ScanBlockRequest) {
	x, y := int(request.X), int(request.Y)
	if x < 0 || x >= _map.MapWidth || y < 0 || y >= _map.MapHeight {
		ctx.Respond(fail("request param err"))
		return
	}

	f := ctx.RequestFuture(p.worldPID,
		messages.HWScanBlock{
			X:      x,
			Y:      y,
			Length: int(request.Length),
		}, 500*time.Millisecond)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}
		wHScanBlock, respOK := res.(messages.WHScanBlock)
		if !respOK {
			ctx.Respond(fail("invalid response type"))
			return
		}
		response := ok()
		response.Body = &playerpb.PlayerResponse_ScanBlockResponse{
			ScanBlockResponse: ToPBWHScanBlock(wHScanBlock),
		}
		ctx.Respond(response)
	})
}

func (h *PlayerHandler) HandleOpenCollectionRequest(ctx actor.Context, p *PlayerActor, request *playerpb.OpenCollectionRequest) {
	interval := basic.BasicConf.Role.CollectInterval
	limit := basic.BasicConf.Role.CollectTimesLimit
	if p == nil || p.Entity() == nil || p.Entity().Attribute() == nil {
		ctx.Respond(fail("player attribute not initialized"))
		return
	}
	attribute := p.Entity().Attribute()
	collectTimes := attribute.CollectTimes()

	lastCollectTime := attribute.LastCollectTime()
	now := time.Now()
	intervalMills := int64(interval * 1000)

	nextTime := int64(-1)
	if !IsSameDayCST(now, lastCollectTime) {
		attribute.SetLastCollectTime(TodayZeroCST(now))
		attribute.SetCollectTimes(0)
		nextTime = 0
		collectTimes = 0
	} else if collectTimes >= limit {
		nextTime = NextCSTMidnight(now).UnixMilli()
	} else {
		nextTime = lastCollectTime.UnixMilli() + intervalMills
		if collectTimes == 0 {
			nextTime = 0
		}
	}

	response := ok()
	response.Body = &playerpb.PlayerResponse_OpenCollectionResponse{
		OpenCollectionResponse: &playerpb.OpenCollectionResponse{
			Limit:    int32(limit),
			CurTimes: int32(collectTimes),
			NextTime: nextTime,
		},
	}
	ctx.Respond(response)
}

func (h *PlayerHandler) HandleCollectionRequest(ctx actor.Context, p *PlayerActor, request *playerpb.CollectionRequest) {
	player := p.Entity()
	interval := basic.BasicConf.Role.CollectInterval
	limit := basic.BasicConf.Role.CollectTimesLimit
	if p == nil || player == nil || player.Attribute() == nil {
		ctx.Respond(fail("player attribute not initialized"))
		return
	}
	attribute := player.Attribute()
	collectTimes := attribute.CollectTimes()

	if collectTimes >= limit {
		ctx.Respond(fail("collect times limit exceeded"))
		return
	}

	lastCollectTime := attribute.LastCollectTime()
	now := time.Now()
	nextCollectTime := lastCollectTime.Add(time.Duration(interval) * time.Second)

	if collectTimes != 0 && nextCollectTime.After(now) {
		ctx.Respond(fail("in cd can not operate"))
		return
	}

	// 最终的产量 = 建筑 + 城池设施收益

	// 城池设施收益
	yield := ComputeFacilityYield(player)

	// 更新状态
	attribute.SetLastCollectTime(now)
	collectTimes = collectTimes + 1
	attribute.SetCollectTimes(collectTimes)
	gold := player.Resource().Gold() + yield.Gold
	player.Resource().SetGold(gold)

	// 下一次领取时间
	nextTime := now.Add(time.Duration(interval) * time.Second).UnixMilli()

	response := ok()
	response.Body = &playerpb.PlayerResponse_CollectionResponse{
		CollectionResponse: &playerpb.CollectionResponse{
			Gold:     int32(gold),
			Limit:    int32(limit),
			CurTimes: int32(collectTimes),
			NextTime: nextTime,
		},
	}
	ctx.Respond(response)
}
