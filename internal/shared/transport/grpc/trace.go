package grpc

import (
	"ThreeKingdoms/modules/kit/tracex"
	"context"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	traceIDHeader = "x-trace-id"
	spanIDHeader  = "x-span-id"
)

// UnaryClientTraceInterceptor 为客户端 unary 请求自动注入 trace/span。
func UnaryClientTraceInterceptor() gogrpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *gogrpc.ClientConn,
		invoker gogrpc.UnaryInvoker,
		opts ...gogrpc.CallOption,
	) error {
		return invoker(injectTraceToOutgoing(ctx), method, req, reply, cc, opts...)
	}
}

// StreamClientTraceInterceptor 为客户端 stream 请求自动注入 trace/span。
func StreamClientTraceInterceptor() gogrpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *gogrpc.StreamDesc,
		cc *gogrpc.ClientConn,
		method string,
		streamer gogrpc.Streamer,
		opts ...gogrpc.CallOption,
	) (gogrpc.ClientStream, error) {
		return streamer(injectTraceToOutgoing(ctx), desc, cc, method, opts...)
	}
}

// UnaryServerTraceInterceptor 为服务端 unary 请求自动提取 trace/span。
func UnaryServerTraceInterceptor() gogrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *gogrpc.UnaryServerInfo,
		handler gogrpc.UnaryHandler,
	) (any, error) {
		return handler(extractTraceFromIncoming(ctx), req)
	}
}

// StreamServerTraceInterceptor 为服务端 stream 请求自动提取 trace/span。
func StreamServerTraceInterceptor() gogrpc.StreamServerInterceptor {
	return func(
		srv any,
		ss gogrpc.ServerStream,
		info *gogrpc.StreamServerInfo,
		handler gogrpc.StreamHandler,
	) error {
		ctx := extractTraceFromIncoming(ss.Context())
		return handler(srv, &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		})
	}
}

type wrappedServerStream struct {
	gogrpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func injectTraceToOutgoing(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID, ok := tracex.TraceIDFrom(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, traceIDHeader, traceID)
	}
	if spanID, ok := tracex.SpanIDFrom(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, spanIDHeader, spanID)
	}
	return ctx
}

func extractTraceFromIncoming(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	if values := md.Get(traceIDHeader); len(values) > 0 && values[0] != "" {
		ctx = tracex.WithTraceID(ctx, values[0])
	}
	if values := md.Get(spanIDHeader); len(values) > 0 && values[0] != "" {
		ctx = tracex.WithSpanID(ctx, values[0])
	}
	return ctx
}
