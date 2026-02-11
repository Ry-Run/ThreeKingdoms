package interfaces

import (
	"ThreeKingdoms/internal/account/infra/repo"
	"ThreeKingdoms/internal/account/interfaces/handler"
	"ThreeKingdoms/internal/shared/security"
	"ThreeKingdoms/internal/shared/utils"
	"ThreeKingdoms/modules/kit/logx"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Module struct {
	log        logx.Logger
	userRepo   *repo.UserRepo
	lhRepo     *repo.LoginHistoryRepo
	llRepo     *repo.LoginLastRepo
	pwdEncrypt func(pwd string, passcode string) string
	Account    *handler.Account
	randSeq    func(n int) string
	roleRepo   *repo.RoleRepo
}

func New(db *gorm.DB, l *zap.Logger) *Module {
	m := Module{
		log:        logx.NewZapLogger(l),
		userRepo:   repo.NewUserRepo(db),
		lhRepo:     repo.NewLoginHistoryRepo(db),
		llRepo:     repo.NewLoginLastRepo(db),
		pwdEncrypt: security.PwdEncrypt,
		randSeq:    utils.RandSeq,
		roleRepo:   repo.NewRoleRepo(db),
	}
	m.Account = handler.NewAccount(m.userRepo, m.pwdEncrypt, m.log, m.lhRepo, m.llRepo, m.randSeq, m.roleRepo)
	return &m
}
