package server

import net "ThreeKingdoms"

type Service interface {
	Name() string
	Init() error
	RegisterRoutes(r net.Router)
}
