package messages

import (
	"time"
)

type Pos struct {
	X int
	Y int
}

type WorldCity struct {
	PlayerId   int
	CityId     int64
	Name       string
	Pos        Pos
	IsMain     bool
	Level      int8
	CurDurable int
	MaxDurable int
	OccupyTime int64 // 毫秒时间戳
	UnionId    int
	UnionName  string
	ParentId   int
}

type Building struct {
	PlayerId   int
	Id         int // cell 的 index
	Type       int8
	Level      int8
	OPLevel    int8
	Pos        Pos
	Name       string
	Wood       int
	Iron       int
	Stone      int
	Grain      int
	Defender   int
	CurDurable int
	MaxDurable int
	OccupyTime time.Time
	EndTime    time.Time
	GiveUpTime time.Time
	RNick      string
	UnionId    int
	UnionName  string
	ParentId   int
}

type Army struct {
	Id        int
	CityId    int
	UnionId   int  //联盟id
	Order     int8 //第几队，1-5队
	Generals  [3]int
	Soldiers  [3]int
	ConTimes  [3]int64
	ConCounts [3]int
	Cmd       int8
	State     int8 //状态:1=行军中(running)，0=停留/驻守(stop)
	FromPos   Pos
	ToPos     Pos
	Start     int64 //出征开始时间
	End       int64 //出征结束时间
}
