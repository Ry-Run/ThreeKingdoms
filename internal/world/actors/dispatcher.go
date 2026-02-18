package actors

import (
	worldpb "ThreeKingdoms/internal/shared/gen/world"
	"reflect"

	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/protobuf/proto"
)

type Dispatcher struct {
	handlers map[string]Handler
}

type Handler struct {
	fn      reflect.Value
	reqType reflect.Type
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
	register(d, WH.HandleNationMapConfigRequest)
}

func register[Req proto.Message](
	d *Dispatcher,
	fn func(ctx actor.Context, p *WorldActor, req Req),
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

func (d *Dispatcher) Dispatch(ctx actor.Context, p *WorldActor, req *worldpb.EmptyRequest) {
	if req == nil {
		ctx.Respond(fail("nil req"))
		return
	}

	body := unwrapWorldRequestBody(req)
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

func unwrapWorldRequestBody(req *worldpb.EmptyRequest) proto.Message {
	return req
}
