package actors

import (
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"reflect"

	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/protobuf/proto"
)

type Dispatcher struct {
	handlers map[string]Handler
}

type Handler struct {
	fn      reflect.Value // handler 函数
	reqType reflect.Type  // 请求类型
}

func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		handlers: make(map[string]Handler),
	}
	d.registerAll()
	return d
}

func protoMessageName(msg proto.Message) string {
	return string(proto.MessageName(msg))
}

func (d *Dispatcher) registerAll() {
	register(d, PH.HandleEnterServerRequest)
	register(d, PH.HandleCreateRole)
	register(d, PH.HandleWorldMapRequest)
	register(d, PH.HandleMyPropertyRequest)
	register(d, PH.HandleMyGeneralsRequest)
}

// register 注册统一分发函数，要求 Req/Rep 都是 protobuf 指针消息。
func register[Req proto.Message](
	d *Dispatcher,
	fn func(ctx actor.Context, p *PlayerActor, req Req),
) {
	reqType := reflect.TypeOf((*Req)(nil)).Elem()
	if reqType.Kind() != reflect.Ptr {
		panic("dispatcher req type must be pointer message")
	}
	reqName := protoMessageName(reflect.New(reqType.Elem()).Interface().(proto.Message))

	d.handlers[reqName] = Handler{
		fn:      reflect.ValueOf(fn),
		reqType: reqType,
	}
}

func (d *Dispatcher) Dispatch(ctx actor.Context, p *PlayerActor, req *playerpb.PlayerRequest) {
	if req == nil {
		ctx.Respond(fail("nil req"))
		return
	}

	body := unwrapPlayerRequestBody(req)
	if body == nil {
		ctx.Respond(fail("empty request body"))
		return
	}

	handler, ok := d.handlers[protoMessageName(body)]
	if !ok {
		ctx.Respond(fail("no handler for request body"))
		return
	}

	if reflect.TypeOf(body) != handler.reqType {
		ctx.Respond(fail("request body type mismatch"))
		return
	}

	handler.fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(p),
		reflect.ValueOf(body),
	})
}

func unwrapPlayerRequestBody(req *playerpb.PlayerRequest) proto.Message {
	switch body := req.GetBody().(type) {
	case *playerpb.PlayerRequest_EnterServerRequest:
		return body.EnterServerRequest
	case *playerpb.PlayerRequest_CreateRoleRequest:
		return body.CreateRoleRequest
	case *playerpb.PlayerRequest_WorldMapRequest:
		return body.WorldMapRequest
	case *playerpb.PlayerRequest_MyPropertyRequest:
		return body.MyPropertyRequest
	case *playerpb.PlayerRequest_MyGeneralsRequest:
		return body.MyGeneralsRequest
	default:
		return nil
	}
}
