package service

import (
	"ThreeKingdoms/internal/player/entity"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/security"
	"errors"
	"time"
)

type PlayerService struct{}

var PS = &PlayerService{}

func (s *PlayerService) EnterServer(player *entity.PlayerEntity) (*playerpb.PlayerResponse, error) {
	if player == nil {
		return nil, errors.New("player not loaded")
	}

	token, err := security.Award(int(player.PlayerID()))
	if err != nil {
		return nil, err
	}

	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{Ok: true},
		Body: &playerpb.PlayerResponse_EnterServerResponse{
			EnterServerResponse: &playerpb.EnterServerResponse{
				Role:     ToPBRole(player.Profile()),
				Resource: ToPBResource(player.Resource()),
				Token:    token,
				Time:     time.Now().UnixNano() / 1e6,
			},
		},
	}, nil
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
	player.ForEachArmies(func(i int, v entity.ArmyState) {
		armies = append(armies, ToPBArmy(v))
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

func (s *PlayerService) MyGenerals(request *playerpb.MyGeneralsRequest) (*playerpb.PlayerResponse, error) {
	if request == nil {
		return nil, errors.New("request parameter error")
	}
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{Ok: true},
		Body: &playerpb.PlayerResponse_MyGeneralsResponse{
			MyGeneralsResponse: &playerpb.MyGeneralsResponse{
				PlayerId: request.PlayerId,
			},
		},
	}, nil
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
	skills := make([]*playerpb.GSkill, 0, len(g.SkillsArray))
	for _, value := range g.SkillsArray {
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
