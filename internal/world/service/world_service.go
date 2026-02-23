package service

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/global"
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
	city := CityState{
		CityId:     cityID,
		X:          rand.Intn(global.MapWith),
		Y:          rand.Intn(global.MapHeight),
		Name:       request.NickName,
		CurDurable: basic.BasicConf.City.Durable,
		IsMain:     true,
	}
	cityMap := make(map[CityID]CityState)
	cityMap[cityID] = city
	e.PutCityByPlayer(PlayerId, cityMap)
	return cityID
}
