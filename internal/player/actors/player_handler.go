package actors

import (
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/building"
	"ThreeKingdoms/internal/shared/gameconfig/facility"
	"ThreeKingdoms/internal/shared/gameconfig/general"
	_map "ThreeKingdoms/internal/shared/gameconfig/map"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/utils"
	"context"
	"fmt"
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
		&messages.HWCreateCity{
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

		if createCityRes, ok := res.(*messages.WHCreateCity); ok {
			player.SetCityID(CityID(createCityRes.CityId))
			player.City().SetLevel(1)
			player.City().SetX(createCityRes.X)
			player.City().SetY(createCityRes.Y)
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

	f := ctx.RequestFuture(p.worldPID, &messages.HWMyCities{
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
		worldCities, ok := res.(*messages.WHMyCities)
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
	player := p.Entity()
	pbArmies := make([]*playerpb.Army, 0, player.LenArmies())
	player.ForEachArmies(func(i int, v entity.ArmyState) {
		pbArmies = append(pbArmies, ToPBArmy(player.CityID(), v))
	})

	ctx.Respond(&playerpb.ArmyListResponse{
		CityId: request.CityId,
		Armies: pbArmies,
	})
}

func (h *PlayerHandler) HandleWarReportRequest(ctx actor.Context, p *PlayerActor, request *playerpb.WarReportRequest) {
	player := p.Entity()
	warReports := make([]*playerpb.WarReport, 0, player.LenWarReports())
	player.ForEachWarReports(func(i int, v entity.WarReportState) {
		warReports = append(warReports, ToPBWarReport(player.CityID(), v))
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
		&messages.HWScanBlock{
			WorldBaseMessage: messages.WorldBaseMessage{
				WorldId:  int(*p.WorldId),
				PlayerId: int(*p.PlayerId),
			},
			X:      x,
			Y:      y,
			Length: int(request.Length),
		}, 500*time.Millisecond)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}
		wHScanBlock, respOK := res.(*messages.WHScanBlock)
		if !respOK {
			ctx.Respond(fail("invalid response type"))
			return
		}
		response := ok()
		response.Body = &playerpb.PlayerResponse_ScanBlockResponse{
			ScanBlockResponse: ToPBWHScanBlock(*wHScanBlock),
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

func (h *PlayerHandler) HandleAllianceListRequest(ctx actor.Context, p *PlayerActor, request *playerpb.AllianceListRequest) {
	if p == nil || p.WorldId == nil || p.alliancePID == nil {
		ctx.Respond(okWithAllianceList(nil))
		return
	}
	req := &messages.HAAllianceList{
		AllianceBaseMessage: messages.AllianceBaseMessage{
			WorldId: int(*p.WorldId),
		},
	}
	f := ctx.RequestFuture(p.alliancePID, req, 500*time.Millisecond)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}
		switch msg := res.(type) {
		case *messages.AHAllianceList:
			if msg == nil {
				ctx.Respond(okWithAllianceList(nil))
				return
			}
			ctx.Respond(okWithAllianceList(msg.List))
		default:
			ctx.Respond(fail("invalid alliance list response type"))
		}
	})
}

func (h *PlayerHandler) HandleAllianceInfoRequest(ctx actor.Context, p *PlayerActor, request *playerpb.AllianceInfoRequest) {
	if p == nil || p.WorldId == nil || p.alliancePID == nil || request == nil || request.AllianceId <= 0 {
		ctx.Respond(fail("request parameter error"))
		return
	}

	req := &messages.HAAllianceInfo{
		AllianceBaseMessage: messages.AllianceBaseMessage{
			WorldId:    int(*p.WorldId),
			AllianceId: int(request.AllianceId),
		},
	}
	f := ctx.RequestFuture(p.alliancePID, req, 500*time.Millisecond)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}
		switch msg := res.(type) {
		case *messages.AHAllianceInfo:
			if msg == nil || msg.Alliance.Id <= 0 {
				ctx.Respond(fail("alliance not found"))
				return
			}
			response := ok()
			response.Body = &playerpb.PlayerResponse_AllianceInfoResponse{
				AllianceInfoResponse: &playerpb.AllianceInfoResponse{
					Alliance: toPBAlliance(msg.Alliance),
				},
			}
			ctx.Respond(response)
		default:
			ctx.Respond(fail("invalid alliance response type"))
		}
	})
}

func (h *PlayerHandler) HandleAllianceApplyListRequest(ctx actor.Context, p *PlayerActor, request *playerpb.AllianceApplyListRequest) {
	if p == nil || p.WorldId == nil || p.alliancePID == nil || request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	if p.AllianceID == nil || *p.AllianceID <= 0 {
		ctx.Respond(okWithAllianceApplyList(nil))
		return
	}

	req := &messages.HAAllianceApplyList{
		AllianceBaseMessage: messages.AllianceBaseMessage{
			WorldId:    int(*p.WorldId),
			AllianceId: int(*p.AllianceID),
		},
	}
	f := ctx.RequestFuture(p.alliancePID, req, 500*time.Millisecond)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}
		switch msg := res.(type) {
		case *messages.AHAllianceApplyList:
			if msg == nil {
				ctx.Respond(okWithAllianceApplyList(nil))
				return
			}
			ctx.Respond(okWithAllianceApplyList(msg.ApplyItem))
		default:
			ctx.Respond(fail("invalid alliance response type"))
		}
	})
}

func (h *PlayerHandler) HandleDrawGeneralRequest(ctx actor.Context, p *PlayerActor, request *playerpb.DrawGeneralRequest) {
	//1. 计算抽卡花费的金钱
	//2. 判断金钱是否足够
	//3. 抽卡的次数 + 已有的武将 卡池是否足够
	//4. 随机生成武将即可（之前有实现）
	//5. 金币的扣除
	if request == nil || p == nil || p.Entity() == nil || p.Entity().Resource() == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}

	player := p.Entity()
	drawTimes := int(request.DrawTimes)
	if drawTimes <= 0 {
		ctx.Respond(fail("invalid draw times"))
		return
	}

	resource := player.Resource()
	conf := basic.BasicConf.General
	cost := conf.DrawGeneralCost
	if cost <= 0 {
		ctx.Respond(fail("invalid draw general cost config"))
		return
	}
	totalCost := drawTimes * cost

	if !resource.IsEnoughGold(totalCost) {
		ctx.Respond(fail("not enough gold"))
		return
	}

	limit := conf.Limit
	if player.LenGenerals()+drawTimes > limit {
		ctx.Respond(fail("too many general"))
		return
	}
	generals, err := draw(drawTimes)
	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}

	var dirty bool
	for _, g := range generals {
		dirty = player.PutGenerals(g.Id, g) || dirty
	}

	// 扣钱
	dirty = resource.SetGold(resource.Gold()-totalCost) || dirty

	if dirty {
		// todo 玩家 asset、resource 等需要强一致，暂时不提供强一致 API
		// 等待脏数据落库
		if err := p.DC().FlushSync(context.TODO()); err != nil {
			ctx.Respond(fail("flush player failed"))
			return
		}
	}

	response := ok()
	pbGenerals := make([]*playerpb.General, 0, len(generals))
	for _, v := range generals {
		pbGenerals = append(pbGenerals, ToPBGeneral(v))
	}
	response.Body = &playerpb.PlayerResponse_DrawGeneralResponse{
		DrawGeneralResponse: &playerpb.DrawGeneralResponse{
			Generals: pbGenerals,
		},
	}
	ctx.Respond(response)
}

func (h *PlayerHandler) HandleFacilitiesRequest(ctx actor.Context, p *PlayerActor, request *playerpb.FacilitiesRequest) {
	if p == nil || request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	player := p.Entity()
	if player == nil {
		ctx.Respond(fail("player state invalid"))
		return
	}
	facilities := make([]*playerpb.Facility, 0, player.LenFacility())
	player.ForEachFacility(func(i int, v entity.FacilityState) {
		facilities = append(facilities, toPBFacility(v))
	})

	response := ok()
	response.Body = &playerpb.PlayerResponse_FacilitiesResponse{
		FacilitiesResponse: &playerpb.FacilitiesResponse{
			CityId:     int32(player.CityID()),
			Facilities: facilities,
		},
	}
	ctx.Respond(response)
}

func (h *PlayerHandler) HandleUpFacilityRequest(ctx actor.Context, p *PlayerActor, request *playerpb.UpFacilityRequest) {
	if p == nil || request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	player := p.Entity()
	if player == nil || player.Resource() == nil {
		ctx.Respond(fail("player state invalid"))
		return
	}

	playerCityID := int32(player.CityID())
	if request.GetCityId() <= 0 || request.GetCityId() != playerCityID {
		ctx.Respond(fail("city id mismatch"))
		return
	}

	fType := int8(request.GetFType())
	nowMS := time.Now().UnixMilli()

	var (
		err           error
		found         bool
		dirty         bool
		facilityState entity.FacilityState
	)

	player.RangeFacility(func(i int, v entity.FacilityState) bool {
		if v.FType != fType {
			return true
		}
		found = true

		maxLevel := facility.FacilityConf.MaxLevel(fType)
		if maxLevel <= 0 {
			err = fmt.Errorf("facility config not found")
			return false
		}

		// 结算已经完成但尚未清理的升级进度，避免永久卡在升级中状态。
		if v.UpTime > 0 {
			if v.UpTime > nowMS {
				err = fmt.Errorf("facility is upgrading")
				return false
			}
			if v.PrivateLevel < maxLevel {
				dirty = player.UpdateFacilityAt(i, func(fe *entity.FacilityEntity) {
					fe.SetPrivateLevel(v.PrivateLevel + 1)
					fe.SetUpTime(0)
				}) || dirty
				v.PrivateLevel++
			} else {
				dirty = player.UpdateFacilityAt(i, func(fe *entity.FacilityEntity) {
					fe.SetUpTime(0)
				}) || dirty
			}
			v.UpTime = 0
		}

		if v.PrivateLevel >= maxLevel {
			err = fmt.Errorf("facility level max")
			return false
		}

		nextLevel := v.PrivateLevel + 1
		cfg, ok := facility.FacilityConf.GetFacility(fType)
		if !ok || cfg == nil {
			err = fmt.Errorf("facility config not found")
			return false
		}
		levelCfg, ok := cfg.LevelMap[nextLevel]
		if !ok {
			err = fmt.Errorf("facility level config not found")
			return false
		}

		cost := entity.ResourceState{
			Wood:   levelCfg.Need.Wood,
			Iron:   levelCfg.Need.Iron,
			Stone:  levelCfg.Need.Stone,
			Grain:  levelCfg.Need.Grain,
			Gold:   levelCfg.Need.Gold,
			Decree: levelCfg.Need.Decree,
		}
		if !Consume(player.Resource(), cost) {
			err = fmt.Errorf("resource is not enough")
			return false
		}

		dirty = player.UpdateFacilityAt(i, func(fe *entity.FacilityEntity) {
			fe.SetUpTime(nowMS)
		}) || dirty
		facilityState, _ = player.AtFacility(i)
		return false
	})

	if err != nil {
		ctx.Respond(fail(err.Error()))
		return
	}

	if !found {
		ctx.Respond(fail("facility not found"))
		return
	}

	if dirty {
		_ = p.DC().FlushSync(context.TODO())
	}

	response := ok()
	response.Body = &playerpb.PlayerResponse_UpFacilityResponse{
		UpFacilityResponse: &playerpb.UpFacilityResponse{
			CityId:   playerCityID,
			Facility: toPBFacility(facilityState),
			Resource: ToPBResource(player.Resource()),
		},
	}
	ctx.Respond(response)
}

func (h *PlayerHandler) HandleTransformRequest(ctx actor.Context, p *PlayerActor, request *playerpb.TransformRequest) {
	player := p.Entity()
	if f, ok := player.AtFacility(facility.JiShi); !ok || f.PrivateLevel <= 0 {
		ctx.Respond(fail("f is unlock"))
		return
	}

	from := make([]int, 0, 4)
	to := make([]int, 0, 4)
	for _, v := range request.From {
		from = append(from, int(v))
	}
	for _, v := range request.To {
		to = append(to, int(v))
	}

	isEnough := Consume(player.Resource(), entity.ResourceState{
		Wood:  from[0],
		Iron:  from[1],
		Stone: from[2],
		Grain: from[3],
	})

	if !isEnough {
		ctx.Respond(fail("resource not enough"))
		return
	}

	Gain(player.Resource(), entity.ResourceState{
		Wood:  to[0],
		Iron:  to[1],
		Stone: to[2],
		Grain: to[3],
	})

	_ = p.DC().FlushSync(context.TODO())
	response := ok()
	response.Body = &playerpb.PlayerResponse_UpFacilityResponse{
		UpFacilityResponse: &playerpb.UpFacilityResponse{},
	}
	ctx.Respond(response)
}

func (h *PlayerHandler) HandleDisposeRequest(ctx actor.Context, p *PlayerActor, request *playerpb.DisposeRequest) {
	order := int(request.Order)
	position := int(request.Position)
	generalId := int(request.GeneralId)
	if order <= 0 || order > 5 || position < -1 || position > 2 || generalId <= 0 {
		ctx.Respond(fail("request param is invalid"))
		return
	}
	// 校场每升一级一个队伍
	player := p.Entity()
	if f, b := player.AtFacility(facility.JiaoChang); !b || f.PrivateLevel < order {
		ctx.Respond(fail("order is unlock"))
		return
	}

	opGeneral, b := player.GetGenerals(generalId)
	if !b {
		ctx.Respond(fail("generals not found"))
		return
	}

	army, b := player.GetArmies(order)
	x := player.City().X()
	y := player.City().Y()
	if !b {
		army.Id = order
		army = entity.ArmyState{
			Id:                order,
			Order:             int8(order),
			CityId:            player.CityID(),
			Generals:          []int{0, 0, 0},
			Soldiers:          []int{0, 0, 0},
			ConscriptCounts:   []int{0, 0, 0},
			ConscriptEndTimes: []int64{0, 0, 0},
			CellX:             x,
			CellY:             y,
			FromX:             x,
			FromY:             y,
			ToX:               x,
			ToY:               y,
		}
	}

	// 判断 army 是否在城内
	//if army.CellX != x || army.CellY != y {
	//	ctx.Respond(fail("army dont in city"))
	//	return
	//}

	if army.ToX != 0 || army.ToY != 0 {
		ctx.Respond(fail("army dont in city"))
		return
	}

	//下阵
	if position == -1 {
		for pos, g := range army.Generals {
			if g != 0 && g == opGeneral.Id {
				// 征兵中不能下阵
				if !PositionCanModify(army, pos) {
					if army.Cmd == entity.ArmyCmdConscript {
						ctx.Respond(fail("generals busy"))
					} else {
						ctx.Respond(fail("army busy"))
					}
					return
				}
				player.UpdateArmies(army.Id, func(value *entity.ArmyEntity) {
					value.SetGeneralsAt(pos, 0)
					value.SetSoldiersAt(pos, 0)
					value.SetCityId(0)
				})
				break
			}
		}
		player.UpdateGenerals(opGeneral.Id, func(value *entity.GeneralEntity) {
			value.SetCurArms(0)
			value.SetOrder(0)
		})
	} else {
		//征兵中不能上阵
		if !PositionCanModify(army, position) == false {
			if army.Cmd == entity.ArmyCmdConscript {
				ctx.Respond(fail("generals busy"))
			} else {
				ctx.Respond(fail("army busy"))
			}
			return
		}
		if opGeneral.CityId != 0 {
			ctx.Respond(fail("generals busy"))
			return
		}
		if GeneralIsRepeat(player, army, opGeneral.CfgId) {
			ctx.Respond(fail("generals repeat"))
			return
		}
		// position == 2 是前锋，判断是否能配前锋
		var err error
		if position == 2 {
			player.RangeFacility(func(i int, v entity.FacilityState) bool {
				if v.FType == facility.TongShuaiTing {
					if v.PrivateLevel <= 0 {
						err = fmt.Errorf("TongShuaiTing is unlock")
					}
					return false
				}
				return true
			})
		}

		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}

		//判断 cost。点数不足不能上阵
		cost := general.General.Cost(opGeneral.CfgId)
		for i, g := range army.Generals {
			curGeneral, b := player.GetGenerals(g)
			if !b || g == 0 || i == position {
				continue
			}
			cost += general.General.Cost(curGeneral.CfgId)
		}

		if GetCost(player) < cost {
			ctx.Respond(fail("cost is insufficient"))
			return
		}

		oldG := army.Generals[position]
		if oldG != 0 {
			//旧的下阵
			player.UpdateGenerals(oldG, func(value *entity.GeneralEntity) {
				value.SetCurArms(0)
				value.SetOrder(0)
				value.SetCityId(0)
			})
		}

		//新的上阵
		army.Generals[position] = opGeneral.Id
		army.Soldiers[position] = 0

		player.UpdateGenerals(opGeneral.Id, func(value *entity.GeneralEntity) {
			value.SetCurArms(order)
			value.SetOrder(int8(order))
			value.SetCityId(int(request.CityId))
		})

		player.UpdateArmies(army.Id, func(value *entity.ArmyEntity) {
			value.SetFromX(player.City().X())
			value.SetFromY(player.City().Y())
		})
	}

	army, _ = player.GetArmies(order)
	response := ok()
	response.Body = &playerpb.PlayerResponse_DisposeResponse{
		DisposeResponse: &playerpb.DisposeResponse{
			Army: ToPBArmy(player.CityID(), army),
		},
	}
	ctx.Respond(response)
}

func draw(times int) ([]entity.GeneralState, error) {
	if times <= 0 {
		return nil, fmt.Errorf("invalid draw times")
	}
	generals := make([]entity.GeneralState, 0, times)
	for i := 0; i < times; i++ {
		cfgId := general.Rand()
		if cfgId <= 0 {
			return nil, fmt.Errorf("draw general config not ready")
		}
		id, err := utils.NextSnowflakeID()
		if err != nil {
			return nil, fmt.Errorf("generate general id failed: %w", err)
		}
		generalS := entity.GeneralState{
			Id:    int(id),
			CfgId: cfgId,
			Level: 1,
		}
		generals = append(generals, generalS)
	}
	return generals, nil
}

func PositionCanModify(a entity.ArmyState, pos int) bool {
	if pos >= 3 || pos < 0 {
		return false
	}

	if a.Cmd == entity.ArmyCmdIdle {
		return true
	} else if a.Cmd == entity.ArmyCmdConscript {
		endTime := a.ConscriptEndTimes[pos]
		return endTime == 0
	}

	return false
}

func GeneralIsRepeat(p *entity.PlayerEntity, a entity.ArmyState, cfgId int) bool {
	if p == nil {
		return true
	}
	for _, g := range a.Generals {
		generalState, ok := p.GetGenerals(g)
		if ok && generalState.CfgId == cfgId {
			return true
		}
	}
	return false
}

func GetCost(p *entity.PlayerEntity) int8 {
	cost := 0
	p.ForEachFacility(func(i int, v entity.FacilityState) {
		if v.PrivateLevel > 0 {
			f, ok := facility.FacilityConf.GetFacility(v.FType)
			if ok {
				level, ok := f.LevelMap[v.PrivateLevel]
				if ok {
					for i, fType := range f.Additions {
						if fType == facility.TypeCost {
							cost += level.Values[i]
						}
					}
				}
			}
		}
	})
	return int8(cost)
}

func okWithAllianceList(list []messages.Alliance) *playerpb.PlayerResponse {
	pbList := make([]*playerpb.Alliance, 0, len(list))
	for _, item := range list {
		pbList = append(pbList, toPBAlliance(item))
	}
	resp := ok()
	resp.Body = &playerpb.PlayerResponse_AllianceListResponse{
		AllianceListResponse: &playerpb.AllianceListResponse{
			List: pbList,
		},
	}
	return resp
}

func okWithAllianceApplyList(items []messages.ApplyItem) *playerpb.PlayerResponse {
	pbItems := make([]*playerpb.ApplyItem, 0, len(items))
	for _, item := range items {
		pbItems = append(pbItems, toPBApplyItem(item))
	}
	resp := ok()
	resp.Body = &playerpb.PlayerResponse_AllianceApplyListResponse{
		AllianceApplyListResponse: &playerpb.AllianceApplyListResponse{
			Item: pbItems,
		},
	}
	return resp
}

func toPBAlliance(in messages.Alliance) *playerpb.Alliance {
	majors := make([]*playerpb.Major, 0, len(in.Major))
	for _, major := range in.Major {
		if major == nil {
			continue
		}
		majors = append(majors, &playerpb.Major{
			Rid:   major.Rid,
			Name:  major.Name,
			Title: toPBAllianceTitle(major.Title),
		})
	}
	return &playerpb.Alliance{
		Id:     in.Id,
		Name:   in.Name,
		Cnt:    in.Cnt,
		Notice: in.Notice,
		Major:  majors,
	}
}

func toPBApplyItem(in messages.ApplyItem) *playerpb.ApplyItem {
	return &playerpb.ApplyItem{
		PlayerId: int32(in.PlayerId),
		NickName: in.NickName,
	}
}

func toPBAllianceTitle(in messages.AllianceTitle) playerpb.AllianceTitle {
	switch in {
	case messages.ALLIANCE_CHAIRMAN:
		return playerpb.AllianceTitle_ALLIANCE_CHAIRMAN
	case messages.ALLIANCE_VICE_CHAIRMAN:
		return playerpb.AllianceTitle_ALLIANCE_VICE_CHAIRMAN
	case messages.ALLIANCE_COMMON:
		return playerpb.AllianceTitle_ALLIANCE_COMMON
	default:
		return playerpb.AllianceTitle_ALLIANCE_COMMON
	}
}

func toPBFacility(in entity.FacilityState) *playerpb.Facility {
	return &playerpb.Facility{
		Name:   in.Name,
		Level:  int32(in.PrivateLevel),
		Type:   int32(in.FType),
		UpTime: in.UpTime,
	}
}

func IsEnough(r *entity.ResourceEntity, cost entity.ResourceState) bool {
	return r.Wood() >= cost.Wood &&
		r.Iron() >= cost.Iron &&
		r.Stone() >= cost.Stone &&
		r.Grain() >= cost.Grain &&
		r.Gold() >= cost.Gold &&
		r.Decree() >= cost.Decree
}

func Consume(r *entity.ResourceEntity, cost entity.ResourceState) bool {
	enough := IsEnough(r, cost)
	if !enough {
		return false
	}
	r.SetWood(r.Wood() - cost.Wood)
	r.SetIron(r.Iron() - cost.Iron)
	r.SetStone(r.Stone() - cost.Stone)
	r.SetGrain(r.Grain() - cost.Grain)
	r.SetGold(r.Gold() - cost.Gold)
	r.SetDecree(r.Decree() - cost.Decree)
	return true
}

func Gain(r *entity.ResourceEntity, gain entity.ResourceState) bool {
	r.SetWood(r.Wood() + gain.Wood)
	r.SetIron(r.Iron() + gain.Iron)
	r.SetStone(r.Stone() + gain.Stone)
	r.SetGrain(r.Grain() + gain.Grain)
	r.SetGold(r.Gold() + gain.Gold)
	r.SetDecree(r.Decree() + gain.Decree)
	return true
}
