package domain

import (
	"time"
)

// entity
type RoleAttribute struct {
	unionId         int // mapper:ignore
	parentId        int
	collectTimes    int8
	lastCollectTime time.Time
	posTags         []PosTag
}

// entity
type PosTag struct {
	x    int
	y    int
	name string
}
