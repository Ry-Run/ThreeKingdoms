package domain

type PlayerID int
type WorldID int

// entity
type Player struct {
	playerID  PlayerID
	worldID   WorldID
	profile   *Role
	resource  *Resource
	attribute *RoleAttribute
	x         int
	y         int
	buildings []*Building
	armies    []*Army
	generals  []*General
}
