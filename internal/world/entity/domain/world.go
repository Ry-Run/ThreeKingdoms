package domain

type WorldID int
type PlayerID int

type CityID int

type ArmyID int

type AllianceID int

// entity
type World struct {
	worldId      WorldID
	cityByPlayer map[PlayerID]map[CityID]*City
	worldMap     map[int]*Cell
	armies       map[PlayerID]map[ArmyID]*Army  // 地图上的军队池
	marches      map[PlayerID]map[ArmyID]*March // 行军数据（高频更新）
	cellToMarch  map[int][]March                // 空间索引
}
