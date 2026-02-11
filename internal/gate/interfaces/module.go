package interfaces

import (
	"ThreeKingdoms/internal/gate/interfaces/handler"
	"ThreeKingdoms/internal/gate/interfaces/handler/http"
	ws2 "ThreeKingdoms/internal/gate/interfaces/handler/ws"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"ThreeKingdoms/internal/shared/session"
	transporthttp "ThreeKingdoms/internal/shared/transport/http"
	"ThreeKingdoms/internal/shared/transport/ws"

	"github.com/gin-gonic/gin"
)

type Module struct {
	wsHandler   *ws2.WsHandler
	httpHandler *http.HttpHandler
}

func New(s session.Manager, accountClient accountpb.AccountServiceClient) *Module {
	gate := handler.NewGate(s, accountClient)
	return &Module{
		wsHandler:   ws2.NewWsHandler(gate),
		httpHandler: http.NewHttpHandler(gate),
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
