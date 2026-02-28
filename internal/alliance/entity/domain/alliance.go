package domain

type WorldID int
type PlayerID int
type AllianceID int

type CityID int

// entity
type Alliance struct {
	id        AllianceID
	worldId   WorldID
	name      string
	notice    string
	majors    map[PlayerID]*Major // 联盟主要人物，盟主副盟主
	members   map[PlayerID]*Member
	applyList []*ApplyItem
}
