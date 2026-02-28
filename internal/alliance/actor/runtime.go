package actor

import (
	"ThreeKingdoms/internal/alliance/actors"
	"ThreeKingdoms/internal/alliance/service/port"
	"ThreeKingdoms/internal/shared/transport"
	"context"
	"errors"
	"time"

	protoactor "github.com/asynkron/protoactor-go/actor"
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
	system  *protoactor.ActorSystem
	root    *protoactor.RootContext
	manager *protoactor.PID
	timeout time.Duration
	ownSys  bool
}

func NewRuntime(repo port.AllianceRepository, askTimeout time.Duration) *Runtime {
	return NewRuntimeWithActorSystem(nil, repo, 0, askTimeout)
}

func NewRuntimeWithActorSystem(system *protoactor.ActorSystem, repo port.AllianceRepository, worldID int, askTimeout time.Duration) *Runtime {
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
		return actors.NewManagerActor(repo, worldID)
	})
	manager := root.Spawn(managerProps)

	return &Runtime{
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

// AllianceActorID 返回 alliance runtime 对外入口 actor 的 PID（当前为 ManagerActor）。
func (r *Runtime) AllianceActorID() *protoactor.PID {
	if r == nil {
		return nil
	}
	return r.manager
}

// AllianceActorId 兼容调用命名（非 Go 风格），内部转发到 AllianceActorID。
func (r *Runtime) AllianceActorId() *protoactor.PID {
	return r.AllianceActorID()
}

func (r *Runtime) ActorSystem() *protoactor.ActorSystem {
	if r == nil {
		return nil
	}
	return r.system
}

func (r *Runtime) request(pid *protoactor.PID, msg any, timeout time.Duration) (any, error) {
	if r == nil || r.root == nil {
		return nil, &RuntimeError{Code: transport.SystemError, Message: "actor runtime 未初始化"}
	}
	if pid == nil {
		return nil, &RuntimeError{Code: transport.SystemError, Message: "actor pid 为空"}
	}

	future := r.root.RequestFuture(pid, msg, timeout)
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
