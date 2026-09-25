package repository

import (
	"errors"
	"fmt"
	"time"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RectificationTaskRepository 整改任务仓储。
type RectificationTaskRepository struct {
	db *gorm.DB
}

// NewRectificationTaskRepository 构造整改任务仓储。
func NewRectificationTaskRepository(db *gorm.DB) *RectificationTaskRepository {
	return &RectificationTaskRepository{db: db}
}

// CreateIfAbsentTx 在事务内为不合格检查项创建整改任务；同一检查项已存在任务时跳过（重复提交只保留一次办理结果）。
func (r *RectificationTaskRepository) CreateIfAbsentTx(tx *gorm.DB, task *model.RectificationTask) (bool, error) {
	var count int64
	if err := tx.Model(&model.RectificationTask{}).Where("inspection_item_id = ?", task.InspectionItemID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("count rectification task by item: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	if err := tx.Create(task).Error; err != nil {
		return false, fmt.Errorf("create rectification task: %w", err)
	}
	return true, nil
}

// FindByID 按 ID 查询整改任务。
func (r *RectificationTaskRepository) FindByID(id uint64) (*model.RectificationTask, error) {
	var t model.RectificationTask
	if err := r.db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find rectification task by id: %w", err)
	}
	return &t, nil
}

// FindByIDForUpdate 在事务内锁定整改任务行。
func (r *RectificationTaskRepository) FindByIDForUpdate(tx *gorm.DB, id uint64) (*model.RectificationTask, error) {
	var t model.RectificationTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find rectification task by id for update: %w", err)
	}
	return &t, nil
}

// FindByItemID 按检查项查询整改任务。
func (r *RectificationTaskRepository) FindByItemID(itemID uint64) (*model.RectificationTask, error) {
	var t model.RectificationTask
	if err := r.db.Where("inspection_item_id = ?", itemID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find rectification task by item: %w", err)
	}
	return &t, nil
}

// List 分页查询整改任务，支持待整改/已逾期筛选；逾期任务排在最前。
func (r *RectificationTaskRepository) List(page, pageSize int, filter, status string, inspectionID uint64, now time.Time) ([]model.RectificationTask, int64, error) {
	var list []model.RectificationTask
	var total int64
	q := r.db.Model(&model.RectificationTask{})
	switch filter {
	case constants.RectificationFilterPending:
		q = q.Where("status <> ?", constants.RectificationApproved)
	case constants.RectificationFilterOverdue:
		q = q.Where("status <> ? AND deadline IS NOT NULL AND deadline < ?", constants.RectificationApproved, now)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if inspectionID > 0 {
		q = q.Where("inspection_id = ?", inspectionID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count rectification tasks: %w", err)
	}
	overdueExpr := "(status <> '" + constants.RectificationApproved + "' AND deadline IS NOT NULL AND deadline < ?)"
	if err := q.
		Order(gorm.Expr(overdueExpr+" DESC", now)).
		Order("CASE WHEN status = '" + constants.RectificationApproved + "' THEN 1 ELSE 0 END ASC").
		Order("deadline IS NULL ASC").
		Order("deadline ASC").
		Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list rectification tasks: %w", err)
	}
	return list, total, nil
}

// UpdateTx 在事务内更新整改任务。
func (r *RectificationTaskRepository) UpdateTx(tx *gorm.DB, t *model.RectificationTask) error {
	if err := tx.Save(t).Error; err != nil {
		return fmt.Errorf("update rectification task: %w", err)
	}
	return nil
}

// AppendRecordTx 在事务内追加整改办理记录（只增不改，保留原记录）。
func (r *RectificationTaskRepository) AppendRecordTx(tx *gorm.DB, rec *model.RectificationRecord) error {
	if err := tx.Create(rec).Error; err != nil {
		return fmt.Errorf("append rectification record: %w", err)
	}
	return nil
}

// ListRecords 查询某整改任务的全部办理记录。
func (r *RectificationTaskRepository) ListRecords(taskID uint64) ([]model.RectificationRecord, error) {
	var list []model.RectificationRecord
	if err := r.db.Where("task_id = ?", taskID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list rectification records: %w", err)
	}
	return list, nil
}

// PendingStats 统计待整改与已逾期任务数（复查通过不计入待整改）。
func (r *RectificationTaskRepository) PendingStats(now time.Time) (pending int64, overdue int64, err error) {
	if err = r.db.Model(&model.RectificationTask{}).Where("status <> ?", constants.RectificationApproved).Count(&pending).Error; err != nil {
		return 0, 0, fmt.Errorf("count pending rectification tasks: %w", err)
	}
	if err = r.db.Model(&model.RectificationTask{}).
		Where("status <> ? AND deadline IS NOT NULL AND deadline < ?", constants.RectificationApproved, now).
		Count(&overdue).Error; err != nil {
		return 0, 0, fmt.Errorf("count overdue rectification tasks: %w", err)
	}
	return pending, overdue, nil
}
