package messages

type AllianceMessage interface {
	WorldID() int
	AllianceID() int
	PlayerID() int
}

type AllianceBaseMessage struct {
	WorldId    int
	AllianceId int
	PlayerId   int
}

func (a *AllianceBaseMessage) WorldID() int {
	if a == nil {
		return 0
	}
	return a.WorldId
}

func (a *AllianceBaseMessage) AllianceID() int {
	if a == nil {
		return 0
	}
	return a.AllianceId
}

func (a *AllianceBaseMessage) PlayerID() int {
	if a == nil {
		return 0
	}
	return a.PlayerId
}

type HAAllianceList struct {
	AllianceBaseMessage
}

type AHAllianceList struct {
	List []Alliance
}

// AllianceSummaryUpsert 表示联盟摘要的增量上报事件。
// Version 用于幂等与乱序保护（仅接受更大版本）。
type AllianceSummaryUpsert struct {
	WorldId int
	Version uint64
	Summary Alliance
}
