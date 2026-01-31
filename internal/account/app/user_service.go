package app

import (
	"ThreeKingdoms/internal/account/app/model"
	"ThreeKingdoms/internal/account/domain"
	"ThreeKingdoms/internal/shared/security"
	"context"
	"errors"
	"time"
)

type UserRepo interface {
	GetUserByUserName(ctx context.Context, username string) (*domain.User, error)
	Save(ctx context.Context, n domain.User) error
}

type LoginHistoryRepo interface {
	Save(ctx context.Context, history domain.LoginHistory) error
}

type LoginLastRepo interface {
	GetLoginLast(ctx context.Context, uid int) (domain.LoginLast, error)
	Save(ctx context.Context, ll domain.LoginLast) error
}

type PwdEncrypter func(pwd, passcode string) string

type RandSeq func(n int) string

type UserService struct {
	userRepo     UserRepo
	pwdEncrypter PwdEncrypter
	log          Logger
	lhRepo       LoginHistoryRepo
	llRepo       LoginLastRepo
	randSeq      RandSeq
}

func NewUserService(userRepo UserRepo, pwdEncrypter PwdEncrypter, log Logger, loginHistoryRepo LoginHistoryRepo, llRepo LoginLastRepo, randSeq RandSeq) *UserService {
	return &UserService{
		userRepo:     userRepo,
		pwdEncrypter: pwdEncrypter,
		log:          log,
		lhRepo:       loginHistoryRepo,
		llRepo:       llRepo,
		randSeq:      randSeq,
	}
}

// Login 处理登录流程
func (s *UserService) Login(ctx context.Context, req model.LoginReq) (*model.LoginResp, error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil {
		// 区分"用户不存在"（业务错误）和"数据库挂了"（技术错误）
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			return nil, ErrInvalidCredentials.WithData("reason", "用户不存在")
		// 其他系统错误：在接口层统一打印一次日志，这里只保留 cause 链用于溯源。
		default:
			return nil, ErrUnavailable.WithCause(err)
		}
	}
	checkRes := user.CheckPassword(req.Password, s.pwdEncrypter)
	if !checkRes {
		return nil, ErrInvalidCredentials.WithData("reason", "密码错误")
	}

	now := time.Now()
	token, err := security.Award(user.UId)
	if err != nil {
		return nil, ErrInternalServer.WithData("uid", user.UId).WithCause(err)
	}

	// 保存登录历史
	lh := domain.LoginHistory{UId: user.UId, CTime: now, Ip: req.Ip,
		Hardware: req.Hardware, State: domain.LoginSuccess} // todo 检查枚举值
	if err = s.lhRepo.Save(ctx, lh); err != nil {
		return nil, ErrUnavailable.WithCause(err)
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
		return nil, ErrUnavailable.WithCause(err)
	}
	ll.LoginTime = now
	ll.Ip = req.Ip
	ll.Session = token
	ll.Hardware = req.Hardware
	ll.IsLogout = 0
	if err = s.llRepo.Save(ctx, ll); err != nil {
		return nil, ErrUnavailable.WithCause(err)
	}

	return &model.LoginResp{
		Username: user.Username,
		UId:      user.UId,
		Session:  token,
	}, nil
}

func (s *UserService) Register(ctx context.Context, req model.RegisterReq) error {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil && errors.Is(err, domain.ErrSystemUnavailable) {
		return ErrUnavailable.WithCause(err)
	}

	if user != nil {
		// 用户已存在
		return ErrUserExist
	}

	now := time.Now()
	passcode := s.randSeq(6)

	n := domain.User{
		Username: req.Username,
		Passwd:   s.pwdEncrypter(req.Password, passcode),
		Passcode: passcode,
		Mtime:    now,
		Ctime:    now,
		Hardware: req.Hardware,
	}
	if err = s.userRepo.Save(ctx, n); err != nil {
		return ErrUnavailable.WithCause(err)
	}
	return nil
}
