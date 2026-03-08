package messages

import (
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"time"
)

type WorldMessage interface {
	WorldID() int
	PlayerID() int
}

type WorldBaseMessage struct {
	WorldId  int
	PlayerId int
}

func (w *WorldBaseMessage) WorldID() int {
	if w == nil {
		return 0
	}
	return w.WorldId
}

func (w *WorldBaseMessage) PlayerID() int {
	if w == nil {
		return 0
	}
	return w.PlayerId
}

type HWCreateCity struct {
	WorldBaseMessage
	NickName     string
	AllianceId   int
	AllianceName string
}

type WHCreateCity struct {
	X, Y   int
	CityId int
}

type HWMyCities struct {
	WorldBaseMessage
}

type WHMyCities struct {
	Cities []WorldCity
}

type HWScanBlock struct {
	WorldBaseMessage
	X, Y, Length int
}

type WHScanBlock struct {
	Cities    []WorldCity
	Armies    []Army
	Buildings []Building
}

type HWAttack struct {
	WorldBaseMessage
	DefenderPos Pos
	Army        Army
}

type WHAttack struct {
	OK        bool
	StartTime time.Time
	EndTime   time.Time
}

type HWBack struct {
	WorldBaseMessage
	ArmyId int
}

type WHBack struct {
	OK   bool
	Army Army
}

type HWSyncCityFacility struct {
	WorldBaseMessage
	CityId     int
	Facilities []Facility
}

type WHSyncCityFacility struct {
	OK bool
}

type WorldPushBatch struct {
	WorldBaseMessage
	MsgType MsgType
	Items   []WorldPushItem
}

type MsgType string

const (
	ArmyPush = "army.push"
)

type WorldPushItem struct {
	PlayerID int64
	Army     *playerpb.Army // ArmyPush
}
