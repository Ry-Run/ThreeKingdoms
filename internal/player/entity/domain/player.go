package domain

type PlayerID int
type WorldID int
type CityID int

// entity
type Player struct {
	playerID   PlayerID
	worldID    WorldID
	profile    *Role
	resource   *Resource
	attribute  *RoleAttribute
	x          int
	y          int
	buildings  []*Building
	armies     map[CityID][]*Army
	generals   []*General
	facility   []*Facility
	warReports map[int]*WarReport
}
