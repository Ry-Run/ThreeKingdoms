package entity

type PlayerPersistSnapshot struct {
	Version      uint64
	Role         RoleEntitySnapshot
	Resource     ResourceEntitySnapshot
	SaveRole     bool
	SaveResource bool
}
