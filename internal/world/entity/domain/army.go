package domain

import "time"

const (
	ArmyCmdIdle        = 0 //空闲
	ArmyCmdAttack      = 1 //攻击
	ArmyCmdDefend      = 2 //驻守
	ArmyCmdReclamation = 3 //屯垦
	ArmyCmdBack        = 4 //撤退
	ArmyCmdConscript   = 5 //征兵
	ArmyCmdTransfer    = 6 //调动
)

const (
	ArmyStop    = 0
	ArmyRunning = 1
)

// 军队
// entity
type Army struct {
	id                int    // order 第几编队 同 order
	cityId            CityID // 城市id
	playerId          PlayerID
	allianceId        AllianceID
	order             int8       // 第几队 1-5队
	generals          []*General // 将领详情
	soldiers          []int      // 士兵
	cmd               int8       // 命令  0:空闲 1:攻击 2：驻军 3:返回
	fromX             int        // 来自x坐标
	fromY             int        // 来自y坐标
	toX               int        // 去往x坐标
	toY               int        // 去往y坐标
	startTime         time.Time  // 出发时间
	endTime           time.Time  // 到达时间
	state             int8       // 状态: 0:running,1:stop
	conscriptEndTimes []int64    //征兵结束时间
	conscriptCounts   []int      //征兵数量
	cellX             int
	cellY             int
}
