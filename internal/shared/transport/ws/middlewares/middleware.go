package middlewares

import (
	"ThreeKingdoms/internal/shared/transport/ws"
	"context"
)

func Log() ws.MiddlewareFunc {
	return func(next ws.HandlerFunc) ws.HandlerFunc {
		return func(ctx context.Context, req *ws.WsMsgReq, resp *ws.WsMsgResp) {
			// 打印入站日志
			next(ctx, req, resp)
			// 打印出站日志
		}
	}
}

func Trace() ws.MiddlewareFunc {
	return func(next ws.HandlerFunc) ws.HandlerFunc {
		return func(ctx context.Context, req *ws.WsMsgReq, resp *ws.WsMsgResp) {
			// 加入 Trace 信息
			next(ctx, req, resp)
		}
	}
}
