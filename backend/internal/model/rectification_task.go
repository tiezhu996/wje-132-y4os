package model

import "time"

// RectificationTask 整改任务：每个不合格检查项对应唯一一条整改任务（inspection_item_id 唯一）。
type RectificationTask struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	InspectionItemID uint64     `gorm:"not null;uniqueIndex:uk_rect_item" json:"inspection_item_id"`
	InspectionID     uint64     `gorm:"not null;index" json:"inspection_id"`
	ItemName         string     `gorm:"size:200;not null" json:"item_name"`
	AssigneeID       uint64     `gorm:"not null;default:0" json:"assignee_id"`
	Deadline         *time.Time `json:"deadline"`
	Status           string     `gorm:"size:30;not null;default:pending;index" json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (RectificationTask) TableName() string { return "rectification_tasks" }

// RectificationRecord 整改办理记录：只增不改，保留每次登记/提交/复查的原始记录。
type RectificationRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID       uint64    `gorm:"not null;index" json:"task_id"`
	Action       string    `gorm:"size:30;not null" json:"action"`
	Content      string    `gorm:"size:500;not null;default:''" json:"content"`
	OperatorID   uint64    `gorm:"not null;default:0" json:"operator_id"`
	OperatorName string    `gorm:"size:50;not null;default:''" json:"operator_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 指定表名。
func (RectificationRecord) TableName() string { return "rectification_records" }
