package tracex

import (
	"context"
	"testing"
)

func TestTraceID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "t-1")
	if got, ok := TraceIDFrom(ctx); !ok || got != "t-1" {
		t.Fatalf("期望 TraceIDFrom round-trip 成功，got=%q ok=%v", got, ok)
	}
}
