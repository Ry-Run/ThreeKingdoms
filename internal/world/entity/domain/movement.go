package domain

import "time"

// entity
type March struct {
	playerId   PlayerID
	allianceId AllianceID
	armyID     ArmyID

	from Pos
	to   Pos

	startAt  time.Time
	arriveAt time.Time
}
