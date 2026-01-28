package login

import (
	net "ThreeKingdoms"
	"server/login/controller"
)

type Module struct{}

func New() *Module { return &Module{} }

func (m *Module) Register(r *net.Router) {
	controller.NewAccount().RegisterRoutes(r)
}
