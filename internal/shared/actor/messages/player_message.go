package messages

type PlayerMessage interface {
	PlayerID() int
}

type PlayerBaseMessage struct {
	PlayerId int
}

func (w PlayerBaseMessage) PlayerID() int {
	return w.PlayerId
}
