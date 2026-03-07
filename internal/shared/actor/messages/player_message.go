package messages

type PlayerMessage interface {
	PlayerID() int
	WorldID() int
}

type PlayerBaseMessage struct {
	InternalMessage
	WorldId  int
	PlayerId int
}

func (p *PlayerBaseMessage) PlayerID() int {
	return p.PlayerId
}

func (p *PlayerBaseMessage) WorldID() int {
	return p.WorldId
}

type WHWarReport struct {
	PlayerBaseMessage
	WarReport WarReport
}

type WHBattleResult struct {
	PlayerBaseMessage
	Army *Army
}
