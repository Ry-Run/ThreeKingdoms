package service

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

type CityID = entity.CityID
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
			CityId:     cityID,
			X:          x,
			Y:          y,
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

func CanBuildCity(x int, y int) bool {
	sysBuilding := _map.MapResource.SysBuilding
	confs := _map.MapResource.Confs
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
