package app

import (
	"ThreeKingdoms/internal/account/domain"
	"ThreeKingdoms/internal/account/dto"
	"ThreeKingdoms/internal/shared/security"
	"context"
	"errors"
	"time"
)

type UserRepo interface {
	GetUserByUserName(ctx context.Context, username string) (domain.User, error)
}

type LoginHistoryRepo interface {
	Save(ctx context.Context, history domain.LoginHistory) error
}

type LoginLastRepo interface {
	GetLoginLast(ctx context.Context, uid int) (domain.LoginLast, error)
	Save(ctx context.Context, ll domain.LoginLast) error
}

type PwdEncrypter func(pwd, passcode string) string

type UserService struct {
	userRepo     UserRepo
	pwdEncrypter PwdEncrypter
	log          Logger
	lhRepo       LoginHistoryRepo
	llRepo       LoginLastRepo
}

func NewUserService(userRepo UserRepo, pwdEncrypter PwdEncrypter, log Logger, loginHistoryRepo LoginHistoryRepo, llRepo LoginLastRepo) *UserService {
	return &UserService{
		userRepo:     userRepo,
		pwdEncrypter: pwdEncrypter,
		log:          log,
		lhRepo:       loginHistoryRepo,
		llRepo:       llRepo,
	}
}

// Login 处理登录流程
func (s *UserService) Login(ctx context.Context, req dto.LoginReq) (resp dto.LoginResp, err error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil {
		// 区分"用户不存在"（业务错误）和"数据库挂了"（技术错误）
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			return resp, ErrInvalidCredentials.WithData("reason", "用户不存在")
		// 其他系统错误：在接口层统一打印一次日志，这里只保留 cause 链用于溯源。
		default:
			return resp, ErrUnavailable.WithCause(err)
		}
	}
	checkRes := user.CheckPassword(req.Password, s.pwdEncrypter)
	if !checkRes {
		return resp, ErrInvalidCredentials.WithData("reason", "密码错误")
	}

	now := time.Now()
	token, err := security.Award(user.UId)
	if err != nil {
		return resp, ErrInternalServer.WithData("uid", user.UId).WithCause(err)
	}

	// 保存登录历史
	lh := domain.LoginHistory{UId: user.UId, CTime: now, Ip: req.Ip,
		Hardware: req.Hardware, State: domain.LoginSuccess} // todo 检查枚举值
	if err = s.lhRepo.Save(ctx, lh); err != nil {
		return resp, ErrUnavailable.WithCause(err)
	}

	// 保存最后一次登录的状态
	ll, err := s.llRepo.GetLoginLast(ctx, user.UId)
	switch {
	case err == nil:
		// 已存在：刷新状态
	case errors.Is(err, domain.ErrLastLoginNotFound):
		// 不存在：创建新记录（Id=0）
		ll = domain.LoginLast{UId: user.UId}
	default:
		return resp, ErrUnavailable.WithCause(err)
	}
	ll.LoginTime = now
	ll.Ip = req.Ip
	ll.Session = token
	ll.Hardware = req.Hardware
	ll.IsLogout = 0
	if err = s.llRepo.Save(ctx, ll); err != nil {
		return resp, ErrUnavailable.WithCause(err)
	}

	// 缓存 ws连接 和当前用户数据

	return dto.LoginResp{
		Username: user.Username,
		UId:      user.UId,
		Password: user.Passwd,
		Session:  token,
	}, nil
}
