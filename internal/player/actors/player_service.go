package actors

import (
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/facility"
	"ThreeKingdoms/internal/shared/gameconfig/general"
	_map "ThreeKingdoms/internal/shared/gameconfig/map"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/security"
	"ThreeKingdoms/internal/shared/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type PlayerService struct{}

var PS = &PlayerService{}

func (s *PlayerService) EnterServer(p *PlayerActor) (*playerpb.PlayerResponse, error) {
	player := p.Entity()
	if player == nil {
		return nil, errors.New("player not loaded")
	}
	if !s.hasCreatedRole(player) {
		return failRoleNotExist(), nil
	}

	if err := s.initPlayer(p); err != nil {
		// 暂时忽略 flushSync 的 err
	}

	token, err := security.Award(int(player.PlayerID()))
	if err != nil {
		return nil, err
	}
	role := ToPBRole(player.Profile())
	role.Rid = int32(player.PlayerID())
	role.Uid = int32(player.PlayerID())

	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{Ok: true},
		Body: &playerpb.PlayerResponse_EnterServerResponse{
			EnterServerResponse: &playerpb.EnterServerResponse{
				Role:       role,
				Resource:   ToPBResource(player.Resource()),
				Token:      token,
				Time:       time.Now().UnixNano() / 1e6,
				AllianceId: int32(player.AllianceID()),
			},
		},
	}, nil
}

func (s *PlayerService) CreateRole(p *PlayerActor, request *playerpb.CreateRoleRequest) (*playerpb.PlayerResponse, error) {
	if p == nil || request == nil {
		return fail("request parameter error"), nil
	}
	player := p.Entity()
	if player == nil {
		return nil, errors.New("player not loaded")
	}
	if s.hasCreatedRole(player) {
		return fail("role already created"), nil
	}

	now := time.Now()
	needFlush := false
	needFlush = player.SetProfile(entity.RoleState{
		Headid:    int16(request.HeadId),
		Sex:       int8(request.Sex),
		NickName:  request.NickName,
		Balance:   0,
		LoginTime: now,
		CreatedAt: now,
	})
	if needFlush {
		if err := p.DC().FlushSync(context.TODO()); err != nil {
			return nil, err
		}
	}

	role := ToPBRole(player.Profile())
	role.Rid = int32(player.PlayerID())
	role.Uid = int32(player.PlayerID())

	resp := ok()
	resp.Body = &playerpb.PlayerResponse_CreateRoleResponse{
		CreateRoleResponse: &playerpb.CreateRoleResponse{
			Role: role,
		},
	}
	return resp, nil
}

func (s *PlayerService) MyProperty(player *entity.PlayerEntity) *playerpb.PlayerResponse {
	//建筑
	buildings := make([]*playerpb.Building, 0, player.LenBuildings())
	player.ForEachBuildings(func(i int, v entity.BuildingState) {
		buildings = append(buildings, ToPBBuilding(v))
	})
	//资源
	resource := ToPBResource(player.Resource())
	//武将
	generals := make([]*playerpb.General, 0, player.LenGenerals())
	player.ForEachGenerals(func(i int, v entity.GeneralState) {
		generals = append(generals, ToPBGeneral(v))
	})
	//军队
	armies := make([]*playerpb.Army, 0, player.LenArmies())
	player.ForEachArmies(func(k int, v entity.ArmyState) {
		armies = append(armies, ToPBArmy(player.CityID(), v))
	})

	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{Ok: true},
		Body: &playerpb.PlayerResponse_MyPropertyResponse{
			MyPropertyResponse: &playerpb.MyPropertyResponse{
				Resource:  resource,
				Buildings: buildings,
				Generals:  generals,
				Armies:    armies,
			},
		},
	}
}

func (s *PlayerService) initPlayer(p *PlayerActor) error {
	if p == nil {
		return errors.New("player is nil")
	}
	player := p.Entity()

	var needFlush bool
	if player.Resource() == nil {
		needFlush = player.SetResource(s.buildInitialResource()) || needFlush
	}

	if player.Attribute() == nil {
		needFlush = player.SetAttribute(s.buildInitialAttribute()) || needFlush
	}

	if player.LenFacility() <= 0 {
		needFlush = player.ReplaceFacility(s.buildInitialFacility()) || needFlush
	}

	if needFlush {
		return p.DC().FlushSync(context.TODO())
	}
	return nil
}

func (s *PlayerService) hasCreatedRole(player *entity.PlayerEntity) bool {
	if player == nil || player.Profile() == nil {
		return false
	}
	return player.Profile().NickName() != ""
}

func (s *PlayerService) buildInitialProfile() entity.RoleState {
	return entity.RoleState{
		Headid:    0,
		Sex:       0,
		NickName:  "momo",
		CreatedAt: time.Now(),
	}
}

func (s *PlayerService) buildInitialResource() entity.ResourceState {
	config := basic.BasicConf.Role

	return entity.ResourceState{
		Wood:   config.Wood,
		Iron:   config.Iron,
		Stone:  config.Stone,
		Grain:  config.Grain,
		Gold:   config.Gold,
		Decree: config.Decree,
	}
}

func (s *PlayerService) buildInitialAttribute() entity.RoleAttributeState {
	return entity.RoleAttributeState{
		ParentId: 0,
	}
}

func (s *PlayerService) buildInitialFacility() []entity.FacilityState {
	facilityList := facility.FacilityConf.List
	facilities := make([]entity.FacilityState, 0, len(facilityList))
	for _, v := range facilityList {
		f := entity.FacilityState{Name: v.Name, PrivateLevel: 0, FType: v.Type, UpTime: 0}
		facilities = append(facilities, f)
	}
	return facilities
}

func (s *PlayerService) GetGenerals(player *entity.PlayerEntity) (*playerpb.PlayerResponse, error) {
	// 随机三个武将 做为初始武将
	needGenerals := general.SkillLimit - player.LenGenerals()
	if needGenerals > 0 {
		for i := 0; i < needGenerals; i++ {
			cfgId := general.Rand()
			// 创建 general
			cfg := general.General.GMap[cfgId]
			id, _ := utils.NextSnowflakeID()
			generalState := entity.GeneralState{
				Id:             int(id),
				Power:          basic.BasicConf.General.PowerLimit,
				CfgId:          cfg.CfgId,
				Order:          0,
				CityId:         0,
				Level:          0,
				CreatedAt:      time.Now(),
				CurArms:        cfg.Arms[0],
				HasPrPoint:     0,
				UsePrPoint:     0,
				AttackDistance: 0,
				ForceAdded:     0,
				StrategyAdded:  0,
				DefenseAdded:   0,
				SpeedAdded:     0,
				DestroyAdded:   0,
				Star:           cfg.Star,
				StarLv:         0,
				ParentId:       0,
				Skills:         make([]entity.GSkillState, 0),
				State:          general.GeneralNormal,
			}
			player.PutGenerals(generalState.Id, generalState)
		}
	}

	generals := make([]*playerpb.General, 0)
	player.ForEachGenerals(func(i int, v entity.GeneralState) {
		generals = append(generals, ToPBGeneral(v))
	})

	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{Ok: true},
		Body: &playerpb.PlayerResponse_MyGeneralsResponse{
			MyGeneralsResponse: &playerpb.MyGeneralsResponse{
				Generals: generals,
			},
		},
	}, nil
}

func (s *PlayerService) Back(ctx actor.Context, p *PlayerActor, army entity.ArmyState) {
	player := p.Entity()
	worldPID := p.WorldPID()
	if worldPID == nil || p.WorldId == nil {
		ctx.Respond(fail("world actor unavailable"))
		return
	}

	f := ctx.RequestFuture(worldPID,
		&messages.HWBack{
			WorldBaseMessage: messages.WorldBaseMessage{WorldId: int(*p.WorldId), PlayerId: int(*p.PlayerId)},
			ArmyId:           army.Id,
		},
		500*time.Millisecond,
	)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}

		if backRes, ok := res.(*messages.WHBack); ok && backRes.OK {
			updated := player.UpdateArmies(army.Id, func(v *entity.ArmyEntity) {
				if v == nil {
					return
				}
				a := backRes.Army
				v.SetCmd(a.Cmd)
				v.SetState(a.State)
				v.SetFromX(a.FromPos.X)
				v.SetFromY(a.FromPos.Y)
				v.SetToX(a.ToPos.X)
				v.SetToY(a.ToPos.Y)
				v.SetStartTime(time.UnixMilli(a.Start))
				v.SetEndTime(time.UnixMilli(a.End))
				v.SetFrozen(true)
			})
			if !updated {
				ctx.Respond(fail("army not found"))
				return
			}
			a, _ := player.GetArmies(army.Id)
			AssignArmyResponse(ctx, player, a)
		} else {
			ctx.Logger().Info("position", "err", entity.ErrCreateCity)
			ctx.Respond(fail("can't back the aim"))
		}
	})
}

func (s *PlayerService) Attack(ctx actor.Context, p *PlayerActor, army entity.ArmyState, x, y int) {
	player := p.Entity()
	worldPID := p.WorldPID()
	if worldPID == nil || p.WorldId == nil {
		ctx.Respond(fail("world actor unavailable"))
		return
	}

	f := ctx.RequestFuture(worldPID,
		&messages.HWAttack{
			WorldBaseMessage: messages.WorldBaseMessage{WorldId: int(*p.WorldId), PlayerId: int(*p.PlayerId)},
			DefenderPos:      messages.Pos{X: x, Y: y},
			Army:             s.toMessageArmy(player, army),
		},
		500*time.Millisecond,
	)
	ctx.ReenterAfter(f, func(res interface{}, err error) {
		if err != nil {
			ctx.Respond(fail(err.Error()))
			return
		}

		if attackRes, ok := res.(*messages.WHAttack); ok && attackRes.OK {
			updated := player.UpdateArmies(army.Id, func(v *entity.ArmyEntity) {
				if v == nil {
					return
				}

				v.SetCmd(entity.ArmyCmdAttack)
				v.SetState(entity.ArmyRunning)
				v.SetFromX(player.City().X())
				v.SetFromY(player.City().Y())
				v.SetToX(x)
				v.SetToY(y)
				v.SetStartTime(attackRes.StartTime)
				v.SetEndTime(attackRes.EndTime)
				v.SetFrozen(true)
			})
			if !updated {
				ctx.Respond(fail("army not found"))
				return
			}
			a, _ := player.GetArmies(army.Id)
			AssignArmyResponse(ctx, player, a)
		} else {
			ctx.Logger().Info("position", "err", entity.ErrCreateCity)
			ctx.Respond(fail("can't attack the aim"))
		}
	})
}

func (s *PlayerService) toMessageArmy(player *entity.PlayerEntity, army entity.ArmyState) messages.Army {
	var soldiers [3]int
	for i := 0; i < len(soldiers) && i < len(army.Soldiers); i++ {
		soldiers[i] = army.Soldiers[i]
	}
	var conTimes [3]int64
	for i := 0; i < len(conTimes) && i < len(army.ConscriptEndTimes); i++ {
		conTimes[i] = army.ConscriptEndTimes[i]
	}
	var conCounts [3]int
	for i := 0; i < len(conCounts) && i < len(army.ConscriptCounts); i++ {
		conCounts[i] = army.ConscriptCounts[i]
	}

	generals := s.toMessageGenerals(player, army.Generals)

	return messages.Army{
		Id:         army.Id,
		CityId:     int(army.CityId),
		PlayerId:   int(army.PlayerId),
		AllianceId: int(army.AllianceId),
		Order:      army.Order,
		Generals:   generals,
		Soldiers:   soldiers,
		ConTimes:   conTimes,
		ConCounts:  conCounts,
		Cmd:        army.Cmd,
		State:      army.State,
		FromPos:    messages.Pos{X: army.FromX, Y: army.FromY},
		ToPos:      messages.Pos{X: army.ToX, Y: army.ToY},
		Start:      timeToMillis(army.StartTime),
		End:        timeToMillis(army.EndTime),
	}
}

func (s *PlayerService) toMessageGenerals(player *entity.PlayerEntity, generalIDs []int) []*messages.General {
	if player == nil || len(generalIDs) == 0 {
		return nil
	}
	gens := make([]*messages.General, 0, len(generalIDs))
	for _, generalID := range generalIDs {
		if generalID <= 0 {
			continue
		}
		general, ok := player.GetGenerals(generalID)
		if !ok {
			continue
		}
		gens = append(gens, s.toMessageGeneral(general))
	}
	return gens
}

func (s *PlayerService) toMessageGeneral(g entity.GeneralState) *messages.General {
	return &messages.General{
		Id:             g.Id,
		CfgId:          g.CfgId,
		Power:          g.Power,
		Level:          g.Level,
		Exp:            g.Exp,
		Order:          g.Order,
		CityId:         g.CityId,
		CreatedAt:      g.CreatedAt,
		CurArms:        g.CurArms,
		HasPrPoint:     g.HasPrPoint,
		UsePrPoint:     g.UsePrPoint,
		AttackDistance: g.AttackDistance,
		ForceAdded:     g.ForceAdded,
		StrategyAdded:  g.StrategyAdded,
		DefenseAdded:   g.DefenseAdded,
		SpeedAdded:     g.SpeedAdded,
		DestroyAdded:   g.DestroyAdded,
		StarLv:         g.StarLv,
		Star:           g.Star,
		ParentId:       g.ParentId,
		Skills:         s.toMessageGSkills(g.Skills),
		State:          g.State,
	}
}

func (s *PlayerService) toMessageGSkills(skills []entity.GSkillState) []messages.GSkill {
	if len(skills) == 0 {
		return nil
	}
	result := make([]messages.GSkill, 0, len(skills))
	for _, skill := range skills {
		result = append(result, messages.GSkill{
			Id:    skill.Id,
			CfgId: skill.CfgId,
			Lv:    skill.Lv,
		})
	}
	return result
}

func (s *PlayerService) Defend(ctx actor.Context, p *PlayerActor, army entity.ArmyState, x int, y int) {

}

func (s *PlayerService) Reclamation(ctx actor.Context, p *PlayerActor, army entity.ArmyState, x int, y int) {

}

func (s *PlayerService) Transfer(ctx actor.Context, p *PlayerActor, army entity.ArmyState, x int, y int) {

}

func AssignArmyResponse(ctx actor.Context, player *entity.PlayerEntity, army entity.ArmyState) {
	response := ok()
	response.Body = &playerpb.PlayerResponse_ArmyInfoResponse{
		ArmyInfoResponse: &playerpb.ArmyInfoResponse{
			Army: ToPBArmy(player.CityID(), army),
		},
	}
	ctx.Respond(response)
}

func (s *PlayerService) AssignPreCheck(army entity.ArmyState, x int, y int) error {
	//是否能出站
	if !s.IsCanOutWar(army) {
		return fmt.Errorf("army is busy")
	}
	// 判断此土地是否是能攻击的类型 比如山地
	nm, ok := _map.MapConf.GetCell(x, y)
	if !ok {
		return fmt.Errorf("request param invalid")
	}
	// 山地不能移动到此
	if nm.Type == 0 {
		return fmt.Errorf("request param invalid")
	}
	return nil
}

func (s *PlayerService) IsCanOutWar(a entity.ArmyState) bool {
	return a.Generals[0] == 0 && a.Cmd == entity.ArmyCmdIdle
}

func ComputeFacilityYield(player *entity.PlayerEntity) facility.FacilityYield {
	var yield facility.FacilityYield
	player.ForEachFacility(func(i int, v entity.FacilityState) {
		facility, ok := facility.FacilityConf.GetFacility(v.FType)
		if !ok {
			return
		}
		y := facility.GetFacilityYield(v.PrivateLevel)
		yield.Wood += y.Wood
		yield.Grain += y.Grain
		yield.Iron += y.Iron
		yield.Stone += y.Stone
		yield.Gold += y.Gold
	})
	return yield
}

func OK() *playerpb.PlayerResponse {
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{
			Ok: true,
		},
	}
}

func Fail(reason string) *playerpb.PlayerResponse {
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{
			Ok:      false,
			Reason:  reason,
			Message: reason,
		},
	}
}

func ToPBRole(r *entity.RoleEntity) *playerpb.Role {
	if r == nil {
		return &playerpb.Role{}
	}
	return &playerpb.Role{
		NickName: r.NickName(),
		Sex:      int32(r.Sex()),
		Balance:  int32(r.Balance()),
		HeadId:   int32(r.Headid()),
		Profile:  r.Profile(),
	}
}

func ToPBResource(res *entity.ResourceEntity) *playerpb.Resource {
	if res == nil {
		return &playerpb.Resource{}
	}
	return &playerpb.Resource{
		Wood:   int32(res.Wood()),
		Iron:   int32(res.Iron()),
		Stone:  int32(res.Stone()),
		Grain:  int32(res.Grain()),
		Gold:   int32(res.Gold()),
		Decree: int32(res.Decree()),
	}
}

func ToPBBuilding(b entity.BuildingState) *playerpb.Building {
	// BuildingEntity 当前不携带联盟/上级/昵称信息，proto 对应字段保持默认值。
	// GiveUpTime 历史实现按“秒 -> 毫秒”返回，这里沿用；若实体内已是毫秒需去掉 *1000。
	return &playerpb.Building{
		Name:       b.Name,
		X:          int32(b.X),
		Y:          int32(b.Y),
		Type:       int32(b.BuildingType),
		Level:      int32(b.Level),
		OpLevel:    int32(b.OPLevel),
		CurDurable: int32(b.CurDurable),
		MaxDurable: int32(b.MaxDurable),
		Defender:   int32(b.Defender),
		OccupyTime: timeToMillis(b.OccupyTime),
		EndTime:    timeToMillis(b.EndTime),
		GiveUpTime: b.GiveUpTime * 1000,
		ParentId:   0,
		UnionId:    0,
		UnionName:  "",
		Rnick:      "",
	}
}

func ToPBGeneral(g entity.GeneralState) *playerpb.General {
	skills := make([]*playerpb.GSkill, 0, len(g.Skills))
	for _, value := range g.Skills {
		skills = append(skills, ToPBGSkill(value))
	}
	return &playerpb.General{
		Id:             int32(g.Id),
		CfgId:          int32(g.CfgId),
		PhysicalPower:  int32(g.Power),
		Order:          int32(g.Order),
		Level:          int32(g.Level),
		Exp:            int32(g.Exp),
		CityId:         int32(g.CityId),
		CurArms:        int32(g.CurArms),
		HasPrPoint:     int32(g.HasPrPoint),
		UsePrPoint:     int32(g.UsePrPoint),
		AttackDistance: int32(g.AttackDistance),
		ForceAdded:     int32(g.ForceAdded),
		StrategyAdded:  int32(g.StrategyAdded),
		DefenseAdded:   int32(g.DefenseAdded),
		SpeedAdded:     int32(g.SpeedAdded),
		DestroyAdded:   int32(g.DestroyAdded),
		StarLv:         int32(g.StarLv),
		Star:           int32(g.Star),
		ParentId:       int32(g.ParentId),
		Skills:         skills,
		State:          int32(g.State),
	}
}

func ToPBGSkill(skill entity.GSkillState) *playerpb.GSkill {
	return &playerpb.GSkill{
		Id:    int32(skill.Id),
		Lv:    int32(skill.Lv),
		CfgId: int32(skill.CfgId),
	}
}

func ToPBArmy(cityId CityID, a entity.ArmyState) *playerpb.Army {
	return buildPBArmy(pbArmyPayload{
		id:       a.Id,
		cityID:   int(cityId),
		order:    int(a.Order),
		unionID:  0,
		generals: append([]int(nil), a.Generals...),
		soldiers: append([]int(nil), a.Soldiers...),
		conTimes: append([]int64(nil), a.ConscriptEndTimes...),
		conCnts:  append([]int(nil), a.ConscriptCounts...),
		cmd:      int(a.Cmd),
		state:    int(a.State),
		fromX:    a.FromX,
		fromY:    a.FromY,
		toX:      a.ToX,
		toY:      a.ToY,
		start:    timeToMillis(a.StartTime),
		end:      timeToMillis(a.EndTime),
	})
}

func ToPBWHScanBlock(v messages.WHScanBlock) *playerpb.ScanBlockResponse {
	buildings := make([]*playerpb.Building, 0, len(v.Buildings))
	for _, b := range v.Buildings {
		buildings = append(buildings, ToPBScanBuilding(b))
	}
	cities := make([]*playerpb.City, 0, len(v.Cities))
	for _, c := range v.Cities {
		cities = append(cities, ToPBScanCity(c))
	}
	armies := make([]*playerpb.Army, 0, len(v.Armies))
	for _, a := range v.Armies {
		armies = append(armies, buildPBArmy(pbArmyPayloadFromMessage(a)))
	}
	return &playerpb.ScanBlockResponse{
		Buildings: buildings,
		Cities:    cities,
		Armies:    armies,
	}
}

func ToPBScanBuilding(b messages.Building) *playerpb.Building {
	return &playerpb.Building{
		PlayerId:   int32(b.PlayerId),
		Rnick:      b.RNick,
		Name:       b.Name,
		UnionId:    int32(b.AllianceId),
		UnionName:  b.AllianceName,
		ParentId:   int32(b.ParentId),
		X:          int32(b.Pos.X),
		Y:          int32(b.Pos.Y),
		Type:       int32(b.Type),
		Level:      int32(b.Level),
		OpLevel:    int32(b.OPLevel),
		CurDurable: int32(b.CurDurable),
		MaxDurable: int32(b.MaxDurable),
		Defender:   int32(b.Defender),
		OccupyTime: timeToMillis(b.OccupyTime),
		EndTime:    timeToMillis(b.EndTime),
		GiveUpTime: timeToMillis(b.GiveUpTime),
	}
}

func ToPBScanCity(c messages.WorldCity) *playerpb.City {
	return &playerpb.City{
		PlayerId:   int32(c.PlayerId),
		CityId:     c.CityId,
		Name:       c.Name,
		UnionId:    int32(c.AllianceId),
		UnionName:  c.AllianceName,
		ParentId:   int32(c.ParentId),
		X:          int32(c.Pos.X),
		Y:          int32(c.Pos.Y),
		IsMain:     c.IsMain,
		Level:      int32(c.Level),
		CurDurable: int32(c.CurDurable),
		MaxDurable: int32(c.MaxDurable),
		OccupyTime: c.OccupyTime,
	}
}

type pbArmyPayload struct {
	id       int
	cityID   int
	order    int
	unionID  int
	generals []int
	soldiers []int
	conTimes []int64
	conCnts  []int
	cmd      int
	state    int
	fromX    int
	fromY    int
	toX      int
	toY      int
	start    int64
	end      int64
}

func buildPBArmy(payload pbArmyPayload) *playerpb.Army {
	generals := make([]int32, 0, len(payload.generals))
	for _, value := range payload.generals {
		generals = append(generals, int32(value))
	}
	soldiers := make([]int32, 0, len(payload.soldiers))
	for _, value := range payload.soldiers {
		soldiers = append(soldiers, int32(value))
	}
	conTimes := make([]int64, 0, len(payload.conTimes))
	for _, value := range payload.conTimes {
		conTimes = append(conTimes, value)
	}
	conCnts := make([]int32, 0, len(payload.conCnts))
	for _, value := range payload.conCnts {
		conCnts = append(conCnts, int32(value))
	}

	return &playerpb.Army{
		Id:       int32(payload.id),
		CityId:   int32(payload.cityID),
		Order:    int32(payload.order),
		UnionId:  int32(payload.unionID),
		Generals: generals,
		Soldiers: soldiers,
		ConTimes: conTimes,
		ConCnts:  conCnts,
		Cmd:      int32(payload.cmd),
		State:    int32(payload.state),
		FromX:    int32(payload.fromX),
		FromY:    int32(payload.fromY),
		ToX:      int32(payload.toX),
		ToY:      int32(payload.toY),
		Start:    payload.start,
		End:      payload.end,
	}
}

func pbArmyPayloadFromMessage(a messages.Army) pbArmyPayload {
	generals := make([]int, 0, len(a.Generals))
	for _, v := range a.Generals {
		if v == nil {
			generals = append(generals, 0)
			continue
		}
		generals = append(generals, v.Id)
	}
	soldiers := make([]int, 0, len(a.Soldiers))
	for _, v := range a.Soldiers {
		soldiers = append(soldiers, v)
	}
	conTimes := make([]int64, 0, len(a.ConTimes))
	for _, v := range a.ConTimes {
		conTimes = append(conTimes, v)
	}
	conCnts := make([]int, 0, len(a.ConCounts))
	for _, v := range a.ConCounts {
		conCnts = append(conCnts, v)
	}
	return pbArmyPayload{
		id:       a.Id,
		cityID:   a.CityId,
		order:    int(a.Order),
		unionID:  a.AllianceId,
		generals: generals,
		soldiers: soldiers,
		conTimes: conTimes,
		conCnts:  conCnts,
		cmd:      int(a.Cmd),
		state:    int(a.State),
		fromX:    a.FromPos.X,
		fromY:    a.FromPos.Y,
		toX:      a.ToPos.X,
		toY:      a.ToPos.Y,
		start:    a.Start,
		end:      a.End,
	}
}

func timeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func millisToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func firstOrNil(s []entity.GeneralState) entity.GeneralState {
	if len(s) == 0 {
		return entity.GeneralState{}
	}
	return s[0]
}

func ToPBWarReport(cityId CityID, v entity.WarReportState) *playerpb.WarReport {
	return &playerpb.WarReport{
		Id:                int32(v.Id),
		AttackRid:         int32(v.Attacker),
		DefenseRid:        int32(v.Defender),
		BegAttackArmy:     ToPBArmy(cityId, v.BegAttackArmy),
		BegDefenseArmy:    ToPBArmy(cityId, v.BegDefenseArmy),
		EndAttackArmy:     ToPBArmy(cityId, v.EndAttackArmy),
		EndDefenseArmy:    ToPBArmy(cityId, v.EndDefenseArmy),
		BegAttackGeneral:  ToPBGeneral(firstOrNil(v.BegAttackGeneral)),
		BegDefenseGeneral: ToPBGeneral(firstOrNil(v.BegDefenseGeneral)),
		EndAttackGeneral:  ToPBGeneral(firstOrNil(v.EndAttackGeneral)),
		EndDefenseGeneral: ToPBGeneral(firstOrNil(v.EndDefenseGeneral)),
		Result:            int32(v.Result),
		Rounds:            v.Rounds,
		AttackIsRead:      v.AttackIsRead,
		DefenseIsRead:     v.DefenseIsRead,
		DestroyDurable:    int32(v.DestroyDurable),
		Occupy:            int32(v.Occupy),
		X:                 int32(v.X),
		Y:                 int32(v.Y),
		Ctime:             int64(v.CTime),
	}
}

func Generals(v []*messages.General) []entity.GeneralState {
	generals := make([]entity.GeneralState, 0, len(v))
	for _, g := range v {
		generals = append(generals, msgToEntityGeneral(g))
	}
	return generals
}

func serializeWarReportRounds(rounds []*messages.Round) string {
	if len(rounds) == 0 {
		return ""
	}
	raw, err := json.Marshal(rounds)
	if err != nil {
		return ""
	}
	return string(raw)
}

func ToWarReport(v messages.WarReport) entity.WarReportState {
	state := entity.WarReportState{
		Id:                v.Id,
		Attacker:          v.Attacker,
		Defender:          v.Defender,
		BegAttackArmy:     msgArmyToArmy(v.BegAttackArmy),
		BegDefenseArmy:    msgArmyToArmy(v.BegDefenseArmy),
		EndAttackArmy:     msgArmyToArmy(v.EndAttackArmy),
		EndDefenseArmy:    msgArmyToArmy(v.EndDefenseArmy),
		BegAttackGeneral:  Generals(v.BegAttackGeneral),
		BegDefenseGeneral: Generals(v.BegDefenseGeneral),
		EndAttackGeneral:  Generals(v.EndAttackGeneral),
		EndDefenseGeneral: Generals(v.EndDefenseGeneral),
		Result:            int(v.Result),
		Rounds:            serializeWarReportRounds(v.Rounds),
		AttackIsRead:      v.AttackIsRead,
		DefenseIsRead:     v.DefenseIsRead,
		DestroyDurable:    v.DestroyDurable,
		Occupy:            v.Occupy,
		X:                 v.X,
		Y:                 v.Y,
		CTime:             v.CTime,
	}
	if state.CTime <= 0 {
		state.CTime = int(time.Now().UnixMilli())
	}
	return state
}

func msgArmyToArmy(army *messages.Army) entity.ArmyState {
	if army == nil {
		return entity.ArmyState{}
	}
	generals := make([]int, 0, len(army.Generals))
	for _, g := range army.Generals {
		if g == nil {
			generals = append(generals, 0)
			continue
		}
		generals = append(generals, g.Id)
	}
	soldiers := make([]int, 0, len(army.Soldiers))
	for _, value := range army.Soldiers {
		soldiers = append(soldiers, value)
	}
	conscriptEndTimes := make([]int64, 0, len(army.ConTimes))
	for _, value := range army.ConTimes {
		conscriptEndTimes = append(conscriptEndTimes, value)
	}
	conscriptCounts := make([]int, 0, len(army.ConCounts))
	for _, value := range army.ConCounts {
		conscriptCounts = append(conscriptCounts, value)
	}

	state := entity.ArmyState{
		Id:                army.Id,
		CityId:            entity.CityID(army.CityId),
		PlayerId:          entity.PlayerID(army.PlayerId),
		AllianceId:        entity.AllianceID(army.AllianceId),
		Order:             army.Order,
		Generals:          generals,
		Soldiers:          soldiers,
		Cmd:               army.Cmd,
		FromX:             army.FromPos.X,
		FromY:             army.FromPos.Y,
		ToX:               army.ToPos.X,
		ToY:               army.ToPos.Y,
		State:             army.State,
		ConscriptEndTimes: conscriptEndTimes,
		ConscriptCounts:   conscriptCounts,
	}
	state.StartTime = millisToTime(army.Start)
	state.EndTime = millisToTime(army.End)
	return state
}

func msgToEntityGeneral(g *messages.General) entity.GeneralState {
	if g == nil {
		return entity.GeneralState{}
	}
	return entity.GeneralState{
		Id:             g.Id,
		CfgId:          g.CfgId,
		Power:          g.Power,
		Level:          g.Level,
		Exp:            g.Exp,
		Order:          g.Order,
		CityId:         g.CityId,
		CreatedAt:      g.CreatedAt,
		CurArms:        g.CurArms,
		HasPrPoint:     g.HasPrPoint,
		UsePrPoint:     g.UsePrPoint,
		AttackDistance: g.AttackDistance,
		ForceAdded:     g.ForceAdded,
		StrategyAdded:  g.StrategyAdded,
		DefenseAdded:   g.DefenseAdded,
		SpeedAdded:     g.SpeedAdded,
		DestroyAdded:   g.DestroyAdded,
		StarLv:         g.StarLv,
		Star:           g.Star,
		ParentId:       g.ParentId,
		Skills:         toEntityWarReportGSkills(g.Skills),
		State:          g.State,
	}
}

func toEntityWarReportGSkills(skills []messages.GSkill) []entity.GSkillState {
	if len(skills) == 0 {
		return nil
	}
	result := make([]entity.GSkillState, 0, len(skills))
	for _, skill := range skills {
		result = append(result, entity.GSkillState{
			Id:    skill.Id,
			CfgId: skill.CfgId,
			Lv:    skill.Lv,
		})
	}
	return result
}

func ToPBSkill(v entity.SkillState) *playerpb.Skill {
	return &playerpb.Skill{
		Id:       int32(v.Id),
		CfgId:    int32(v.CfgId),
		Generals: ConvertIntToInt32(v.Generals),
	}
}

func ConvertIntToInt32(src []int) []int32 {
	dst := make([]int32, len(src))
	for i := range src {
		dst[i] = int32(src[i])
	}
	return dst
}

func collectionCST() *time.Location {
	return time.FixedZone("CST", 8*3600)
}

func IsSameDayCST(t1, t2 time.Time) bool {
	tz := collectionCST()
	y1, m1, d1 := t1.In(tz).Date()
	y2, m2, d2 := t2.In(tz).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func TodayZeroCST(t time.Time) time.Time {
	tz := collectionCST()
	y, m, d := t.In(tz).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, tz)
}

func NextCSTMidnight(t time.Time) time.Time {
	tz := collectionCST()
	t = t.In(tz)
	nextZero := time.Date(
		t.Year(),
		t.Month(),
		t.Day()+1,
		0, 0, 0, 0,
		tz,
	)
	return nextZero
}
