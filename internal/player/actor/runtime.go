package actor

import (
	"ThreeKingdoms/internal/player/actors"
	"ThreeKingdoms/internal/player/service/port"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
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
}

func NewRuntime(repo port.PlayerRepository, askTimeout time.Duration) *Runtime {
	if askTimeout <= 0 {
		askTimeout = defaultAskTimeout
	}

	/**
	创建一个新的 ActorSystem，相当于容器/运行时环境：
	管理 PID、调度、邮箱、系统消息
	挂载扩展（remote/cluster/metrics 等）
	提供 root context 等入口
	*/
	system := protoactor.NewActorSystem()
	/**
	拿到 ActorSystem 的 根上下文。主要用来：
	Spawn 创建 actor
	Send/Request/Stop 等操作
	它代表“系统外部（或顶层）对 actor 的操作入口”。
	*/
	root := system.Root
	/**
	用 root context 创建（spawn） 一个 actor，返回它的 PID。
	PropsFromProducer(...) 表示：用一个“工厂函数”来生成 actor 实例。
	这样每次需要创建 actor 时，框架会调用这个 producer 来 new 一个出来。
	**/
	managerProps := protoactor.PropsFromProducer(func() protoactor.Actor {
		// 具体返回的 actor 实例
		return actors.NewManagerActor(repo)
	})
	/**
	配置这个 actor 的 Props（行为+邮箱+中间件等）。
	manager：作为一个“管理/协调 actor”的 PID，供 Runtime 后续使用。
	manager 不能干重活，路由、查表、维护元数据可以
	*/
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
