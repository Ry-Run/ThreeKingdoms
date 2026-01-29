package errx

// 这里定义“跨服务统一”的系统类错误码。
//
// 约束：
// - 这些错误码用于“系统/技术类错误”归一化（便于告警、观测、跨服务排障）
// - 业务域错误码（例如 USER_NOT_FOUND）必须由各业务自行定义，不允许在 kit 里集中

const (
	// CodeInternal 表示服务内部不可预期错误（兜底）。
	CodeInternal Code = "INTERNAL_ERROR"
	// CodeUnavailable 表示依赖不可用/服务不可用（DB/Redis/下游服务/网络异常等）。
	CodeUnavailable Code = "SERVICE_UNAVAILABLE"
	// CodeTimeout 表示请求/依赖调用超时。
	CodeTimeout Code = "TIMEOUT"
	// CodeRateLimited 表示被限流/过载保护。
	CodeRateLimited Code = "RATE_LIMITED"
	// CodeMaintenance 表示服务维护/停服。
	CodeMaintenance Code = "MAINTENANCE"
	// 请求参数错误
	CodeReqParamError Code = "CODE_REQ_PARAM_ERROR"
)

// 统一系统类哨兵错误（允许 WithData/WithCause 派生新对象）。
var (
	ErrInternal    = NewSys(CodeInternal, "服务器内部错误")
	ErrUnavailable = NewSys(CodeUnavailable, "服务不可用")
	ErrTimeout     = NewSys(CodeTimeout, "请求超时")
	ErrRateLimited = NewSys(CodeRateLimited, "请求过于频繁")
	ErrMaintenance = NewSys(CodeMaintenance, "服务维护中")
	ErrReqParamERR = NewSys(CodeReqParamError, "请求参数错误")
)
