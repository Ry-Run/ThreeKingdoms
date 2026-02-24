package dto

import (
	"ThreeKingdoms/internal/gate/app/model"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
)

type CreateRoleResp struct {
	Role model.Role `json:"role"`
}

type WorldMapResp struct {
	Map []Cell `json:"map"`
}

type Cell struct {
	Type     int32  `json:"type"`
	Name     string `json:"name"`
	Level    int32  `json:"level"`
	Grain    int64  `json:"grain"`
	Wood     int64  `json:"wood"`
	Iron     int64  `json:"iron"`
	Stone    int64  `json:"stone"`
	Durable  int64  `json:"durable"`
	Defender int64  `json:"defender"`
}

type MyPropertyResp struct {
	Resource  model.Resource `json:"resource"`
	Buildings []Building     `json:"buildings"`
	Generals  []General      `json:"generals"`
	Cities    []City         `json:"cities"`
	Armies    []Army         `json:"armies"`
}

type Building struct {
	Rnick      string `json:"rnick"`
	Name       string `json:"name"`
	UnionId    int32  `json:"union_id"`
	UnionName  string `json:"union_name"`
	ParentId   int32  `json:"parent_id"`
	X          int32  `json:"x"`
	Y          int32  `json:"y"`
	Type       int32  `json:"type"`
	Level      int32  `json:"level"`
	OpLevel    int32  `json:"op_level"`
	CurDurable int32  `json:"cur_durable"`
	MaxDurable int32  `json:"max_durable"`
	Defender   int32  `json:"defender"`
	OccupyTime int64  `json:"occupy_time"`
	EndTime    int64  `json:"end_time"`
	GiveUpTime int64  `json:"give_up_time"`
}

type General struct {
	Id             int32    `json:"id"`
	CfgId          int32    `json:"cfg_id"`
	PhysicalPower  int32    `json:"physical_power"`
	Order          int32    `json:"order"`
	Level          int32    `json:"level"`
	Exp            int32    `json:"exp"`
	CityId         int32    `json:"city_id"`
	CurArms        int32    `json:"cur_arms"`
	HasPrPoint     int32    `json:"has_pr_point"`
	UsePrPoint     int32    `json:"use_pr_point"`
	AttackDistance int32    `json:"attack_distance"`
	ForceAdded     int32    `json:"force_added"`
	StrategyAdded  int32    `json:"strategy_added"`
	DefenseAdded   int32    `json:"defense_added"`
	SpeedAdded     int32    `json:"speed_added"`
	DestroyAdded   int32    `json:"destroy_added"`
	StarLv         int32    `json:"star_lv"`
	Star           int32    `json:"star"`
	ParentId       int32    `json:"parent_id"`
	Skills         []GSkill `json:"skills"`
	State          int32    `json:"state"`
}

type GSkill struct {
	Id    int32 `json:"id"`
	Lv    int32 `json:"lv"`
	CfgId int32 `json:"cfgId"`
}

type City struct {
	Name       string `json:"name"`
	UnionId    int32  `json:"union_id"`
	UnionName  string `json:"union_name"`
	ParentId   int32  `json:"parent_id"`
	X          int32  `json:"x"`
	Y          int32  `json:"y"`
	IsMain     bool   `json:"is_main"`
	Level      int32  `json:"level"`
	CurDurable int32  `json:"cur_durable"`
	MaxDurable int32  `json:"max_durable"`
	OccupyTime int64  `json:"occupy_time"`
}

type Army struct {
	Id       int32   `json:"id"`
	CityId   int32   `json:"city_id"`
	UnionId  int32   `json:"union_id"`
	Order    int32   `json:"order"`
	Generals []int32 `json:"generals"`
	Soldiers []int32 `json:"soldiers"`
	ConTimes []int64 `json:"con_times"`
	ConCnts  []int32 `json:"con_cnts"`
	Cmd      int32   `json:"cmd"`
	State    int32   `json:"state"`
	FromX    int32   `json:"from_x"`
	FromY    int32   `json:"from_y"`
	ToX      int32   `json:"to_x"`
	ToY      int32   `json:"to_y"`
	Start    int64   `json:"start"`
	End      int64   `json:"end"`
}

func NewCreateRoleResp(resp *playerpb.CreateRoleResponse) CreateRoleResp {
	out := CreateRoleResp{}
	if resp == nil {
		return out
	}
	out.Role = roleFromPB(resp.GetRole())
	return out
}

func NewWorldMapResp(resp *playerpb.WorldMapResponse) WorldMapResp {
	out := WorldMapResp{}
	if resp == nil {
		return out
	}
	cells := resp.GetMap()
	out.Map = make([]Cell, 0, len(cells))
	for _, c := range cells {
		if c == nil {
			continue
		}
		out.Map = append(out.Map, Cell{
			Type:     c.GetType(),
			Name:     c.GetName(),
			Level:    c.GetLevel(),
			Grain:    c.GetGrain(),
			Wood:     c.GetWood(),
			Iron:     c.GetIron(),
			Stone:    c.GetStone(),
			Durable:  c.GetDurable(),
			Defender: c.GetDefender(),
		})
	}
	return out
}

func NewMyPropertyResp(resp *playerpb.MyPropertyResponse) MyPropertyResp {
	out := MyPropertyResp{}
	if resp == nil {
		return out
	}
	out.Resource = resourceFromPB(resp.GetResource())

	if items := resp.GetBuildings(); len(items) > 0 {
		out.Buildings = make([]Building, 0, len(items))
		for _, b := range items {
			if b == nil {
				continue
			}
			out.Buildings = append(out.Buildings, Building{
				Rnick:      b.GetRnick(),
				Name:       b.GetName(),
				UnionId:    b.GetUnionId(),
				UnionName:  b.GetUnionName(),
				ParentId:   b.GetParentId(),
				X:          b.GetX(),
				Y:          b.GetY(),
				Type:       b.GetType(),
				Level:      b.GetLevel(),
				OpLevel:    b.GetOpLevel(),
				CurDurable: b.GetCurDurable(),
				MaxDurable: b.GetMaxDurable(),
				Defender:   b.GetDefender(),
				OccupyTime: b.GetOccupyTime(),
				EndTime:    b.GetEndTime(),
				GiveUpTime: b.GetGiveUpTime(),
			})
		}
	}

	if items := resp.GetGenerals(); len(items) > 0 {
		out.Generals = make([]General, 0, len(items))
		for _, g := range items {
			if g == nil {
				continue
			}
			row := General{
				Id:             g.GetId(),
				CfgId:          g.GetCfgId(),
				PhysicalPower:  g.GetPhysicalPower(),
				Order:          g.GetOrder(),
				Level:          g.GetLevel(),
				Exp:            g.GetExp(),
				CityId:         g.GetCityId(),
				CurArms:        g.GetCurArms(),
				HasPrPoint:     g.GetHasPrPoint(),
				UsePrPoint:     g.GetUsePrPoint(),
				AttackDistance: g.GetAttackDistance(),
				ForceAdded:     g.GetForceAdded(),
				StrategyAdded:  g.GetStrategyAdded(),
				DefenseAdded:   g.GetDefenseAdded(),
				SpeedAdded:     g.GetSpeedAdded(),
				DestroyAdded:   g.GetDestroyAdded(),
				StarLv:         g.GetStarLv(),
				Star:           g.GetStar(),
				ParentId:       g.GetParentId(),
				State:          g.GetState(),
			}
			if skills := g.GetSkills(); len(skills) > 0 {
				row.Skills = make([]GSkill, 0, len(skills))
				for _, s := range skills {
					if s == nil {
						continue
					}
					row.Skills = append(row.Skills, GSkill{
						Id:    s.GetId(),
						Lv:    s.GetLv(),
						CfgId: s.GetCfgId(),
					})
				}
			}
			out.Generals = append(out.Generals, row)
		}
	}

	if items := resp.GetCities(); len(items) > 0 {
		out.Cities = make([]City, 0, len(items))
		for _, c := range items {
			if c == nil {
				continue
			}
			out.Cities = append(out.Cities, City{
				Name:       c.GetName(),
				UnionId:    c.GetUnionId(),
				UnionName:  c.GetUnionName(),
				ParentId:   c.GetParentId(),
				X:          c.GetX(),
				Y:          c.GetY(),
				IsMain:     c.GetIsMain(),
				Level:      c.GetLevel(),
				CurDurable: c.GetCurDurable(),
				MaxDurable: c.GetMaxDurable(),
				OccupyTime: c.GetOccupyTime(),
			})
		}
	}

	if items := resp.GetArmies(); len(items) > 0 {
		out.Armies = make([]Army, 0, len(items))
		for _, a := range items {
			if a == nil {
				continue
			}
			out.Armies = append(out.Armies, Army{
				Id:       a.GetId(),
				CityId:   a.GetCityId(),
				UnionId:  a.GetUnionId(),
				Order:    a.GetOrder(),
				Generals: append([]int32(nil), a.GetGenerals()...),
				Soldiers: append([]int32(nil), a.GetSoldiers()...),
				ConTimes: append([]int64(nil), a.GetConTimes()...),
				ConCnts:  append([]int32(nil), a.GetConCnts()...),
				Cmd:      a.GetCmd(),
				State:    a.GetState(),
				FromX:    a.GetFromX(),
				FromY:    a.GetFromY(),
				ToX:      a.GetToX(),
				ToY:      a.GetToY(),
				Start:    a.GetStart(),
				End:      a.GetEnd(),
			})
		}
	}

	return out
}

func roleFromPB(role *playerpb.Role) model.Role {
	if role == nil {
		return model.Role{}
	}
	return model.Role{
		RId:      int(role.GetRid()),
		UId:      int(role.GetUid()),
		NickName: role.GetNickName(),
		Sex:      int8(role.GetSex()),
		Balance:  int(role.GetBalance()),
		HeadId:   int16(role.GetHeadId()),
		Profile:  role.GetProfile(),
	}
}

func resourceFromPB(resource *playerpb.Resource) model.Resource {
	if resource == nil {
		return model.Resource{}
	}
	return model.Resource{
		Wood:          int(resource.GetWood()),
		Iron:          int(resource.GetIron()),
		Stone:         int(resource.GetStone()),
		Grain:         int(resource.GetGrain()),
		Gold:          int(resource.GetGold()),
		Decree:        int(resource.GetDecree()),
		WoodYield:     int(resource.GetWoodYield()),
		IronYield:     int(resource.GetIronYield()),
		StoneYield:    int(resource.GetStoneYield()),
		GrainYield:    int(resource.GetGrainYield()),
		GoldYield:     int(resource.GetGoldYield()),
		DepotCapacity: int(resource.GetDepotCapacity()),
	}
}
