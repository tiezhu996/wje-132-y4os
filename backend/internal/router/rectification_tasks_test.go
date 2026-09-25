package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"safetyplatform/internal/config"
	"safetyplatform/internal/constants"
	"safetyplatform/internal/handler"
	"safetyplatform/internal/model"
	"safetyplatform/internal/repository"
	"safetyplatform/internal/service"
	"safetyplatform/internal/util"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestEngine 用内存 SQLite 装配完整 HTTP 服务，验证路由/中间件/JSON 全链路。
func newTestEngine(t *testing.T) (*httptest.Server, map[string]string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:rectification_http?mode=memory&cache=shared"), &gorm.Config{})
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
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpireHours: 1, RateLimitPerMinute: 1000, UploadDir: t.TempDir(), UploadMaxMB: 1}

	userRepo := repository.NewUserRepository(db)
	incidentRepo := repository.NewSafetyIncidentRepository(db)
	inspectionRepo := repository.NewSafetyInspectionRepository(db)
	itemRepo := repository.NewInspectionItemRepository(db)
	rectRepo := repository.NewRectificationTaskRepository(db)
	trainingRepo := repository.NewSafetyTrainingRepository(db)
	certRepo := repository.NewWorkerCertificationRepository(db)

	userSvc := service.NewUserService(userRepo, logger)
	incidentSvc := service.NewSafetyIncidentService(incidentRepo, userRepo, logger)
	rectTaskSvc := service.NewRectificationTaskService(db, rectRepo, userRepo, inspectionRepo, logger)
	inspectionSvc := service.NewSafetyInspectionService(db, inspectionRepo, itemRepo, userRepo, rectTaskSvc, logger)
	trainingSvc := service.NewSafetyTrainingService(trainingRepo, userRepo, logger)
	certSvc := service.NewWorkerCertificationService(certRepo, userRepo, logger)
	dashboardSvc := service.NewDashboardService(incidentSvc, inspectionSvc, trainingSvc, certSvc, rectTaskSvc, logger)

	r := New(cfg, db, logger,
		handler.NewUserHandler(userSvc, logger),
		handler.NewSafetyIncidentHandler(incidentSvc, logger),
		handler.NewSafetyInspectionHandler(inspectionSvc, logger),
		handler.NewInspectionItemHandler(inspectionSvc, logger),
		handler.NewRectificationTaskHandler(rectTaskSvc, logger),
		handler.NewSafetyTrainingHandler(trainingSvc, logger),
		handler.NewWorkerCertificationHandler(certSvc, logger),
		handler.NewDashboardHandler(dashboardSvc, logger),
		handler.NewUploadHandler(cfg, logger),
		handler.NewAuditLogHandler(db, logger),
	)
	srv := httptest.NewServer(r.Setup())
	t.Cleanup(srv.Close)

	tokens := map[string]string{}
	for _, u := range []struct {
		id    uint64
		phone string
		role  string
	}{{2, "13800000002", constants.RoleSafetyManager}, {3, "13800000003", constants.RoleInspector}, {4, "13800000004", constants.RoleWorker}} {
		tok, err := util.GenerateToken(cfg.JWTSecret, time.Hour, u.id, u.phone, u.role)
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		tokens[u.role] = tok
	}
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
	return srv, tokens
}

func doJSON(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

// TestRectificationHTTPFlow 整改任务全链路：执行检查 -> 登记 -> 提交 -> 退回 -> 再提交 -> 复查通过。
func TestRectificationHTTPFlow(t *testing.T) {
	srv, tokens := newTestEngine(t)
	inspector := tokens[constants.RoleInspector]
	worker := tokens[constants.RoleWorker]
	manager := tokens[constants.RoleSafetyManager]

	// 创建检查计划并执行，勾出不合格项。
	status, resp := doJSON(t, "POST", srv.URL+"/api/v1/inspections", inspector, map[string]any{
		"name": "临时用电专项检查", "inspection_type": "special", "area": "加工区",
		"inspection_date": time.Now().Format(time.RFC3339), "inspector_id": 3,
		"items": []map[string]any{{"item_name": "电缆绝缘完好"}, {"item_name": "配电箱接地"}},
	})
	if status != http.StatusOK {
		t.Fatalf("create inspection: status=%d resp=%v", status, resp)
	}
	inspectionID := uint64(resp["data"].(map[string]any)["id"].(float64))

	status, resp = doJSON(t, "GET", fmt.Sprintf("%s/api/v1/inspection-items/by-inspection/%d", srv.URL, inspectionID), inspector, nil)
	items := resp["data"].([]any)
	failItemID := uint64(items[0].(map[string]any)["id"].(float64))
	passItemID := uint64(items[1].(map[string]any)["id"].(float64))

	status, resp = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/inspections/%d/execute", srv.URL, inspectionID), inspector, map[string]any{
		"items": []map[string]any{
			{"id": failItemID, "item_name": "电缆绝缘完好", "passed": false, "remark": "电缆破损"},
			{"id": passItemID, "item_name": "配电箱接地", "passed": true},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("execute inspection: status=%d resp=%v", status, resp)
	}

	// 待整改列表出现该任务。
	status, resp = doJSON(t, "GET", srv.URL+"/api/v1/rectification-tasks?filter=pending", worker, nil)
	if status != http.StatusOK {
		t.Fatalf("list tasks: status=%d resp=%v", status, resp)
	}
	page := resp["data"].(map[string]any)
	if page["total"].(float64) != 1 {
		t.Fatalf("expected 1 pending task, got %v", page["total"])
	}
	task := page["list"].([]any)[0].(map[string]any)
	taskID := uint64(task["id"].(float64))
	if task["status"] != constants.RectificationPending || task["item_name"] != "电缆绝缘完好" {
		t.Fatalf("unexpected task: %v", task)
	}

	// 工人无权登记责任人。
	status, _ = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/assign", srv.URL, taskID), worker, map[string]any{
		"assignee_id": 4, "deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	if status != http.StatusForbidden {
		t.Fatalf("worker assign should be 403, got %d", status)
	}

	// 监理登记责任人和整改期限。
	status, resp = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/assign", srv.URL, taskID), inspector, map[string]any{
		"assignee_id": 4, "deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	if status != http.StatusOK {
		t.Fatalf("assign: status=%d resp=%v", status, resp)
	}

	// 非责任人提交被拒。
	status, _ = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/submit", srv.URL, taskID), manager, map[string]any{"note": "代提交"})
	if status != http.StatusForbidden {
		t.Fatalf("non-assignee submit should be 403, got %d", status)
	}

	// 责任人提交整改说明。
	status, resp = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/submit", srv.URL, taskID), worker, map[string]any{"note": "已更换破损电缆"})
	if status != http.StatusOK {
		t.Fatalf("submit: status=%d resp=%v", status, resp)
	}

	// 复查退回必须填原因。
	status, _ = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/review", srv.URL, taskID), inspector, map[string]any{"passed": false})
	if status == http.StatusOK {
		t.Fatal("review return without reason should fail")
	}

	// 退回并保留原记录。
	status, resp = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/review", srv.URL, taskID), inspector, map[string]any{"passed": false, "note": "现场仍不合格"})
	if status != http.StatusOK {
		t.Fatalf("review return: status=%d resp=%v", status, resp)
	}
	status, resp = doJSON(t, "GET", fmt.Sprintf("%s/api/v1/rectification-tasks/%d", srv.URL, taskID), inspector, nil)
	detail := resp["data"].(map[string]any)
	if detail["task"].(map[string]any)["status"] != constants.RectificationReturned {
		t.Fatalf("expected returned, got %v", detail["task"])
	}
	if len(detail["records"].([]any)) != 3 {
		t.Fatalf("expected 3 records preserved, got %d", len(detail["records"].([]any)))
	}

	// 重新提交并复查通过。
	doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/submit", srv.URL, taskID), worker, map[string]any{"note": "已重新整改"})
	status, resp = doJSON(t, "POST", fmt.Sprintf("%s/api/v1/rectification-tasks/%d/review", srv.URL, taskID), inspector, map[string]any{"passed": true, "note": "复查合格"})
	if status != http.StatusOK {
		t.Fatalf("review pass: status=%d resp=%v", status, resp)
	}

	// 复查通过后不再计入待整改。
	_, resp = doJSON(t, "GET", srv.URL+"/api/v1/rectification-tasks?filter=pending", worker, nil)
	if resp["data"].(map[string]any)["total"].(float64) != 0 {
		t.Fatalf("approved task should not be pending: %v", resp["data"])
	}

	// 仪表盘统计包含整改任务数据。
	_, resp = doJSON(t, "GET", srv.URL+"/api/v1/dashboard/stats", inspector, nil)
	rect := resp["data"].(map[string]any)["rectification"].(map[string]any)
	if rect["pending"].(float64) != 0 || rect["overdue"].(float64) != 0 {
		t.Fatalf("dashboard rectification stats mismatch: %v", rect)
	}
}
