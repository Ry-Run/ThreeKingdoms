package messages

type InternalMessage struct{}

func (InternalMessage) NotInfluenceReceiveTimeout() {}

type DCTick struct {
	InternalMessage
}

type Tick struct {
	InternalMessage
}
