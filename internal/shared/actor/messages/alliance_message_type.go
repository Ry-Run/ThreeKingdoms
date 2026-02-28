package messages

// 联盟职位
type AllianceTitle int32

const (
	ALLIANCE_CHAIRMAN      AllianceTitle = 0 // 盟主
	ALLIANCE_VICE_CHAIRMAN AllianceTitle = 1 // 副盟主
	ALLIANCE_COMMON        AllianceTitle = 2 // 普通成员
)

// 申请状态
type AllianceApplyStatus int32

const (
	ALLIANCE_UNTREATED AllianceApplyStatus = 0 // 未处理
	ALLIANCE_REFUSE    AllianceApplyStatus = 1 // 拒绝
	ALLIANCE_ADOPT     AllianceApplyStatus = 2 // 通过
)

type Alliance struct {
	// 联盟摘要：当前用于联盟列表；后续可按业务演进持续补充字段。
	Id     int32
	Name   string
	Cnt    int32
	Notice string
	Major  []*Major
}

type Major struct {
	Rid   int32
	Name  string
	Title AllianceTitle
}

type Member struct {
	Rid   int32
	Name  string
	Title AllianceTitle
	Pos   Pos
}

// AllianceSummaryUpsert 表示联盟摘要的增量上报事件。
// Version 用于幂等与乱序保护（仅接受更大版本）。
type AllianceSummaryUpsert struct {
	WorldId int
	Version uint64
	Summary Alliance
}

type ApplyItem struct {
	PlayerId int
	NickName string
}
