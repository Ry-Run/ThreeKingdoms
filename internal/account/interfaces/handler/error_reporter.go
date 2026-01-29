package handler

import (
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/modules/kit/logx"

	"go.uber.org/zap"
)

type ErrorLog = logx.ErrorLog

// BuildErrorLog 把“错误码/语义/上下文/cause链/发生处栈”提取成便于阅读的结构，用于接口层统一打印。
func BuildErrorLog(err error) ErrorLog {
	return logx.BuildErrorLog(err)
}

// ReportError 在接口层打印一次“易读”的错误日志：建议每个请求/消息只调用一次。
func ReportError(msg string, err error, fields ...zap.Field) {
	if err == nil {
		return
	}
	logx.ReportError(logs.Logger(), msg, err, fields...)
}
