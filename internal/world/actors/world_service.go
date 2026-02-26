package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/map"
	"ThreeKingdoms/internal/shared/utils"
	"ThreeKingdoms/internal/world/entity"
	"math/rand"
)

type WorldService struct{}

var WS = &WorldService{}

type PlayerID = entity.PlayerID
type CityID = entity.CityID
type ArmyID = entity.ArmyID
type CityState = entity.CityState

func (s *WorldService) CreateCity(e *entity.WorldEntity, request messages.HWCreateCity) CityID {
	PlayerId := entity.PlayerID(request.PlayerId)
	cities, ok := e.GetCityByPlayer(PlayerId)

	if ok && cities != nil {
		return -1
	}
	id, _ := utils.NextSnowflakeID()
	cityID := CityID(id)

	var city CityState
	for {
		x := rand.Intn(_map.MapWidth)
		y := rand.Intn(_map.MapHeight)

		if !CanBuildCity(x, y) {
			continue
		}

		city = CityState{
			CityId: cityID,
			Pos: entity.PosState{
				X: x,
				Y: y,
			},
			Name:       request.NickName,
			CurDurable: basic.BasicConf.City.Durable,
			IsMain:     true,
		}
		break
	}
	cityMap := make(map[CityID]CityState)
	cityMap[cityID] = city
	e.PutCityByPlayer(PlayerId, cityMap)
	return cityID
}

func (s *WorldService) ScanBlock(w *WorldActor, request messages.HWScanBlock) messages.WHScanBlock {
	world := w.Entity()
	x, y, Length := request.X, request.Y, request.Length
	if x < 0 || x >= _map.MapWidth || y < 0 || y >= _map.MapHeight {
		return messages.WHScanBlock{}
	}
	maxX := min(_map.MapWidth-1, x+Length-1)
	maxY := min(_map.MapHeight-1, y+Length-1)

	buildings := make([]messages.Building, 0)
	cities := make([]messages.WorldCity, 0)
	// 驻军 && 行军
	armies := make([]messages.Army, 0)

	for i := x; i <= maxX; i++ {
		for j := y; j <= maxY; j++ {
			pos := _map.ToPosition(i, j)
			cell, ok := world.GetWorldMap(pos)
			if !ok {
				continue
			}

			kind := cell.Occupancy.Kind
			if kind == _map.MapPlayerCity {
				// 返回玩家城市
				cityId := CityID(cell.Occupancy.RefId)
				playerId := PlayerID(cell.Occupancy.Owner)
				cityMap, ok := world.GetCityByPlayer(playerId)
				if !ok {
					continue
				}

				if city, ok := cityMap[cityId]; ok {
					cities = append(cities, ToMessagesCity(city, playerId))
				}
			} else if cell.Occupancy.Garrison.ArmyId != 0 && cell.Occupancy.Owner != 0 {
				// 返回驻军信息
				armyID := ArmyID(cell.Occupancy.Garrison.ArmyId)
				playerId := PlayerID(cell.Occupancy.Owner)
				armyMap, ok := world.GetArmies(playerId)
				if !ok {
					continue
				}

				if army, ok := armyMap[armyID]; ok {
					armies = append(armies, ToMessagesArmy(army))
				}
			} else if cell.Occupancy.Owner != 0 || kind == _map.MapBuildSysFortress || kind == _map.MapBuildSysCity {
				// 返回动态建筑/占领地（含系统战略点与玩家动态占据地块）
				buildings = append(buildings, ToMessagesBuilding(cell))
			}

			// 行军信息
			marches, ok := world.GetCellToMarch(cell.Id)
			if ok && len(marches) > 0 {
				for _, march := range marches {
					armyID := march.ArmyID
					playerId := march.PlayerID
					armyMap, ok := world.GetArmies(playerId)
					if !ok {
						continue
					}

					if army, ok := armyMap[armyID]; ok {
						armies = append(armies, ToMessagesArmy(army))
					}
				}
			}
		}
	}

	return messages.WHScanBlock{
		Cities:    cities,
		Armies:    armies,
		Buildings: buildings,
	}
}

func ToMessagesBuilding(cell entity.CellState) messages.Building {
	b := messages.Building{}
	b.PlayerId = cell.Occupancy.Owner
	b.RNick = cell.Occupancy.RoleNick
	b.UnionId = cell.Occupancy.UnionId
	b.UnionName = cell.Occupancy.UnionName
	b.ParentId = cell.Occupancy.ParentId
	b.Pos = messages.Pos{X: cell.Pos.X, Y: cell.Pos.Y}
	b.Type = cell.Occupancy.Kind
	b.Name = cell.Name

	b.OccupyTime = cell.OccupyTime
	b.GiveUpTime = cell.GiveUpTime
	b.EndTime = cell.EndTime

	//if cell.EndTime.IsZero() == false {
	//	if IsHasTransferAuth(cell) {
	//		if time.Now().Before(cell.EndTime) == false {
	//			if cell.OPLevel == 0 {
	//				cell.ConvertToRes()
	//			} else {
	//				cell.Level = cell.OPLevel
	//				cell.EndTime = time.Time{}
	//				cfg, ok := static_conf.MapBCConf.BuildConfig(cell.Type, cell.Level)
	//				if ok {
	//					cell.MaxDurable = cfg.Durable
	//					cell.CurDurable = min(cell.MaxDurable, cell.CurDurable)
	//					cell.Defender = cfg.Defender
	//				}
	//			}
	//		}
	//	}
	//}

	b.CurDurable = cell.CurDurable
	b.MaxDurable = cell.MaxDurable
	b.Defender = cell.Defender
	b.Level = cell.Level
	b.OPLevel = cell.OpLevel
	return b
}

func ToMessagesCity(city entity.CityState, playerId PlayerID) messages.WorldCity {
	return messages.WorldCity{
		PlayerId:   int(playerId),
		CityId:     int64(city.CityId),
		Name:       city.Name,
		Pos:        messages.Pos{X: city.Pos.X, Y: city.Pos.Y},
		IsMain:     city.IsMain,
		Level:      city.Level,
		CurDurable: city.CurDurable,
		MaxDurable: city.MaxDurable,
		OccupyTime: city.OccupyTime.UnixNano() / 1e6,
		UnionId:    city.UnionId,
		UnionName:  city.UnionName,
		ParentId:   city.ParentId,
	}
}

func ToMessagesArmy(army entity.ArmyState) messages.Army {
	var generals [3]int
	for i := 0; i < len(generals) && i < len(army.GeneralArray); i++ {
		generals[i] = army.GeneralArray[i]
	}
	var soldiers [3]int
	for i := 0; i < len(soldiers) && i < len(army.SoldierArray); i++ {
		soldiers[i] = army.SoldierArray[i]
	}
	var conTimes [3]int64
	for i := 0; i < len(conTimes) && i < len(army.ConscriptTimeArray); i++ {
		conTimes[i] = army.ConscriptTimeArray[i]
	}
	var conCounts [3]int
	for i := 0; i < len(conCounts) && i < len(army.ConscriptCntArray); i++ {
		conCounts[i] = army.ConscriptCntArray[i]
	}

	return messages.Army{
		Id:        army.Id,
		CityId:    int(army.CityId),
		UnionId:   0, // world 军队状态当前未维护联盟归属
		Order:     army.Order,
		Generals:  generals,
		Soldiers:  soldiers,
		ConTimes:  conTimes,
		ConCounts: conCounts,
		Cmd:       army.Cmd,
		State:     army.State,
		FromPos:   messages.Pos{X: army.FromX, Y: army.FromY},
		ToPos:     messages.Pos{X: army.ToX, Y: army.ToY},
		Start:     army.StartTime.Unix(),
		End:       army.EndTime.Unix(),
	}
}

// todo 根据 world 的 map 数据来确定是否可以建立城市
func CanBuildCity(x int, y int) bool {
	sysBuilding := _map.MapConf.SysBuilding
	confs := _map.MapConf.Confs
	index := _map.ToPosition(x, y)

	_, ok := confs[index]
	// 超出地图范围
	if !ok {
		return false
	}
	// 系统城池 5 格内不能有玩家城池
	for _, conf := range sysBuilding {
		if x <= conf.X+5 && x >= conf.X-5 &&
			y <= conf.Y+5 && y >= conf.Y-5 {
			return false
		}
	}
	return true
}
