package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ThreeKingdoms/internal/account/domain"
	"ThreeKingdoms/internal/account/dto"
	"ThreeKingdoms/internal/shared/security"

	"go.uber.org/zap"
)

type fakeUserRepo struct {
	user domain.User
	err  error
}

func (r fakeUserRepo) GetUserByUserName(ctx context.Context, username string) (domain.User, error) {
	return r.user, r.err
}

type fakeHistoryRepo struct {
	calls    int
	lastSave domain.LoginHistory
	err      error
}

func (r *fakeHistoryRepo) Save(ctx context.Context, history domain.LoginHistory) error {
	r.calls++
	r.lastSave = history
	return r.err
}

type fakeLastRepo struct {
	getResult domain.LoginLast
	getErr    error

	saveCalls int
	lastSave  domain.LoginLast
	saveErr   error
}

func (r *fakeLastRepo) GetLoginLast(ctx context.Context, uid int) (domain.LoginLast, error) {
	return r.getResult, r.getErr
}

func (r *fakeLastRepo) Save(ctx context.Context, ll domain.LoginLast) error {
	r.saveCalls++
	r.lastSave = ll
	return r.saveErr
}

type nopLogger struct{}

func (nopLogger) WithContext(ctx context.Context) Logger { return nopLogger{} }
func (nopLogger) Info(msg string, fields ...zap.Field)   {}
func (nopLogger) Error(msg string, fields ...zap.Field)  {}
func (nopLogger) Debug(msg string, fields ...zap.Field)  {}
func (nopLogger) Warn(msg string, fields ...zap.Field)   {}

func TestLogin_Award失败应返回系统错误且不写库(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	user := domain.User{UId: 1, Username: "u", Passwd: "pwd"}
	hRepo := &fakeHistoryRepo{}
	lRepo := &fakeLastRepo{getErr: domain.ErrLastLoginNotFound}

	s := NewUserService(
		fakeUserRepo{user: user},
		func(pwd, passcode string) string { return pwd },
		nopLogger{},
		hRepo,
		lRepo,
	)

	_, err := s.Login(context.Background(), dto.LoginReq{Username: "u", Password: "pwd"})
	if err == nil {
		t.Fatalf("期望返回错误")
	}
	if !errors.Is(err, ErrInternalServer) {
		t.Fatalf("期望返回系统错误 ErrInternalServer, got=%v", err)
	}
	if !errors.Is(err, security.ErrJWTSecretMissing) {
		t.Fatalf("期望保留 JWT_SECRET 缺失的 cause 链, got=%v", err)
	}
	if hRepo.calls != 0 || lRepo.saveCalls != 0 {
		t.Fatalf("期望 Award 失败时不写 login_history/login_last, history=%d last=%d", hRepo.calls, lRepo.saveCalls)
	}
}

func TestLogin_应更新login_last并写入成功状态(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-123")

	user := domain.User{UId: 42, Username: "u", Passwd: "pwd"}
	hRepo := &fakeHistoryRepo{}
	exist := domain.LoginLast{Id: 7, UId: 42, Session: "old", LoginTime: time.Unix(1, 0)}
	lRepo := &fakeLastRepo{getResult: exist, getErr: nil}

	s := NewUserService(
		fakeUserRepo{user: user},
		func(pwd, passcode string) string { return pwd },
		nopLogger{},
		hRepo,
		lRepo,
	)

	resp, err := s.Login(context.Background(), dto.LoginReq{Username: "u", Password: "pwd", Ip: "1.1.1.1", Hardware: "h"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if resp.Session == "" {
		t.Fatalf("期望 Session 非空")
	}
	if hRepo.calls != 1 {
		t.Fatalf("期望写入一次 login_history, got=%d", hRepo.calls)
	}
	if hRepo.lastSave.State != domain.LoginSuccess {
		t.Fatalf("期望 login_history.State 表示成功，got=%d", hRepo.lastSave.State)
	}
	if lRepo.saveCalls != 1 {
		t.Fatalf("期望 upsert 一次 login_last, got=%d", lRepo.saveCalls)
	}
	if lRepo.lastSave.Id != 7 {
		t.Fatalf("期望更新而非插入新记录（保留原 Id），got=%d", lRepo.lastSave.Id)
	}
	if lRepo.lastSave.Session == "" || lRepo.lastSave.Session == "old" {
		t.Fatalf("期望更新 session, got=%q", lRepo.lastSave.Session)
	}
	if lRepo.lastSave.LoginTime.IsZero() {
		t.Fatalf("期望更新 login_time")
	}
}
