package service

import (
	"log/slog"
	"time"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/model"
	"safetyplatform/internal/repository"
	"safetyplatform/internal/util"

	"gorm.io/gorm"
)

// RectificationTaskService 整改任务业务逻辑。
type RectificationTaskService struct {
	db       *gorm.DB
	repo     *repository.RectificationTaskRepository
	insRepo  *repository.SafetyInspectionRepository
	itemRepo *repository.InspectionItemRepository
	userRepo *repository.UserRepository
	logger   *slog.Logger
}

// NewRectificationTaskService 构造整改任务服务。
func NewRectificationTaskService(db *gorm.DB, repo *repository.RectificationTaskRepository,
	insRepo *repository.SafetyInspectionRepository, itemRepo *repository.InspectionItemRepository,
	userRepo *repository.UserRepository, logger *slog.Logger) *RectificationTaskService {
	return &RectificationTaskService{db: db, repo: repo, insRepo: insRepo, itemRepo: itemRepo, userRepo: userRepo, logger: logger}
}

// Register 为不合格检查项登记整改任务（责任人 + 整改期限）。
// 同一检查项只允许一条任务，重复登记返回冲突。
func (s *RectificationTaskService) Register(inspectionID, itemID, assigneeID uint64, deadline time.Time) (*model.RectificationTask, error) {
	if _, err := s.userRepo.FindByID(assigneeID); err != nil {
		return nil, util.Wrap(err, "RectificationTask[assignee_id=%d] register: assignee not found", assigneeID)
	}
	task := &model.RectificationTask{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		ins, err := s.insRepo.FindByIDForUpdate(tx, inspectionID)
		if err != nil {
			return util.Wrap(err, "RectificationTask[inspection_id=%d] register: inspection not found", inspectionID)
		}
		var item model.InspectionItem
		if err := tx.First(&item, itemID).Error; err != nil {
			return util.NewAppError(constants.CodeNotFound, "RectificationTask[item_id="+u64(itemID)+"] register: item not found")
		}
		if item.InspectionID != inspectionID {
			return util.NewAppError(constants.CodeBadRequest, "RectificationTask[item_id="+u64(itemID)+"] register: item does not belong to inspection "+u64(inspectionID))
		}
		if item.Passed {
			return util.NewAppError(constants.CodeValidationFailed, "RectificationTask[item_id="+u64(itemID)+"] register: item passed, no rectification needed")
		}
		if exist, err := s.repo.FindByItemIDTx(tx, itemID); err == nil && exist != nil {
			return util.NewAppError(constants.CodeRectTaskExists, "RectificationTask[item_id="+u64(itemID)+"] register: task already exists, id="+u64(exist.ID))
		} else if err != nil && err != repository.ErrNotFound {
			return util.Wrap(err, "RectificationTask[item_id=%d] register: check existing failed", itemID)
		}
		now := time.Now()
		task = &model.RectificationTask{
			InspectionID: inspectionID,
			ItemID:       itemID,
			ItemName:     item.ItemName,
			AssigneeID:   assigneeID,
			Deadline:     deadline,
			Status:       constants.RectTaskPending,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.repo.CreateTx(tx, task); err != nil {
			return err
		}
		if err := s.repo.CreateHistoryTx(tx, &model.RectificationHistory{
			TaskID: task.ID, Action: constants.RectActionRegister,
			Note:       ins.Name + " 登记整改任务，期限 " + deadline.Format("2006-01-02"),
			OperatorID: ins.InspectorID, CreatedAt: now,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogRectTaskRegisterSuccess, "task_id", task.ID, "item_id", itemID, "assignee_id", assigneeID)
	return task, nil
}

// Submit 责任人提交整改说明，提交后等待复查。
func (s *RectificationTaskService) Submit(id, operatorID uint64, note, photo string) (*model.RectificationTask, error) {
	task := &model.RectificationTask{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		cur, err := s.repo.FindByIDForUpdateTx(tx, id)
		if err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] submit find failed", id)
		}
		if cur.AssigneeID != operatorID {
			return util.NewAppError(constants.CodeForbidden, "RectificationTask[id="+u64(id)+"] submit forbidden: only assignee can submit")
		}
		if cur.Status != constants.RectTaskPending && cur.Status != constants.RectTaskRejected {
			return util.NewAppError(constants.CodeRectTaskConflict, "RectificationTask[id="+u64(id)+"] submit conflict: status="+cur.Status)
		}
		now := time.Now()
		cur.Status = constants.RectTaskSubmitted
		cur.RectificationNote = note
		cur.RectificationPhoto = photo
		cur.SubmittedAt = &now
		cur.UpdatedAt = now
		if err := s.repo.UpdateTx(tx, cur); err != nil {
			return err
		}
		if err := s.repo.CreateHistoryTx(tx, &model.RectificationHistory{
			TaskID: cur.ID, Action: constants.RectActionSubmit,
			Note: note, PhotoURL: photo, OperatorID: operatorID, CreatedAt: now,
		}); err != nil {
			return err
		}
		task = cur
		return nil
	})
	if err != nil {
		s.logger.Warn(constants.LogRectTaskStatusChangeFailed, "action", "submit", "error", err.Error())
		return nil, err
	}
	s.logger.Info(constants.LogRectTaskSubmitSuccess, "task_id", id)
	return task, nil
}

// Review 复查：通过则关闭任务（不再计入待整改）；不通过则退回并保留原提交记录。
func (s *RectificationTaskService) Review(id, reviewerID uint64, approved bool, reviewNote string) (*model.RectificationTask, error) {
	if !approved && reviewNote == "" {
		return nil, util.NewAppError(constants.CodeValidationFailed, "RectificationTask[id="+u64(id)+"] reject: review note required")
	}
	task := &model.RectificationTask{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		cur, err := s.repo.FindByIDForUpdateTx(tx, id)
		if err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] review find failed", id)
		}
		if cur.Status != constants.RectTaskSubmitted {
			return util.NewAppError(constants.CodeRectTaskConflict, "RectificationTask[id="+u64(id)+"] review conflict: status="+cur.Status)
		}
		now := time.Now()
		action := constants.RectActionApprove
		if approved {
			cur.Status = constants.RectTaskApproved
		} else {
			cur.Status = constants.RectTaskRejected
			action = constants.RectActionReject
		}
		cur.ReviewerID = reviewerID
		cur.ReviewedAt = &now
		cur.ReviewNote = reviewNote
		cur.UpdatedAt = now
		if err := s.repo.UpdateTx(tx, cur); err != nil {
			return err
		}
		if err := s.repo.CreateHistoryTx(tx, &model.RectificationHistory{
			TaskID: cur.ID, Action: action,
			Note: reviewNote, OperatorID: reviewerID, CreatedAt: now,
		}); err != nil {
			return err
		}
		task = cur
		return nil
	})
	if err != nil {
		s.logger.Warn(constants.LogRectTaskStatusChangeFailed, "action", "review", "error", err.Error())
		return nil, err
	}
	if approved {
		s.logger.Info(constants.LogRectTaskApproveSuccess, "task_id", id)
	} else {
		s.logger.Info(constants.LogRectTaskRejectSuccess, "task_id", id)
	}
	return task, nil
}

// Get 任务详情 + 流转历史（含被退回的原始提交记录）。
func (s *RectificationTaskService) Get(id uint64) (*model.RectificationTask, []model.RectificationHistory, error) {
	task, err := s.repo.FindByID(id)
	if err != nil {
		return nil, nil, util.Wrap(err, "RectificationTask[id=%d] get failed", id)
	}
	histories, err := s.repo.ListHistories(id)
	if err != nil {
		return nil, nil, err
	}
	return task, histories, nil
}

// List 分页查询整改任务。
func (s *RectificationTaskService) List(page, pageSize int, status string, viewerID uint64, mine bool) ([]repository.RectTaskListItem, int64, error) {
	q := repository.RectTaskQuery{Status: status}
	switch status {
	case "pending_rectification":
		q.PendingAll = true
		q.Status = ""
	case "overdue":
		q.Overdue = true
		q.Status = ""
	}
	if mine {
		q.AssigneeID = viewerID
	}
	return s.repo.List(page, pageSize, q)
}

// ListByInspection 查询某次检查的整改任务。
func (s *RectificationTaskService) ListByInspection(inspectionID uint64) ([]model.RectificationTask, error) {
	return s.repo.ListByInspection(inspectionID)
}

// CountPending 待整改任务数。
func (s *RectificationTaskService) CountPending() (int64, error) {
	return s.repo.CountPending()
}

// CountOverdue 已逾期任务数。
func (s *RectificationTaskService) CountOverdue() (int64, error) {
	return s.repo.CountOverdue()
}
