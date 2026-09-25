package constants

// RectificationStatus 整改任务状态枚举。
const (
	RectificationPending   = "pending"   // 待整改
	RectificationSubmitted = "submitted" // 待复查
	RectificationReturned  = "returned"  // 复查退回
	RectificationApproved  = "approved"  // 复查通过
)

// RectificationStatusValues 全部整改任务状态值。
var RectificationStatusValues = []string{RectificationPending, RectificationSubmitted, RectificationReturned, RectificationApproved}

// RectificationAction 整改办理记录动作枚举。
const (
	RectificationActionAssign       = "assign"        // 登记责任人与期限
	RectificationActionSubmit       = "submit"        // 提交整改说明
	RectificationActionReviewPass   = "review_pass"   // 复查通过
	RectificationActionReviewReturn = "review_return" // 复查退回
)

// RectificationListFilter 整改任务列表筛选枚举。
const (
	RectificationFilterPending = "pending" // 待整改（未复查通过）
	RectificationFilterOverdue = "overdue" // 已逾期（未复查通过且超过整改期限）
)

// IsValidRectificationStatus 校验整改任务状态。
func IsValidRectificationStatus(s string) bool {
	for _, v := range RectificationStatusValues {
		if v == s {
			return true
		}
	}
	return false
}
