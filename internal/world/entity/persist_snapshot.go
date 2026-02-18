package entity

type WorldPersistSnapshot struct {
	Version             uint64
	WorldID             WorldID
	NationMapConfigJSON string
}
