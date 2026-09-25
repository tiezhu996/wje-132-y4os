package model

import "time"

// RectificationTask 不合格检查项对应的整改任务。
// 每个不合格检查项最多一条任务（唯一索引），重复退回/提交都在同一条任务上流转，
// 历史动作保留在 rectification_histories 中。
type RectificationTask struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	InspectionID uint64    `gorm:"not null;index:idx_rect_tasks_inspection" json:"inspection_id"`
	ItemID       uint64    `gorm:"not null;uniqueIndex:uk_rect_tasks_item" json:"item_id"`
	ItemName     string    `gorm:"size:200;not null;default:''" json:"item_name"`
	AssigneeID   uint64    `gorm:"not null;index:idx_rect_tasks_assignee" json:"assignee_id"`
	Deadline     time.Time `gorm:"not null" json:"deadline"`
	Status       string    `gorm:"size:30;not null;default:pending;index:idx_rect_tasks_status" json:"status"`
	// 最近一次整改说明（退回后再次提交会覆盖；每次提交的完整记录见 RectificationHistory）。
	RectificationNote  string     `gorm:"size:500;not null;default:''" json:"rectification_note"`
	RectificationPhoto string     `gorm:"size:255;not null;default:''" json:"rectification_photo"`
	SubmittedAt        *time.Time `json:"submitted_at"`
	ReviewerID         uint64     `gorm:"not null;default:0" json:"reviewer_id"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	ReviewNote         string     `gorm:"size:500;not null;default:''" json:"review_note"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (RectificationTask) TableName() string { return "rectification_tasks" }

// RectificationHistory 整改任务流转记录：提交、退回、复查通过均追加一条，原记录不可改。
type RectificationHistory struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID     uint64    `gorm:"not null;index:idx_rect_hist_task" json:"task_id"`
	Action     string    `gorm:"size:30;not null" json:"action"`
	Note       string    `gorm:"size:500;not null;default:''" json:"note"`
	PhotoURL   string    `gorm:"size:255;not null;default:''" json:"photo_url"`
	OperatorID uint64    `gorm:"not null;default:0" json:"operator_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 指定表名。
func (RectificationHistory) TableName() string { return "rectification_histories" }
