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
	register(d, WH.HandleHWCreateCity)
	register(d, WH.HandleHWWorldMap)
	register(d, WH.HandleHWMyCities)
}

func register[Req any](
	d *Dispatcher,
	fn func(ctx actor.Context, p *WorldActor, req Req),
) {
	reqType := reflect.TypeOf((*Req)(nil)).Elem()
	if reqType == nil {
		panic("dispatcher req type cannot be nil")
	}

	d.handlers[reqType] = Handler{
		fn:      reflect.ValueOf(fn),
		reqType: reqType,
	}
}

func (d *Dispatcher) Dispatch(ctx actor.Context, p *WorldActor, req messages.WorldMessage) {
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
		reflect.ValueOf(p),
		reflect.ValueOf(req),
	})
}
