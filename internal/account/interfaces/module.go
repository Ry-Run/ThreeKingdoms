package interfaces

import (
	"ThreeKingdoms/internal/account/interfaces/handler"
	ws "ThreeKingdoms/internal/shared/transport/ws"
)

type Module struct{}

func New() *Module { return &Module{} }

func (m *Module) Register(r *ws.Router) {
	handler.NewAccount().RegisterRoutes(r)
}
