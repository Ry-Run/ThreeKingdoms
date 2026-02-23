package domain

type WorldID int
type PlayerID int

type CityID int

// entity
type World struct {
	worldId      WorldID
	cityByPlayer map[PlayerID]map[CityID]*City
	worldMap     []*Cell
}
