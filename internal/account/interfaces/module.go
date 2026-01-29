package interfaces

import (
	"ThreeKingdoms/internal/account/infra/logger"
	"ThreeKingdoms/internal/account/infra/repo"
	"ThreeKingdoms/internal/account/interfaces/handler"
	"ThreeKingdoms/internal/shared/security"
	ws "ThreeKingdoms/internal/shared/transport/ws"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Module struct {
	db  *gorm.DB
	log *zap.Logger
}

func New(db *gorm.DB, l *zap.Logger) *Module {
	return &Module{
		db:  db,
		log: l,
	}
}

func (m *Module) Register(r *ws.Router) {
	userRepo := repo.NewUserRepo(m.db)
	lhRepo := repo.NewLoginHistoryRepo(m.db)
	llRepo := repo.NewLoginLastRepo(m.db)
	log := logger.NewZapLoggerAdapter(m.log)
	pwdEncrypt := security.PwdEncrypt
	handler.NewAccount(userRepo, pwdEncrypt, log, lhRepo, llRepo).RegisterRoutes(r)
}
