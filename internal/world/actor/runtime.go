package actor

import (
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/internal/world/actors"
	"ThreeKingdoms/internal/world/app/port"
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
}

func NewRuntime(repo port.WorldRepository, askTimeout time.Duration) *Runtime {
	if askTimeout <= 0 {
		askTimeout = defaultAskTimeout
	}

	system := protoactor.NewActorSystem()
	root := system.Root
	managerProps := protoactor.PropsFromProducer(func() protoactor.Actor {
		return actors.NewManagerActor(repo)
	})
	manager := root.Spawn(managerProps)

	return &Runtime{
		system:  system,
		root:    root,
		manager: manager,
		timeout: askTimeout,
	}
}

func (r *Runtime) Shutdown() {
	if r == nil {
		return
	}
	if r.root != nil && r.manager != nil {
		r.root.Stop(r.manager)
	}
	if r.system != nil {
		r.system.Shutdown()
	}
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
