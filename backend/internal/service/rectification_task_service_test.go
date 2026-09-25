package service

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/model"
	"safetyplatform/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var rectTestDBCounter int64

func newRectTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:recttest%d?mode=memory&cache=shared", atomic.AddInt64(&rectTestDBCounter, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.SafetyInspection{}, &model.InspectionItem{},
		&model.RectificationTask{}, &model.RectificationHistory{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedRectFixture(t *testing.T, db *gorm.DB, assigneeID, inspectorID uint64, passed bool, deadline time.Time) (uint64, uint64) {
	t.Helper()
	ins := &model.SafetyInspection{
		Name: "测试检查", InspectionType: constants.InspectionRoutine, Area: "A区",
		InspectionDate: time.Now(), InspectorID: inspectorID, Status: constants.InspectionFailed,
	}
	if err := db.Create(ins).Error; err != nil {
		t.Fatalf("create inspection: %v", err)
	}
	item := &model.InspectionItem{InspectionID: ins.ID, ItemName: "临边防护", Passed: passed, Remark: "栏杆缺失"}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	return ins.ID, item.ID
}

func newRectSvc(db *gorm.DB) *RectificationTaskService {
	logger := slog.Default()
	return NewRectificationTaskService(
		db,
		repository.NewRectificationTaskRepository(db),
		repository.NewSafetyInspectionRepository(db),
		repository.NewInspectionItemRepository(db),
		repository.NewUserRepository(db),
		logger,
	)
}

// TestRectTaskFullLifecycle 登记 -> 提交 -> 退回（保留原记录）-> 重新提交 -> 复查通过。
func TestRectTaskFullLifecycle(t *testing.T) {
	db := newRectTestDB(t)
	assignee := &model.User{Phone: "13900000001", Name: "赵工", Role: constants.RoleWorker}
	inspector := &model.User{Phone: "13900000002", Name: "李监理", Role: constants.RoleInspector}
	if err := db.Create(assignee).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(inspector).Error; err != nil {
		t.Fatal(err)
	}
	insID, itemID := seedRectFixture(t, db, assignee.ID, inspector.ID, false, time.Now().Add(24*time.Hour))
	svc := newRectSvc(db)

	// 登记
	task, err := svc.Register(insID, itemID, assignee.ID, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if task.Status != constants.RectTaskPending {
		t.Fatalf("want pending, got %s", task.Status)
	}

	// 合格项不能登记
	passedInsID, passedItemID := seedRectFixture(t, db, assignee.ID, inspector.ID, true, time.Now().Add(24*time.Hour))
	if _, err := svc.Register(passedInsID, passedItemID, assignee.ID, time.Now().Add(24*time.Hour)); err == nil {
		t.Fatal("register task for passed item should fail")
	}

	// 同一项重复登记只允许一次
	if _, err := svc.Register(insID, itemID, assignee.ID, time.Now().Add(24*time.Hour)); err == nil {
		t.Fatal("duplicate register should fail")
	}

	// 责任人提交整改说明
	task, err = svc.Submit(task.ID, assignee.ID, "已加装防护栏杆", "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Status != constants.RectTaskSubmitted || task.SubmittedAt == nil {
		t.Fatalf("submit status wrong: %+v", task)
	}

	// 待复查状态不能重复提交
	if _, err := svc.Submit(task.ID, assignee.ID, "再次提交", ""); err == nil {
		t.Fatal("duplicate submit in submitted status should fail")
	}

	// 退回必须填写意见
	if _, err := svc.Review(task.ID, inspector.ID, false, ""); err == nil {
		t.Fatal("reject without note should fail")
	}
	// 复查不通过，退回
	task, err = svc.Review(task.ID, inspector.ID, false, "栏杆间距过大")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if task.Status != constants.RectTaskRejected || task.ReviewNote != "栏杆间距过大" {
		t.Fatalf("reject status wrong: %+v", task)
	}

	// 退回后重新提交，同一条任务
	task, err = svc.Submit(task.ID, assignee.ID, "已调整栏杆间距至规范要求", "")
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if task.Status != constants.RectTaskSubmitted {
		t.Fatalf("want submitted, got %s", task.Status)
	}

	// 复查通过，不再计入待整改
	task, err = svc.Review(task.ID, inspector.ID, true, "符合要求")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if task.Status != constants.RectTaskApproved {
		t.Fatalf("want approved, got %s", task.Status)
	}

	// 历史记录完整保留：登记、提交1、退回、提交2、通过 共 5 条
	_, histories, err := svc.Get(task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	wantActions := []string{
		constants.RectActionRegister,
		constants.RectActionSubmit,
		constants.RectActionReject,
		constants.RectActionSubmit,
		constants.RectActionApprove,
	}
	if len(histories) != len(wantActions) {
		t.Fatalf("want %d histories, got %d", len(wantActions), len(histories))
	}
	for i, h := range histories {
		if h.Action != wantActions[i] {
			t.Fatalf("history[%d] action = %s, want %s", i, h.Action, wantActions[i])
		}
	}
	// 第一次被退回的提交说明仍在历史中
	if histories[1].Note != "已加装防护栏杆" {
		t.Fatalf("original submission note not preserved: %s", histories[1].Note)
	}

	// 通过后不计入待整改/逾期
	if n, _ := svc.CountPending(); n != 0 {
		t.Fatalf("approved task must not count as pending, got %d", n)
	}
	if n, _ := svc.CountOverdue(); n != 0 {
		t.Fatalf("approved task must not count as overdue, got %d", n)
	}
}

// TestRectTaskOverdueAndFilters 逾期任务在待整改/逾期筛选与计数中的行为。
func TestRectTaskOverdueAndFilters(t *testing.T) {
	db := newRectTestDB(t)
	assignee := &model.User{Phone: "13900000002", Name: "钱工", Role: constants.RoleWorker}
	inspector := &model.User{Phone: "13900000003", Name: "孙监理", Role: constants.RoleInspector}
	if err := db.Create(assignee).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(inspector).Error; err != nil {
		t.Fatal(err)
	}

	// 任务1：逾期未整改
	ins1, item1 := seedRectFixture(t, db, assignee.ID, inspector.ID, false, time.Now())
	t1, err := newRectSvc(db).Register(ins1, item1, assignee.ID, time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// 任务2：未逾期待整改
	ins2, item2 := seedRectFixture(t, db, assignee.ID, inspector.ID, false, time.Now())
	if _, err := newRectSvc(db).Register(ins2, item2, assignee.ID, time.Now().Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if n, _ := newRectSvc(db).CountPending(); n != 2 {
		t.Fatalf("want 2 pending, got %d", n)
	}
	if n, _ := newRectSvc(db).CountOverdue(); n != 1 {
		t.Fatalf("want 1 overdue, got %d", n)
	}

	svc := newRectSvc(db)
	list, total, err := svc.List(1, 10, "overdue", 0, false)
	if err != nil || total != 1 {
		t.Fatalf("overdue filter: total=%d err=%v", total, err)
	}
	if !list[0].Overdue || list[0].ID != t1.ID {
		t.Fatalf("overdue list should return task %d first", t1.ID)
	}

	list, total, _ = svc.List(1, 10, "pending_rectification", 0, false)
	if total != 2 {
		t.Fatalf("pending_rectification filter: total=%d, want 2", total)
	}
	// 逾期任务排在最前
	if list[0].ID != t1.ID {
		t.Fatalf("overdue task should be ordered first, got id=%d", list[0].ID)
	}

	// mine 过滤
	_, total, _ = svc.List(1, 10, "pending_rectification", assignee.ID, true)
	if total != 2 {
		t.Fatalf("mine filter total=%d, want 2", total)
	}
	_, total, _ = svc.List(1, 10, "pending_rectification", inspector.ID, true)
	if total != 0 {
		t.Fatalf("other user mine filter total=%d, want 0", total)
	}
}
