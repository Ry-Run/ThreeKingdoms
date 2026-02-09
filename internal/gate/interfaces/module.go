package interfaces

import (
	"ThreeKingdoms/internal/gate/interfaces/handler"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"ThreeKingdoms/internal/shared/session"
	transporthttp "ThreeKingdoms/internal/shared/transport/http"
	"ThreeKingdoms/internal/shared/transport/ws"

	"github.com/gin-gonic/gin"
)

type Module struct {
	wsHandler   *handler.WsHandler
	httpHandler *handler.HttpHandler
}

func New(s session.Manager, accountClient accountpb.AccountServiceClient) *Module {
	gate := handler.NewGate(s, accountClient)
	return &Module{
		wsHandler:   handler.NewWsHandler(gate),
		httpHandler: handler.NewHttpHandler(gate),
	}
}

func (m *Module) WsRegister(r *ws.Router) {
	m.wsHandler.RegisterRoutes(r)
}

func (m *Module) HttpRegister(g *gin.RouterGroup) {
	m.httpHandler.RegisterRoutes(g)
}

var _ ws.Registrar = (*Module)(nil)
var _ transporthttp.Registrar = (*Module)(nil)
