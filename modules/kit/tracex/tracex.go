package tracex

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type traceIDKey struct{}
type spanIDKey struct{}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func TraceIDFrom(ctx context.Context) (string, bool) {
	v := ctx.Value(traceIDKey{})
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey{}, spanID)
}

func SpanIDFrom(ctx context.Context) (string, bool) {
	v := ctx.Value(spanIDKey{})
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

// NewTraceID 生成 16 字节随机 trace_id（hex）。
func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
