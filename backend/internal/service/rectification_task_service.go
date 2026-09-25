package service

import (
	"log/slog"
	"time"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/dto"
	"safetyplatform/internal/model"
	"safetyplatform/internal/repository"
	"safetyplatform/internal/util"

	"gorm.io/gorm"
)

// RectificationTaskService 整改任务业务逻辑：不合格检查项 -> 登记 -> 提交 -> 复查 的闭环跟踪。
type RectificationTaskService struct {
	db             *gorm.DB
	repo           *repository.RectificationTaskRepository
	userRepo       *repository.UserRepository
	inspectionRepo *repository.SafetyInspectionRepository
	logger         *slog.Logger
}

// NewRectificationTaskService 构造整改任务服务。
func NewRectificationTaskService(db *gorm.DB, repo *repository.RectificationTaskRepository,
	userRepo *repository.UserRepository, inspectionRepo *repository.SafetyInspectionRepository, logger *slog.Logger) *RectificationTaskService {
	return &RectificationTaskService{db: db, repo: repo, userRepo: userRepo, inspectionRepo: inspectionRepo, logger: logger}
}

// EnsureTasksForFailedTx 在检查执行事务内为不合格项创建整改任务；同一检查项只保留一条任务。
func (s *RectificationTaskService) EnsureTasksForFailedTx(tx *gorm.DB, inspectionID uint64, items []model.InspectionItem) error {
	for _, it := range items {
		if it.Passed {
			continue
		}
		task := &model.RectificationTask{
			InspectionItemID: it.ID,
			InspectionID:     inspectionID,
			ItemName:         it.ItemName,
			Status:           constants.RectificationPending,
		}
		created, err := s.repo.CreateIfAbsentTx(tx, task)
		if err != nil {
			return util.Wrap(err, "RectificationTask[item_id=%d] create failed", it.ID)
		}
		if created {
			s.logger.Info(constants.LogRectificationTaskCreated, "task_id", task.ID, "inspection_item_id", it.ID)
		}
	}
	return nil
}

// Assign 登记责任人和整改期限；待整改/已退回状态可登记，登记后回到待整改。
func (s *RectificationTaskService) Assign(operatorID uint64, id, assigneeID uint64, deadline time.Time) (*model.RectificationTask, error) {
	assignee, err := s.userRepo.FindByID(assigneeID)
	if err != nil {
		return nil, util.Wrap(err, "RectificationTask[id=%d] assign: assignee user[id=%d] not found", id, assigneeID)
	}
	task := &model.RectificationTask{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		cur, err := s.repo.FindByIDForUpdate(tx, id)
		if err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] assign find failed", id)
		}
		if cur.Status != constants.RectificationPending && cur.Status != constants.RectificationReturned {
			return util.NewAppError(constants.CodeRectificationConflict, "RectificationTask[id="+u64(id)+"] assign conflict: status="+cur.Status)
		}
		cur.AssigneeID = assigneeID
		cur.Deadline = &deadline
		cur.Status = constants.RectificationPending
		if err := s.repo.UpdateTx(tx, cur); err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] assign save failed", id)
		}
		rec := &model.RectificationRecord{
			TaskID:       cur.ID,
			Action:       constants.RectificationActionAssign,
			Content:      "登记责任人：" + assignee.Name + "；整改期限：" + util.FormatDateTime(deadline),
			OperatorID:   operatorID,
			OperatorName: s.operatorName(operatorID),
		}
		if err := s.repo.AppendRecordTx(tx, rec); err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] assign record failed", id)
		}
		task = cur
		return nil
	})
	if err != nil {
		s.logger.Error(constants.LogRectificationStatusFailed, "task_id", id, "error", err.Error())
		return nil, err
	}
	s.logger.Info(constants.LogRectificationAssignSuccess, "task_id", task.ID, "assignee_id", assigneeID)
	return task, nil
}

// Submit 责任人提交整改说明，提交后进入待复查；只有责任人本人（或管理员）可提交。
func (s *RectificationTaskService) Submit(operatorID uint64, operatorRole string, id uint64, note string) (*model.RectificationTask, error) {
	task := &model.RectificationTask{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		cur, err := s.repo.FindByIDForUpdate(tx, id)
		if err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] submit find failed", id)
		}
		if cur.Status != constants.RectificationPending && cur.Status != constants.RectificationReturned {
			return util.NewAppError(constants.CodeRectificationConflict, "RectificationTask[id="+u64(id)+"] submit conflict: status="+cur.Status)
		}
		if cur.AssigneeID == 0 {
			return util.NewAppError(constants.CodeRectificationConflict, "RectificationTask[id="+u64(id)+"] submit failed: "+constants.MsgRectificationAssignFirst)
		}
		if cur.AssigneeID != operatorID && operatorRole != constants.RoleAdmin {
			return util.NewAppError(constants.CodeRectificationNotAssignee, "RectificationTask[id="+u64(id)+"] submit failed: operator="+u64(operatorID)+" not assignee="+u64(cur.AssigneeID))
		}
		cur.Status = constants.RectificationSubmitted
		if err := s.repo.UpdateTx(tx, cur); err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] submit save failed", id)
		}
		rec := &model.RectificationRecord{
			TaskID:       cur.ID,
			Action:       constants.RectificationActionSubmit,
			Content:      note,
			OperatorID:   operatorID,
			OperatorName: s.operatorName(operatorID),
		}
		if err := s.repo.AppendRecordTx(tx, rec); err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] submit record failed", id)
		}
		task = cur
		return nil
	})
	if err != nil {
		s.logger.Error(constants.LogRectificationStatusFailed, "task_id", id, "error", err.Error())
		return nil, err
	}
	s.logger.Info(constants.LogRectificationSubmitSuccess, "task_id", task.ID, "operator_id", operatorID)
	return task, nil
}

// Review 复查：通过则闭环（不再计入待整改）；不通过则退回并保留原办理记录。
func (s *RectificationTaskService) Review(operatorID uint64, id uint64, passed bool, note string) (*model.RectificationTask, error) {
	if !passed && note == "" {
		return nil, util.NewAppError(constants.CodeValidationFailed, "RectificationTask[id="+u64(id)+"] review failed: "+constants.MsgRectificationReturnReason)
	}
	task := &model.RectificationTask{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		cur, err := s.repo.FindByIDForUpdate(tx, id)
		if err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] review find failed", id)
		}
		if cur.Status != constants.RectificationSubmitted {
			return util.NewAppError(constants.CodeRectificationConflict, "RectificationTask[id="+u64(id)+"] review conflict: status="+cur.Status)
		}
		rec := &model.RectificationRecord{
			TaskID:       cur.ID,
			Content:      note,
			OperatorID:   operatorID,
			OperatorName: s.operatorName(operatorID),
		}
		if passed {
			cur.Status = constants.RectificationApproved
			rec.Action = constants.RectificationActionReviewPass
		} else {
			cur.Status = constants.RectificationReturned
			rec.Action = constants.RectificationActionReviewReturn
		}
		if err := s.repo.UpdateTx(tx, cur); err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] review save failed", id)
		}
		if err := s.repo.AppendRecordTx(tx, rec); err != nil {
			return util.Wrap(err, "RectificationTask[id=%d] review record failed", id)
		}
		task = cur
		return nil
	})
	if err != nil {
		s.logger.Error(constants.LogRectificationStatusFailed, "task_id", id, "error", err.Error())
		return nil, err
	}
	if passed {
		s.logger.Info(constants.LogRectificationReviewPass, "task_id", task.ID, "operator_id", operatorID)
	} else {
		s.logger.Info(constants.LogRectificationReviewReturn, "task_id", task.ID, "operator_id", operatorID)
	}
	return task, nil
}

// List 分页查询整改任务（待整改/已逾期筛选），附带检查名称、责任人姓名与逾期标记。
func (s *RectificationTaskService) List(page, pageSize int, filter, status string, inspectionID uint64) ([]dto.RectificationTaskView, int64, error) {
	now := time.Now()
	tasks, total, err := s.repo.List(page, pageSize, filter, status, inspectionID, now)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.enrich(tasks, now)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// Get 整改任务详情 + 全部办理记录。
func (s *RectificationTaskService) Get(id uint64) (*dto.RectificationTaskView, []model.RectificationRecord, error) {
	task, err := s.repo.FindByID(id)
	if err != nil {
		return nil, nil, util.Wrap(err, "RectificationTask[id=%d] get failed", id)
	}
	views, err := s.enrich([]model.RectificationTask{*task}, time.Now())
	if err != nil {
		return nil, nil, err
	}
	records, err := s.repo.ListRecords(id)
	if err != nil {
		return nil, nil, err
	}
	return &views[0], records, nil
}

// PendingStats 待整改与已逾期任务数（复查通过不计入待整改）。
func (s *RectificationTaskService) PendingStats() (int64, int64, error) {
	return s.repo.PendingStats(time.Now())
}

// enrich 批量补充检查名称、责任人姓名并计算逾期标记。
func (s *RectificationTaskService) enrich(tasks []model.RectificationTask, now time.Time) ([]dto.RectificationTaskView, error) {
	userIDs, inspectionIDs := make([]uint64, 0, len(tasks)), make([]uint64, 0, len(tasks))
	for _, t := range tasks {
		if t.AssigneeID > 0 {
			userIDs = append(userIDs, t.AssigneeID)
		}
		inspectionIDs = append(inspectionIDs, t.InspectionID)
	}
	users, err := s.userRepo.ListByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	inspections, err := s.inspectionRepo.ListByIDs(inspectionIDs)
	if err != nil {
		return nil, err
	}
	userNames := make(map[uint64]string, len(users))
	for _, u := range users {
		userNames[u.ID] = u.Name
	}
	inspectionNames := make(map[uint64]string, len(inspections))
	for _, ins := range inspections {
		inspectionNames[ins.ID] = ins.Name
	}
	views := make([]dto.RectificationTaskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, dto.RectificationTaskView{
			RectificationTask: t,
			InspectionName:    inspectionNames[t.InspectionID],
			AssigneeName:      userNames[t.AssigneeID],
			Overdue:           isOverdue(t, now),
		})
	}
	return views, nil
}

// operatorName 查询操作人姓名，查不到时回退为空串。
func (s *RectificationTaskService) operatorName(operatorID uint64) string {
	u, err := s.userRepo.FindByID(operatorID)
	if err != nil {
		return ""
	}
	return u.Name
}

// isOverdue 未复查通过且已超过整改期限即为逾期。
func isOverdue(t model.RectificationTask, now time.Time) bool {
	if t.Status == constants.RectificationApproved || t.Deadline == nil {
		return false
	}
	return t.Deadline.Before(now)
}
