package service

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/model"
	"safetyplatform/internal/repository"
	"safetyplatform/internal/util"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newRectificationTestEnv 使用内存 SQLite 装配完整服务链，做端到端业务流转验证。
func newRectificationTestEnv(t *testing.T) (*RectificationTaskService, *SafetyInspectionService, *gorm.DB) {
	t.Helper()
	// 每个用例独立的内存库，避免互相污染。
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.SafetyIncident{}, &model.SafetyInspection{}, &model.InspectionItem{},
		&model.RectificationTask{}, &model.RectificationRecord{},
		&model.SafetyTraining{}, &model.WorkerCertification{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	userRepo := repository.NewUserRepository(db)
	inspectionRepo := repository.NewSafetyInspectionRepository(db)
	itemRepo := repository.NewInspectionItemRepository(db)
	rectRepo := repository.NewRectificationTaskRepository(db)
	rectSvc := NewRectificationTaskService(db, rectRepo, userRepo, inspectionRepo, logger)
	inspectionSvc := NewSafetyInspectionService(db, inspectionRepo, itemRepo, userRepo, rectSvc, logger)
	return rectSvc, inspectionSvc, db
}

func seedRectificationUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []model.User{
		{ID: 2, Phone: "13800000002", PasswordHash: "x", Name: "王安全", Role: constants.RoleSafetyManager},
		{ID: 3, Phone: "13800000003", PasswordHash: "x", Name: "李监理", Role: constants.RoleInspector},
		{ID: 4, Phone: "13800000004", PasswordHash: "x", Name: "赵工", Role: constants.RoleWorker},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
}

// TestRectificationLifecycle 覆盖：执行检查生成任务 -> 登记 -> 提交 -> 复查退回 -> 再提交 -> 复查通过。
func TestRectificationLifecycle(t *testing.T) {
	rectSvc, inspectionSvc, db := newRectificationTestEnv(t)
	seedRectificationUsers(t, db)

	// 1. 执行检查，勾出不合格项后自动生成整改任务。
	ins, err := inspectionSvc.Create("临时用电专项检查", constants.InspectionSpecial, "加工区", time.Now(), 3, []model.InspectionItem{
		{ItemName: "电缆绝缘完好"},
		{ItemName: "配电箱接地"},
	})
	if err != nil {
		t.Fatalf("create inspection: %v", err)
	}
	_, items, err := inspectionSvc.GetWithItems(ins.ID)
	if err != nil {
		t.Fatalf("get items: %v", err)
	}
	execItems := []model.InspectionItem{
		{ID: items[0].ID, Passed: false, Remark: "电缆破损"},
		{ID: items[1].ID, Passed: true},
	}
	if _, err := inspectionSvc.Execute(ins.ID, execItems); err != nil {
		t.Fatalf("execute inspection: %v", err)
	}
	views, total, err := rectSvc.List(1, 10, "", "", ins.ID)
	if err != nil || total != 1 {
		t.Fatalf("expected 1 rectification task, got total=%d err=%v", total, err)
	}
	task := views[0]
	if task.Status != constants.RectificationPending || task.ItemName != "电缆绝缘完好" || task.InspectionName != "临时用电专项检查" {
		t.Fatalf("unexpected task: %+v", task)
	}

	// 2. 同一检查项重复生成只保留一次办理结果。
	if err := db.Transaction(func(tx *gorm.DB) error {
		return rectSvc.EnsureTasksForFailedTx(tx, ins.ID, []model.InspectionItem{items[0]})
	}); err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	if _, total, _ = rectSvc.List(1, 10, "", "", ins.ID); total != 1 {
		t.Fatalf("duplicate task created, total=%d", total)
	}

	// 3. 未登记责任人前不能提交整改说明。
	if _, err := rectSvc.Submit(4, constants.RoleWorker, task.ID, "已整改"); err == nil {
		t.Fatal("submit before assign should fail")
	}

	// 4. 登记责任人和整改期限。
	deadline := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	assigned, err := rectSvc.Assign(3, task.ID, 4, deadline)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if assigned.AssigneeID != 4 || assigned.Deadline == nil || assigned.Status != constants.RectificationPending {
		t.Fatalf("assign not applied: %+v", assigned)
	}

	// 5. 非责任人提交被拒绝。
	if _, err := rectSvc.Submit(2, constants.RoleWorker, task.ID, "我来提交"); err == nil {
		t.Fatal("non-assignee submit should fail")
	} else {
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Code != constants.CodeRectificationNotAssignee {
			t.Fatalf("expected not-assignee error, got %v", err)
		}
	}

	// 6. 责任人提交整改说明，进入待复查。
	if _, err := rectSvc.Submit(4, constants.RoleWorker, task.ID, "已更换破损电缆"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// 待复查状态不允许重复提交。
	if _, err := rectSvc.Submit(4, constants.RoleWorker, task.ID, "重复提交"); err == nil {
		t.Fatal("double submit should fail")
	}

	// 7. 复查退回必须填写原因，退回后保留原办理记录。
	if _, err := rectSvc.Review(3, task.ID, false, ""); err == nil {
		t.Fatal("review return without reason should fail")
	}
	if _, err := rectSvc.Review(3, task.ID, false, "现场仍不合格"); err != nil {
		t.Fatalf("review return: %v", err)
	}
	detail, records, err := rectSvc.Get(task.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.Status != constants.RectificationReturned || len(records) != 3 {
		t.Fatalf("expected returned with 3 records, got status=%s records=%d", detail.Status, len(records))
	}
	if records[0].Action != constants.RectificationActionAssign || records[1].Action != constants.RectificationActionSubmit || records[2].Action != constants.RectificationActionReviewReturn {
		t.Fatalf("records not preserved in order: %+v", records)
	}

	// 8. 退回后责任人可重新提交，复查通过后闭环。
	if _, err := rectSvc.Submit(4, constants.RoleWorker, task.ID, "已重新整改并加装套管"); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if _, err := rectSvc.Review(3, task.ID, true, "复查合格"); err != nil {
		t.Fatalf("review pass: %v", err)
	}
	detail, records, _ = rectSvc.Get(task.ID)
	if detail.Status != constants.RectificationApproved || len(records) != 5 {
		t.Fatalf("expected approved with 5 records, got status=%s records=%d", detail.Status, len(records))
	}

	// 9. 复查通过后不再计入待整改/已逾期。
	if _, total, _ = rectSvc.List(1, 10, constants.RectificationFilterPending, "", ins.ID); total != 0 {
		t.Fatalf("approved task should not be pending, total=%d", total)
	}
	pending, overdue, err := rectSvc.PendingStats()
	if err != nil || pending != 0 || overdue != 0 {
		t.Fatalf("pending stats mismatch: pending=%d overdue=%d err=%v", pending, overdue, err)
	}
}

// TestRectificationOverdueFilter 逾期任务进入已逾期筛选并标记 overdue。
func TestRectificationOverdueFilter(t *testing.T) {
	rectSvc, _, db := newRectificationTestEnv(t)
	seedRectificationUsers(t, db)

	ins := model.SafetyInspection{Name: "消防检查", InspectionType: constants.InspectionRoutine, InspectionDate: time.Now(), InspectorID: 3, Status: constants.InspectionFailed}
	if err := db.Create(&ins).Error; err != nil {
		t.Fatalf("create inspection: %v", err)
	}
	item := model.InspectionItem{InspectionID: ins.ID, ItemName: "灭火器在位", Passed: false}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	tasks := []model.RectificationTask{
		{InspectionItemID: item.ID, InspectionID: ins.ID, ItemName: item.ItemName, AssigneeID: 4, Deadline: &past, Status: constants.RectificationPending},
		{InspectionItemID: item.ID + 100, InspectionID: ins.ID, ItemName: "消防通道", AssigneeID: 4, Deadline: &future, Status: constants.RectificationSubmitted},
	}
	for i := range tasks {
		if err := db.Create(&tasks[i]).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	overdueList, overdueTotal, err := rectSvc.List(1, 10, constants.RectificationFilterOverdue, "", 0)
	if err != nil || overdueTotal != 1 {
		t.Fatalf("overdue filter mismatch: total=%d err=%v", overdueTotal, err)
	}
	if !overdueList[0].Overdue || overdueList[0].ItemName != "灭火器在位" || overdueList[0].AssigneeName != "赵工" {
		t.Fatalf("overdue view mismatch: %+v", overdueList[0])
	}
	pendingList, pendingTotal, _ := rectSvc.List(1, 10, constants.RectificationFilterPending, "", 0)
	if pendingTotal != 2 {
		t.Fatalf("pending filter mismatch: total=%d", pendingTotal)
	}
	// 逾期任务排在最前。
	if !pendingList[0].Overdue {
		t.Fatalf("overdue task should be first: %+v", pendingList[0])
	}
	pending, overdue, _ := rectSvc.PendingStats()
	if pending != 2 || overdue != 1 {
		t.Fatalf("stats mismatch: pending=%d overdue=%d", pending, overdue)
	}
}
