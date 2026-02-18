package entity

type PlayerID int

// entity
type Player struct {
	playerID *PlayerID
	profile  *RoleEntity
	resource *ResourceEntity
	X, Y     int
}

func NewPlayer(playerID *PlayerID, profile *RoleEntity, resource *ResourceEntity) *Player {
	return &Player{
		playerID: playerID,
		profile:  profile,
		resource: resource,
	}
}

func (p *Player) ID() PlayerID {
	return *p.playerID
}

func (p *Player) Profile() *RoleEntity {
	return p.profile
}

func (p *Player) Resource() *ResourceEntity {
	return p.resource
}

func (p *Player) Dirty() bool {
	if p == nil {
		return false
	}
	return p.profile.Dirty() || p.resource.Dirty()
}

func (p *Player) ClearDirty() {
	if p == nil {
		return
	}
	p.profile.ClearDirty()
	p.resource.ClearDirty()
}

func (p *Player) BuildPersistSnapshot(version uint64) (*PlayerPersistSnapshot, bool) {
	if p == nil {
		return nil, false
	}

	saveRole := p.profile != nil && p.profile.Dirty()
	saveResource := p.resource != nil && p.resource.Dirty()
	if !saveRole && !saveResource {
		return nil, false
	}

	s := &PlayerPersistSnapshot{
		Version:      version,
		SaveRole:     saveRole,
		SaveResource: saveResource,
	}
	if saveRole {
		s.Role = p.profile.Snapshot()
	}
	if saveResource {
		s.Resource = p.resource.Snapshot()
	}
	return s, true
}
