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
	buildings  []*Building
	armies     map[int]*Army
	generals   map[int]*General
	facility   []*Facility
	warReports map[int]*WarReport
	skills     map[int]*Skill
	city       *City
}
