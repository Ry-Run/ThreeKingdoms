package messages

type FailResp struct {
	Code    int
	Message string
}

type PlayerId int64

type WorldId int64

type HWPosition struct {
	PlayerId PlayerId
	WorldId  WorldId
}

type WHPosition struct {
	PlayerId PlayerId
	X, Y     int
}
