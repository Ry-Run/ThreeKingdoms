package actors

import (
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/facility"
	"ThreeKingdoms/internal/shared/gameconfig/general"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/security"
	"context"
	"errors"
	"time"
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
	player.ForEachArmies(func(i CityID, v []entity.ArmyState) {
		for _, arm := range v {
			armies = append(armies, ToPBArmy(arm))
		}
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
			generalState := entity.GeneralState{
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
			player.AppendGenerals(generalState)
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
		OccupyTime: b.OccupyTime.UnixNano() / 1e6,
		EndTime:    b.EndTime.UnixNano() / 1e6,
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

func ToPBArmy(a entity.ArmyState) *playerpb.Army {
	// ArmyEntity 当前无 unionId，proto.UnionId 保持默认值。
	// start/end 参照旧 ToModel 使用秒；若前端期望毫秒需改为 UnixNano()/1e6。
	generals := make([]int32, 0, len(a.GeneralArray))
	for _, value := range a.GeneralArray {
		generals = append(generals, int32(value))
	}
	soldiers := make([]int32, 0, len(a.SoldierArray))
	for _, value := range a.SoldierArray {
		soldiers = append(soldiers, int32(value))
	}
	conTimes := make([]int64, 0, len(a.ConscriptTimeArray))
	for _, value := range a.ConscriptTimeArray {
		conTimes = append(conTimes, value)
	}
	conCnts := make([]int32, 0, len(a.ConscriptCntArray))
	for _, value := range a.ConscriptCntArray {
		conCnts = append(conCnts, int32(value))
	}

	return &playerpb.Army{
		Id:       int32(a.Id),
		CityId:   int32(a.CityId),
		Order:    int32(a.Order),
		UnionId:  0,
		Generals: generals,
		Soldiers: soldiers,
		ConTimes: conTimes,
		ConCnts:  conCnts,
		Cmd:      int32(a.Cmd),
		State:    int32(a.State),
		FromX:    int32(a.FromX),
		FromY:    int32(a.FromY),
		ToX:      int32(a.ToX),
		ToY:      int32(a.ToY),
		Start:    a.StartTime.Unix(),
		End:      a.EndTime.Unix(),
	}
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
		armies = append(armies, ToPBScanArmy(a))
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
		UnionId:    int32(b.UnionId),
		UnionName:  b.UnionName,
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
	}
}

func ToPBScanArmy(a messages.Army) *playerpb.Army {
	generals := make([]int32, 0, len(a.Generals))
	for _, v := range a.Generals {
		generals = append(generals, int32(v))
	}
	soldiers := make([]int32, 0, len(a.Soldiers))
	for _, v := range a.Soldiers {
		soldiers = append(soldiers, int32(v))
	}
	conTimes := make([]int64, 0, len(a.ConTimes))
	for _, v := range a.ConTimes {
		conTimes = append(conTimes, v)
	}
	conCnts := make([]int32, 0, len(a.ConCounts))
	for _, v := range a.ConCounts {
		conCnts = append(conCnts, int32(v))
	}
	return &playerpb.Army{
		Id:       int32(a.Id),
		CityId:   int32(a.CityId),
		UnionId:  int32(a.UnionId),
		Order:    int32(a.Order),
		Generals: generals,
		Soldiers: soldiers,
		ConTimes: conTimes,
		ConCnts:  conCnts,
		Cmd:      int32(a.Cmd),
		State:    int32(a.State),
		FromX:    int32(a.FromPos.X),
		FromY:    int32(a.FromPos.Y),
		ToX:      int32(a.ToPos.X),
		ToY:      int32(a.ToPos.Y),
		Start:    a.Start,
		End:      a.End,
	}
}

func timeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano() / 1e6
}

func ToPBWarReport(v entity.WarReportState) *playerpb.WarReport {
	// CTime 在实体中是 int，当前直接透传到 proto int64；
	// 若后续统一时间单位（秒/毫秒）规范，这里再一起收敛。
	return &playerpb.WarReport{
		Id:                int32(v.Id),
		AttackRid:         int32(v.Attacker),
		DefenseRid:        int32(v.Defender),
		BegAttackArmy:     ToPBArmy(v.BegAttackArmy),
		BegDefenseArmy:    ToPBArmy(v.BegDefenseArmy),
		EndAttackArmy:     ToPBArmy(v.EndAttackArmy),
		EndDefenseArmy:    ToPBArmy(v.EndDefenseArmy),
		BegAttackGeneral:  ToPBGeneral(v.BegAttackGeneral),
		BegDefenseGeneral: ToPBGeneral(v.BegDefenseGeneral),
		EndAttackGeneral:  ToPBGeneral(v.EndAttackGeneral),
		EndDefenseGeneral: ToPBGeneral(v.EndDefenseGeneral),
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
