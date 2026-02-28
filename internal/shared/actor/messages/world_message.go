package messages

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
	NickName string
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
