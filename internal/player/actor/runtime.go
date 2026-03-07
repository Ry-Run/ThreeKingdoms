package actor

import (
	"ThreeKingdoms/internal/player/actors"
	"ThreeKingdoms/internal/player/service/port"
	sharedactor "ThreeKingdoms/internal/shared/actor"
	gatepb "ThreeKingdoms/internal/shared/gen/gate"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/transport"
	"context"
	"errors"
	"time"

	protoactor "github.com/asynkron/protoactor-go/actor"
	"go.uber.org/zap"
)

const defaultAskTimeout = 3 * time.Second

type RuntimeError struct {
	Code    int
	Message string
	Cause   error
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *RuntimeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type Runtime struct {
	logger  *zap.Logger
	system  *protoactor.ActorSystem
	root    *protoactor.RootContext
	manager *protoactor.PID
	timeout time.Duration
	ownSys  bool
}

func NewRuntime(
	logger *zap.Logger,
	system *protoactor.ActorSystem,
	repo port.PlayerRepository,
	resolver sharedactor.ManagerPIDResolver,
	pusher gatepb.GatePushServiceClient,
	askTimeout time.Duration,
) *Runtime {
	if askTimeout <= 0 {
		askTimeout = defaultAskTimeout
	}

	ownSys := false
	if system == nil {
		system = protoactor.NewActorSystem()
		ownSys = true
	}
	root := system.Root
	managerProps := protoactor.PropsFromProducer(func() protoactor.Actor {
		return actors.NewManagerActor(repo, resolver, pusher)
	})
	manager := root.Spawn(managerProps)

	return &Runtime{
		logger:  logger,
		system:  system,
		root:    root,
		manager: manager,
		timeout: askTimeout,
		ownSys:  ownSys,
	}
}

func (r *Runtime) Shutdown() {
	if r == nil {
		return
	}
	if r.root != nil && r.manager != nil {
		r.root.Stop(r.manager)
	}
	if r.ownSys && r.system != nil {
		r.system.Shutdown()
	}
}

// PlayerMangerPID 返回 player runtime 对外入口 actor 的 PID（当前为 ManagerActor）。
func (r *Runtime) PlayerMangerPID() *protoactor.PID {
	if r == nil {
		return nil
	}
	return r.manager
}

func (r *Runtime) request(pid *protoactor.PID, msg any, timeout time.Duration) (any, error) {
	if r == nil || r.root == nil {
		return nil, &RuntimeError{Code: transport.SystemError, Message: "actor runtime 未初始化"}
	}
	if pid == nil {
		return nil, &RuntimeError{Code: transport.SystemError, Message: "actor pid 为空"}
	}

	// 创建并注册一个 futureProcess（拿到一个 PID 作为 Sender），发送 msg 给目标 pid，设置 timeout，返回 Future
	future := r.root.RequestFuture(pid, msg, timeout)
	// 阻塞等待：直到 future 收到对方回复（通过 Respond 回到 future PID）或超时（ErrTimeout），再返回结果/错误
	res, err := future.Result()
	if err != nil {
		return nil, &RuntimeError{
			Code:    transport.SystemError,
			Message: "actor 请求失败",
			Cause:   err,
		}
	}
	return res, nil
}

func (r *Runtime) timeoutFromContext(ctx context.Context) time.Duration {
	if r == nil || r.timeout <= 0 {
		return defaultAskTimeout
	}
	if ctx == nil {
		return r.timeout
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return r.timeout
	}
	remain := time.Until(deadline)
	if remain <= 0 {
		return time.Millisecond
	}
	if remain < r.timeout {
		return remain
	}
	return r.timeout
}

func (r *Runtime) Handle(ctx context.Context, req *playerpb.PlayerRequest) (*playerpb.PlayerResponse, error) {
	if req == nil {
		return nil, &RuntimeError{
			Code:    transport.InvalidParam,
			Message: "player request 不能为空",
		}
	}

	res, err := r.request(r.manager, req, r.timeoutFromContext(ctx))
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*playerpb.PlayerResponse)
	if !ok {
		return nil, &RuntimeError{
			Code:    transport.SystemError,
			Message: "actor 返回类型非法",
		}
	}
	return resp, nil
}

func CodeFromError(err error) int {
	if err == nil {
		return transport.OK
	}
	var re *RuntimeError
	if errors.As(err, &re) && re != nil && re.Code != 0 {
		return re.Code
	}
	return transport.SystemError
}
