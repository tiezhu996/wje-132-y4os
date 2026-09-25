package dto

import "time"

// RectTaskRegisterRequest 登记整改任务请求（不合格检查项 -> 责任人 + 期限）。
type RectTaskRegisterRequest struct {
	InspectionID uint64    `json:"inspection_id" binding:"required"`
	ItemID       uint64    `json:"item_id" binding:"required"`
	AssigneeID   uint64    `json:"assignee_id" binding:"required"`
	Deadline     time.Time `json:"deadline" binding:"required"`
}

// RectTaskSubmitRequest 责任人提交整改说明。
type RectTaskSubmitRequest struct {
	RectificationNote  string `json:"rectification_note" binding:"required,max=500"`
	RectificationPhoto string `json:"rectification_photo" binding:"max=255"`
}

// RectTaskReviewRequest 复查：通过/退回（退回须填写意见）。
type RectTaskReviewRequest struct {
	Approved   bool   `json:"approved"`
	ReviewNote string `json:"review_note" binding:"max=500"`
}

// RectTaskListQuery 整改任务列表查询。
type RectTaskListQuery struct {
	PageQuery
	// Status：pending/submitted/rejected/approved；
	// pending_rectification 待整改（未复查通过）；overdue 已逾期（未通过且超期）。
	Status string `form:"status"`
	// Mine=1 仅看分配给当前用户的任务。
	Mine string `form:"mine"`
}
