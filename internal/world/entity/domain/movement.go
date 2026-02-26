package domain

import "time"

// entity
type March struct {
	playerID PlayerID
	armyID   ArmyID

	from Pos
	to   Pos

	startAt  time.Time
	arriveAt time.Time
}
