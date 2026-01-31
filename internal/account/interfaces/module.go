package interfaces

import (
	"ThreeKingdoms/internal/account/infra/logger"
	"ThreeKingdoms/internal/account/infra/repo"
	"ThreeKingdoms/internal/account/interfaces/handler"
	"ThreeKingdoms/internal/shared/security"
	"ThreeKingdoms/internal/shared/session"
	ws "ThreeKingdoms/internal/shared/transport/ws"
	"ThreeKingdoms/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Module struct {
	log        *logger.ZapLoggerAdapter
	session    session.Manager
	userRepo   *repo.UserRepo
	lhRepo     *repo.LoginHistoryRepo
	llRepo     *repo.LoginLastRepo
	pwdEncrypt func(pwd string, passcode string) string
	account    *handler.Account
	randSeq    func(n int) string
}

func New(db *gorm.DB, l *zap.Logger, session session.Manager) *Module {
	m := Module{
		log:        logger.NewZapLoggerAdapter(l),
		session:    session,
		userRepo:   repo.NewUserRepo(db),
		lhRepo:     repo.NewLoginHistoryRepo(db),
		llRepo:     repo.NewLoginLastRepo(db),
		pwdEncrypt: security.PwdEncrypt,
		randSeq:    utils.RandSeq,
	}
	m.account = handler.NewAccount(m.userRepo, m.pwdEncrypt, m.log, m.lhRepo, m.llRepo, m.session, m.randSeq)
	return &m
}

func (m *Module) WsRegister(r *ws.Router) {
	m.account.RegisterWsRoutes(r)
}

func (m *Module) HttpRegister(g *gin.RouterGroup) {
	m.account.RegisterHttpRoutes(g)

}
