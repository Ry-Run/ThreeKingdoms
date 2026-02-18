package entity

type WorldID int

type World struct {
	worldID             *WorldID
	nationMapConfigJSON string
	dirty               bool
}

func NewWorld(worldID *WorldID, nationMapConfigJSON string) *World {
	id := worldID
	if id == nil {
		defaultID := WorldID(0)
		id = &defaultID
	}
	if nationMapConfigJSON == "" {
		nationMapConfigJSON = "{}"
	}
	return &World{
		worldID:             id,
		nationMapConfigJSON: nationMapConfigJSON,
	}
}

func (w *World) ID() WorldID {
	return *w.worldID
}

func (w *World) NationMapConfigJSON() string {
	return w.nationMapConfigJSON
}

func (w *World) SetNationMapConfigJSON(payload string) {
	if payload == "" {
		payload = "{}"
	}
	if w.nationMapConfigJSON == payload {
		return
	}
	w.nationMapConfigJSON = payload
	w.dirty = true
}

func (w *World) Dirty() bool {
	return w.dirty
}

func (w *World) ClearDirty() {
	w.dirty = false
}

func (w *World) BuildPersistSnapshot(version uint64) (*WorldPersistSnapshot, bool) {
	if w == nil || !w.Dirty() {
		return nil, false
	}

	return &WorldPersistSnapshot{
		Version:             version,
		WorldID:             w.ID(),
		NationMapConfigJSON: w.NationMapConfigJSON(),
	}, true
}
