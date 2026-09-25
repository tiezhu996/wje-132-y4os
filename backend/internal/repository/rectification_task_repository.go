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

// RectTaskListItem 整改任务列表项（携带检查、责任人、复查人名称）。
type RectTaskListItem struct {
	model.RectificationTask
	InspectionName string `json:"inspection_name"`
	Area           string `json:"area"`
	AssigneeName   string `json:"assignee_name"`
	ReviewerName   string `json:"reviewer_name"`
	Overdue        bool   `json:"overdue"`
}

// RectTaskQuery 整改任务列表查询条件。
type RectTaskQuery struct {
	Status     string // 精确状态
	PendingAll bool   // 待整改聚合（status != approved）
	Overdue    bool   // 已逾期（status != approved AND deadline < now）
	AssigneeID uint64 // 责任人过滤（0 表示不限）
}

// RectificationTaskRepository 整改任务仓储。
type RectificationTaskRepository struct {
	db *gorm.DB
}

// NewRectificationTaskRepository 构造整改任务仓储。
func NewRectificationTaskRepository(db *gorm.DB) *RectificationTaskRepository {
	return &RectificationTaskRepository{db: db}
}

// CreateTx 在事务内创建整改任务。
func (r *RectificationTaskRepository) CreateTx(tx *gorm.DB, t *model.RectificationTask) error {
	if err := tx.Create(t).Error; err != nil {
		return fmt.Errorf("create rectification task: %w", err)
	}
	return nil
}

// CreateHistoryTx 在事务内追加一条流转记录。
func (r *RectificationTaskRepository) CreateHistoryTx(tx *gorm.DB, h *model.RectificationHistory) error {
	if err := tx.Create(h).Error; err != nil {
		return fmt.Errorf("create rectification history: %w", err)
	}
	return nil
}

// FindByID 按 ID 查询整改任务。
func (r *RectificationTaskRepository) FindByID(id uint64) (*model.RectificationTask, error) {
	return r.findByID(r.db, id, false)
}

// FindByIDForUpdateTx 在事务内锁定整改任务行。
func (r *RectificationTaskRepository) FindByIDForUpdateTx(tx *gorm.DB, id uint64) (*model.RectificationTask, error) {
	return r.findByID(tx, id, true)
}

func (r *RectificationTaskRepository) findByID(db *gorm.DB, id uint64, forUpdate bool) (*model.RectificationTask, error) {
	var t model.RectificationTask
	q := db
	if forUpdate {
		q = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find rectification task by id: %w", err)
	}
	return &t, nil
}

// FindByItemIDTx 在事务内按检查项查询整改任务（用于防重）。
func (r *RectificationTaskRepository) FindByItemIDTx(tx *gorm.DB, itemID uint64) (*model.RectificationTask, error) {
	var t model.RectificationTask
	if err := tx.Where("item_id = ?", itemID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find rectification task by item: %w", err)
	}
	return &t, nil
}

// UpdateTx 在事务内更新整改任务。
func (r *RectificationTaskRepository) UpdateTx(tx *gorm.DB, t *model.RectificationTask) error {
	if err := tx.Save(t).Error; err != nil {
		return fmt.Errorf("update rectification task: %w", err)
	}
	return nil
}

// ListHistories 查询任务的全部流转记录（按时间正序）。
func (r *RectificationTaskRepository) ListHistories(taskID uint64) ([]model.RectificationHistory, error) {
	var list []model.RectificationHistory
	if err := r.db.Where("task_id = ?", taskID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list rectification histories: %w", err)
	}
	return list, nil
}

// applyQuery 应用列表筛选条件。
func applyQuery(q *gorm.DB, query RectTaskQuery, now time.Time) *gorm.DB {
	switch {
	case query.Overdue:
		q = q.Where("t.status <> ? AND t.deadline < ?", constants.RectTaskApproved, now)
	case query.PendingAll:
		q = q.Where("t.status <> ?", constants.RectTaskApproved)
	case query.Status != "":
		q = q.Where("t.status = ?", query.Status)
	}
	if query.AssigneeID > 0 {
		q = q.Where("t.assignee_id = ?", query.AssigneeID)
	}
	return q
}

// List 分页查询整改任务，逾期任务排在最前，其次按更新时间倒序。
func (r *RectificationTaskRepository) List(page, pageSize int, query RectTaskQuery) ([]RectTaskListItem, int64, error) {
	now := time.Now()
	base := r.db.Table("rectification_tasks AS t")
	base = applyQuery(base, query, now)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count rectification tasks: %w", err)
	}

	var list []RectTaskListItem
	selectSQL := "t.*, si.name AS inspection_name, si.area AS area, " +
		"au.name AS assignee_name, ru.name AS reviewer_name, " +
		"(t.status <> ? AND t.deadline < ?) AS overdue"
	if err := r.db.Table("rectification_tasks AS t").
		Select(selectSQL, constants.RectTaskApproved, now).
		Joins("LEFT JOIN safety_inspections AS si ON si.id = t.inspection_id").
		Joins("LEFT JOIN users AS au ON au.id = t.assignee_id").
		Joins("LEFT JOIN users AS ru ON ru.id = t.reviewer_id").
		Scopes(func(db *gorm.DB) *gorm.DB { return applyQuery(db, query, now) }).
		Order("overdue DESC, t.updated_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list rectification tasks: %w", err)
	}
	return list, total, nil
}

// ListByInspection 查询某次检查的整改任务（按检查项 ID 索引）。
func (r *RectificationTaskRepository) ListByInspection(inspectionID uint64) ([]model.RectificationTask, error) {
	var list []model.RectificationTask
	if err := r.db.Where("inspection_id = ?", inspectionID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list rectification tasks by inspection: %w", err)
	}
	return list, nil
}

// CountPending 待整改任务数（复查通过后即不再计入）。
func (r *RectificationTaskRepository) CountPending() (int64, error) {
	var n int64
	if err := r.db.Model(&model.RectificationTask{}).
		Where("status <> ?", constants.RectTaskApproved).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count pending rectification tasks: %w", err)
	}
	return n, nil
}

// CountOverdue 已逾期任务数（未复查通过且超过期限）。
func (r *RectificationTaskRepository) CountOverdue() (int64, error) {
	var n int64
	if err := r.db.Model(&model.RectificationTask{}).
		Where("status <> ? AND deadline < ?", constants.RectTaskApproved, time.Now()).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count overdue rectification tasks: %w", err)
	}
	return n, nil
}
