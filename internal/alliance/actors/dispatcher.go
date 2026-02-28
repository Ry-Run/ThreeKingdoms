package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	"reflect"

	"github.com/asynkron/protoactor-go/actor"
)

type Dispatcher struct {
	handlers map[reflect.Type]Handler
}

type Handler struct {
	fn      reflect.Value
	reqType reflect.Type
}

func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		handlers: make(map[reflect.Type]Handler),
	}
	d.registerAll()
	return d
}

func (d *Dispatcher) registerAll() {
}

func register[Req messages.AllianceMessage](
	d *Dispatcher,
	fn func(ctx actor.Context, a *AllianceActor, req Req),
) {
	reqType := reflect.TypeOf((*Req)(nil)).Elem()
	if reqType == nil {
		panic("dispatcher req type cannot be nil")
	}
	if reqType.Kind() != reflect.Ptr {
		panic("alliance dispatcher req type must be pointer message")
	}

	d.handlers[reqType] = Handler{
		fn:      reflect.ValueOf(fn),
		reqType: reqType,
	}
}

func (d *Dispatcher) Dispatch(ctx actor.Context, a *AllianceActor, req messages.AllianceMessage) {
	if req == nil {
		ctx.Respond("nil req")
		return
	}

	bodyType := reflect.TypeOf(req)
	handler, ok := d.handlers[bodyType]
	if !ok {
		ctx.Respond("no handler for request body")
		return
	}

	if bodyType != handler.reqType {
		ctx.Respond("request body type mismatch")
		return
	}

	handler.fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(a),
		reflect.ValueOf(req),
	})
}
