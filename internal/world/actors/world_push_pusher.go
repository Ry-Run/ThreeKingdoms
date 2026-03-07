package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	gatepb "ThreeKingdoms/internal/shared/gen/gate"
	"context"
	"fmt"
)

type WorldPushBatchPusher interface {
	PushWorldPushBatch(ctx context.Context, batch *messages.WorldPushBatch) error
}

type GRPCWorldPushBatchPusher struct {
	client gatepb.GatePushServiceClient
}

func NewGRPCWorldPushBatchPusher(client gatepb.GatePushServiceClient) *GRPCWorldPushBatchPusher {
	if client == nil {
		return nil
	}
	return &GRPCWorldPushBatchPusher{client: client}
}

func (p *GRPCWorldPushBatchPusher) PushWorldPushBatch(ctx context.Context, batch *messages.WorldPushBatch) error {
	if p == nil || p.client == nil || batch == nil || len(batch.Items) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	items := make([]*gatepb.WorldPushItem, 0, len(batch.Items))
	for _, item := range batch.Items {
		if item.Army == nil {
			continue
		}
		items = append(items, &gatepb.WorldPushItem{
			PlayerId: item.PlayerID,
			Army:     item.Army,
		})
	}
	if len(items) == 0 {
		return nil
	}
	resp, err := p.client.PushWorldBatch(ctx, &gatepb.PushWorldBatchRequest{
		WorldId: int32(batch.WorldId),
		MsgType: string(batch.MsgType),
		Items:   items,
	})
	if err != nil {
		return fmt.Errorf("push world batch by grpc: %w", err)
	}
	if resp == nil || !resp.Ok {
		return fmt.Errorf("push world batch reply not ok")
	}
	return nil
}
