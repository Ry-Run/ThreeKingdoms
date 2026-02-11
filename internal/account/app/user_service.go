package app

import (
	"ThreeKingdoms/internal/account/domain"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
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
	roleRepo     RoleRepo
}

func NewUserService(userRepo UserRepo, pwdEncrypter PwdEncrypter, loginHistoryRepo LoginHistoryRepo, llRepo LoginLastRepo, randSeq RandSeq, roleRepo RoleRepo) *UserService {
	return &UserService{
		userRepo:     userRepo,
		pwdEncrypter: pwdEncrypter,
		lhRepo:       loginHistoryRepo,
		llRepo:       llRepo,
		randSeq:      randSeq,
		roleRepo:     roleRepo,
	}
}

// Login 处理登录流程
func (s *UserService) Login(ctx context.Context, req *accountpb.LoginRequest) (*accountpb.LoginReply, error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			return &accountpb.LoginReply{
				Result: fail(ReasonLoginInvalidCredentials),
			}, nil
		default:
			return nil, ErrUnavailable.WithReason(ReasonUserRepoUnavailable).WithCause(err)
		}
	}
	checkRes := user.CheckPassword(req.Password, s.pwdEncrypter)
	if !checkRes {
		return &accountpb.LoginReply{
			Result: fail(ReasonLoginInvalidCredentials),
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
		Username: user.Username,
		Uid:      int32(user.UId),
		Session:  token,
		Result:   ok(),
	}, nil
}

func (s *UserService) Register(ctx context.Context, req *accountpb.RegisterRequest) (*accountpb.RegisterReply, error) {
	user, err := s.userRepo.GetUserByUserName(ctx, req.Username)
	if err != nil && errors.Is(err, domain.ErrSystemUnavailable) {
		return nil, ErrUnavailable.WithReason(ReasonUserRepoUnavailable).WithCause(err)
	}

	if user != nil {
		return &accountpb.RegisterReply{
			Result: fail(ReasonRegisterUserExist),
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
		Result: ok(),
	}, nil
}

func (s *UserService) EnterServer(ctx context.Context, req *accountpb.EnterServerRequest) (*accountpb.EnterServerReply, error) {
	reply := accountpb.EnterServerReply{}
	role, err := s.roleRepo.GetRoleByUid(ctx, int(req.Uid))

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRoleNotFound):
			return &accountpb.EnterServerReply{
				Result: fail(ReasonRoleNotExist),
			}, nil
		default:
			return nil, ErrUnavailable.WithReason(ReasonRoleRepoUnavailable).WithCause(err)
		}
	}

	reply.Token, err = security.Award(role.RId)
	if err != nil {
		return nil, ErrInternalServer.WithReason(ReasonTokenIssue).WithCause(err)
	}
	reply.Result = ok()
	reply.Time = time.Now().UnixNano() / 1e6
	reply.Role = &accountpb.Role{
		Rid:      int32(role.RId),
		Uid:      int32(role.UId),
		NickName: role.NickName,
		Sex:      int32(role.Sex),
		Balance:  int32(role.Balance),
		HeadId:   int32(role.HeadId),
		Profile:  role.Profile,
	}
	return &reply, nil
}

func ok() *commonpb.BizResult {
	return &commonpb.BizResult{Ok: true}
}

func fail(reason Reason) *commonpb.BizResult {
	return &commonpb.BizResult{
		Ok:      false,
		Reason:  reason.Code,
		Message: reason.Message,
	}
}
