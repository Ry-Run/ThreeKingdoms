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
	posTags         string   // `decode:"PosTagsToPosTagArray" encode:"PosTagArrayToPosTags"`,
	posTagArray     []PosTag // mapper:ignore
}

type PosTag struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Name string `json:"name"`
}

func (p *RoleAttribute) PosTagsToPosTagArray() {

}

func (p *RoleAttribute) PosTagArrayToPosTags() {

}
