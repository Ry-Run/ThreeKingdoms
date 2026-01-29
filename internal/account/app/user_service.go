package app

import (
	"ThreeKingdoms/internal/account/domain"
	"ThreeKingdoms/internal/account/dto"
	"context"
	"errors"
)

type UserRepo interface {
	GetUserByUserName(ctx context.Context, username string) (domain.User, error)
}

type PwdEncrypter interface {
	Encrypt(pwd, passcode string) string
}

type UserService struct {
	userRepo     UserRepo
	pwdEncrypter PwdEncrypter
	log          Logger
}

func NewUserService(userRepo UserRepo, pwdEncrypter PwdEncrypter, log Logger) *UserService {
	return &UserService{
		userRepo:     userRepo,
		pwdEncrypter: pwdEncrypter,
		log:          log,
	}
}

// Login 处理登录流程
func (s *UserService) Login(ctx context.Context, req dto.LoginReq) (resp dto.LoginResp, err error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil {
		// 区分"用户不存在"（业务错误）和"数据库挂了"（技术错误）
		if errors.Is(err, domain.ErrUserNotFound) {
			return resp, ErrInvalidCredentials
		}
		// 其他系统错误：在接口层统一打印一次日志，这里只保留 cause 链用于溯源。
		return resp, ErrUnavailable.WithCause(err)
	}
	checkRes := user.CheckPassword(req.Password, s.pwdEncrypter.Encrypt)
	if !checkRes {
		return resp, ErrInvalidCredentials
	}

	return dto.LoginResp{
		Username: user.Username,
		UId:      user.UId,
		Password: user.Passwd,
	}, nil
}
