package app

import (
	"ThreeKingdoms/internal/account/domain"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"ThreeKingdoms/internal/shared/security"
	"context"
	"errors"
	"time"
)

type UserService struct {
	userRepo     UserRepo
	pwdEncrypter PwdEncrypter
	lhRepo       LoginHistoryRepo
	llRepo       LoginLastRepo
	randSeq      RandSeq
}

func NewUserService(userRepo UserRepo, pwdEncrypter PwdEncrypter, loginHistoryRepo LoginHistoryRepo, llRepo LoginLastRepo, randSeq RandSeq) *UserService {
	return &UserService{
		userRepo:     userRepo,
		pwdEncrypter: pwdEncrypter,
		lhRepo:       loginHistoryRepo,
		llRepo:       llRepo,
		randSeq:      randSeq,
	}
}

// Login 处理登录流程
func (s *UserService) Login(ctx context.Context, req *accountpb.LoginRequest) (*accountpb.LoginReply, error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			return &accountpb.LoginReply{
				Ok:      false,
				Reason:  ReasonLoginInvalidCredentials.Code,
				Message: ReasonLoginInvalidCredentials.Message,
			}, nil
		default:
			return nil, ErrUnavailable.WithReason(ReasonUserRepoUnavailable).WithCause(err)
		}
	}
	checkRes := user.CheckPassword(req.Password, s.pwdEncrypter)
	if !checkRes {
		return &accountpb.LoginReply{
			Ok:      false,
			Reason:  ReasonLoginInvalidCredentials.Code,
			Message: ReasonLoginInvalidCredentials.Message,
		}, nil
	}

	now := time.Now()
	token, err := security.Award(user.UId)
	if err != nil {
		return nil, ErrInternalServer.WithReason(ReasonTokenIssue).WithCause(err)
	}

	// 保存登录历史
	lh := domain.LoginHistory{UId: user.UId, CTime: now, Ip: req.Ip,
		Hardware: req.Hardware, State: domain.LoginSuccess} // todo 检查枚举值
	if err = s.lhRepo.Save(ctx, lh); err != nil {
		return nil, ErrUnavailable.WithReason(ReasonLoginHistoryWriteFail).WithCause(err)
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
		return nil, ErrUnavailable.WithReason(ReasonLoginLastReadFail).WithCause(err)
	}
	ll.LoginTime = now
	ll.Ip = req.Ip
	ll.Session = token
	ll.Hardware = req.Hardware
	ll.IsLogout = 0
	if err = s.llRepo.Save(ctx, ll); err != nil {
		return nil, ErrUnavailable.WithReason(ReasonLoginLastWriteFail).WithCause(err)
	}

	return &accountpb.LoginReply{
		Ok:       true,
		Username: user.Username,
		Uid:      int32(user.UId),
		Session:  token,
	}, nil
}

func (s *UserService) Register(ctx context.Context, req *accountpb.RegisterRequest) (*accountpb.RegisterReply, error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil && errors.Is(err, domain.ErrSystemUnavailable) {
		return nil, ErrUnavailable.WithReason(ReasonUserRepoUnavailable).WithCause(err)
	}

	if user != nil {
		return &accountpb.RegisterReply{
			Ok:      false,
			Reason:  ReasonRegisterUserExist.Code,
			Message: ReasonRegisterUserExist.Message,
		}, nil
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
		return nil, ErrUnavailable.WithReason(ReasonUserCreateFail).WithCause(err)
	}
	return &accountpb.RegisterReply{
		Ok: true,
	}, nil
}
