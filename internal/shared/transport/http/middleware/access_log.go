package middleware

import (
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/modules/kit/logx"
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type bodyCaptureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyCaptureWriter) Write(data []byte) (int, error) {
	_, _ = w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *bodyCaptureWriter) WriteString(s string) (int, error) {
	_, _ = w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// AccessLog 统一写访问日志，并尽量从响应体中的 `code` 字段提取业务码。
func AccessLog(log logx.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		action := c.Request.Method + " " + route

		ctx := transport.NewContextWithParent(c.Request.Context(), action)
		c.Request = c.Request.WithContext(ctx)

		bw := &bodyCaptureWriter{ResponseWriter: c.Writer}
		c.Writer = bw

		c.Next()

		if bizCode, ok := parseBizCode(bw.body.Bytes()); ok {
			transport.SetBizCode(ctx, transport.BizCode(bizCode))
		} else if c.Writer.Status() >= http.StatusBadRequest {
			transport.SetBizCode(ctx, transport.BizCode(transport.SystemError))
		} else {
			transport.SetBizCode(ctx, transport.BizCode(transport.OK))
		}

		transport.WriteAccessLog(ctx, log)
	}
}

func parseBizCode(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	// 优先按常见响应体格式解析：{"code":123, ...}
	var payload struct {
		Code *int `json:"code"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, false
	}
	if payload.Code == nil {
		return 0, false
	}
	return *payload.Code, true
}
