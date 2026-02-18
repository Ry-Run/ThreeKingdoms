package errs

import "fmt"

type Kind string

const (
	KindUnknown    Kind = "unknown"
	KindInfra      Kind = "infra"
	KindDependency Kind = "dependency"
	KindBusiness   Kind = "business"
)

type Error struct {
	Op    string         // 发生位置：repo.GetRoleByRID / usecase.CreateRole
	Kind  Kind           // 粗分类
	Meta  map[string]any // 关键参数（rid, uid...）
	Cause error          // 根因（必须保留）
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Op
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }

// Wrap：统一包装入口
func Wrap(op string, kind Kind, cause error, meta map[string]any) error {
	if cause == nil {
		return nil
	}
	return &Error{Op: op, Kind: kind, Cause: cause, Meta: meta}
}
