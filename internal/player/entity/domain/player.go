package domain

type PlayerID int
type WorldID int
type CityID int
type AllianceID int

// entity
type Player struct {
	playerID   PlayerID
	worldID    WorldID
	allianceID AllianceID
	cityID     CityID
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
	skills     map[int]*Skill
}
