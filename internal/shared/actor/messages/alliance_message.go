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

type HAAllianceInfo struct {
	AllianceBaseMessage
}

type AHAllianceInfo struct {
	Alliance Alliance
}

type HAAllianceApplyList struct {
	AllianceBaseMessage
}

type AHAllianceApplyList struct {
	ApplyItem []ApplyItem
}
