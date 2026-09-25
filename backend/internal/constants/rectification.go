package constants

// RectificationTaskStatus 整改任务状态枚举。
const (
	RectTaskPending   = "pending"   // 已登记，待责任人整改
	RectTaskSubmitted = "submitted" // 责任人已提交整改说明，待复查
	RectTaskRejected  = "rejected"  // 复查不通过，退回重新整改
	RectTaskApproved  = "approved"  // 复查通过，不再计入待整改
)

// RectTaskStatusValues 全部整改任务状态值。
var RectTaskStatusValues = []string{RectTaskPending, RectTaskSubmitted, RectTaskRejected, RectTaskApproved}

// RectTaskFilterValues 列表状态筛选允许值（pending_rectification/overdue 为聚合筛选）。
var RectTaskFilterValues = append(append([]string{}, RectTaskStatusValues...), "pending_rectification", "overdue")

// RectHistoryAction 整改流转动作枚举。
const (
	RectActionRegister = "register" // 登记任务
	RectActionSubmit   = "submit"   // 提交整改说明
	RectActionReject   = "reject"   // 复查不通过，退回
	RectActionApprove  = "approve"  // 复查通过
)

// IsValidRectTaskStatus 校验整改任务状态。
func IsValidRectTaskStatus(s string) bool {
	for _, v := range RectTaskStatusValues {
		if v == s {
			return true
		}
	}
	return false
}
