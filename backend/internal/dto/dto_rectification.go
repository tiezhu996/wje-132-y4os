package dto

import (
	"time"

	"safetyplatform/internal/model"
)

// RectificationAssignRequest 登记整改责任人与期限请求。
type RectificationAssignRequest struct {
	AssigneeID uint64    `json:"assignee_id" binding:"required"`
	Deadline   time.Time `json:"deadline" binding:"required"`
}

// RectificationSubmitRequest 责任人提交整改说明请求。
type RectificationSubmitRequest struct {
	Note string `json:"note" binding:"required,max=500"`
}

// RectificationReviewRequest 复查请求：passed=true 复查通过，否则退回并需填写退回原因。
type RectificationReviewRequest struct {
	Passed bool   `json:"passed"`
	Note   string `json:"note" binding:"max=500"`
}

// RectificationTaskView 整改任务视图：附带检查名称、责任人姓名与逾期标记。
type RectificationTaskView struct {
	model.RectificationTask
	InspectionName string `json:"inspection_name"`
	AssigneeName   string `json:"assignee_name"`
	Overdue        bool   `json:"overdue"`
}
