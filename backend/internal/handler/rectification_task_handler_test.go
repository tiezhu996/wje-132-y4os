package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"safetyplatform/internal/config"
	"safetyplatform/internal/constants"
	"safetyplatform/internal/middleware"
	"safetyplatform/internal/model"
	"safetyplatform/internal/repository"
	"safetyplatform/internal/service"
	"safetyplatform/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	managerToken = "token-manager"
	workerToken  = "token-worker"
	otherToken   = "token-other"

	httpDBSeq int64
)

// setupRectRouter 构建连接内存 SQLite 的真实路由（JWT 中间件打桩为注入固定用户）。
func setupRectRouter(t *testing.T) (*gin.Engine, *gorm.DB, uint64, uint64) {
	t.Helper()
	dsn := fmt.Sprintf("file:http%d?mode=memory&cache=shared", atomic.AddInt64(&httpDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.SafetyInspection{}, &model.InspectionItem{},
		&model.RectificationTask{}, &model.RectificationHistory{},
	); err != nil {
		t.Fatal(err)
	}

	manager := &model.User{Phone: "13700000001", Name: "李监理", Role: constants.RoleInspector}
	worker := &model.User{Phone: "13700000002", Name: "赵工", Role: constants.RoleWorker}
	other := &model.User{Phone: "13700000003", Name: "钱工", Role: constants.RoleWorker}
	for _, u := range []*model.User{manager, worker, other} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}

	ins := &model.SafetyInspection{
		Name: "现场安全检查", InspectionType: constants.InspectionRoutine, Area: "一层",
		InspectionDate: time.Now(), InspectorID: manager.ID, Status: constants.InspectionFailed,
	}
	if err := db.Create(ins).Error; err != nil {
		t.Fatal(err)
	}
	item := &model.InspectionItem{InspectionID: ins.ID, ItemName: "配电箱接地", Passed: false, Remark: "未接地"}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}

	logger := util.NewLogger(slog.LevelError)
	insRepo := repository.NewSafetyInspectionRepository(db)
	itemRepo := repository.NewInspectionItemRepository(db)
	userRepo := repository.NewUserRepository(db)
	rectSvc := service.NewRectificationTaskService(db, repository.NewRectificationTaskRepository(db), insRepo, itemRepo, userRepo, logger)
	rectHandler := NewRectificationTaskHandler(rectSvc, logger)
	userSvc := service.NewUserService(userRepo, logger)
	userHandler := NewUserHandler(userSvc, logger)

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpireHours: 72}
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// JWT 打桩：按 token 解析出固定用户，注入的字段与 AuthRequired 一致。
	tokenUsers := map[string]struct {
		id   uint64
		role string
	}{
		managerToken: {manager.ID, constants.RoleInspector},
		workerToken:  {worker.ID, constants.RoleWorker},
		otherToken:   {other.ID, constants.RoleWorker},
	}
	r.Use(func(c *gin.Context) {
		tok := c.GetHeader("Authorization")
		if u, ok := tokenUsers[tok]; ok {
			c.Set("user_id", u.id)
			c.Set("phone", "")
			c.Set("role", u.role)
		}
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		c.Set("jwt_secret", cfg.JWTSecret)
		c.Set("jwt_expire_hours", cfg.JWTExpireHours)
	})

	v1 := r.Group("/api/v1")
	// 用户选项
	v1.GET("/users/options", func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": constants.CodeUnauthorized})
			return
		}
		c.Next()
	}, userHandler.Options)

	tasks := v1.Group("/rectification-tasks")
	tasks.Use(authStub())
	tasks.GET("", rectHandler.List)
	tasks.POST("", middleware.RequireRole(constants.RoleAdmin, constants.RoleSafetyManager, constants.RoleInspector), rectHandler.Register)
	tasks.GET("/by-inspection/:id", rectHandler.ListByInspection)
	tasks.GET("/:id", rectHandler.Get)
	tasks.POST("/:id/submit", rectHandler.Submit)
	tasks.POST("/:id/review", middleware.RequireRole(constants.RoleAdmin, constants.RoleSafetyManager, constants.RoleInspector), rectHandler.Review)

	return r, db, ins.ID, item.ID
}

func authStub() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": constants.CodeUnauthorized})
			return
		}
		c.Next()
	}
}

func doJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// TestRectAPIEndToEnd 覆盖鉴权、登记、提交、退回、复查、筛选、重复登记冲突。
func TestRectAPIEndToEnd(t *testing.T) {
	r, _, insID, itemID := setupRectRouter(t)

	// 未登录被拒
	if code, _ := doJSON(t, r, "GET", "/api/v1/rectification-tasks", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("unauth list code=%d", code)
	}
	// 工人不能登记
	code, body := doJSON(t, r, "POST", "/api/v1/rectification-tasks", workerToken, map[string]any{
		"inspection_id": insID, "item_id": itemID, "assignee_id": 2,
		"deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	if code != http.StatusForbidden {
		t.Fatalf("worker register code=%d body=%v", code, body)
	}

	// 监理登记
	code, body = doJSON(t, r, "POST", "/api/v1/rectification-tasks", managerToken, map[string]any{
		"inspection_id": insID, "item_id": itemID, "assignee_id": 2,
		"deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("register code=%d body=%v", code, body)
	}
	data := body["data"].(map[string]any)
	taskID := int64(data["id"].(float64))
	if data["status"].(string) != constants.RectTaskPending {
		t.Fatalf("new task status=%v", data["status"])
	}

	// 同一项重复登记 -> 409
	code, _ = doJSON(t, r, "POST", "/api/v1/rectification-tasks", managerToken, map[string]any{
		"inspection_id": insID, "item_id": itemID, "assignee_id": 2,
		"deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	if code != http.StatusConflict {
		t.Fatalf("duplicate register code=%d, want 409", code)
	}

	// 非责任人不能提交
	if code, _ = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/submit", taskID), otherToken,
		map[string]any{"rectification_note": "我整改了"}); code != http.StatusForbidden {
		t.Fatalf("non-assignee submit code=%d, want 403", code)
	}

	// 责任人提交
	code, body = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/submit", taskID), workerToken,
		map[string]any{"rectification_note": "已完成接地极施工"})
	if code != http.StatusOK {
		t.Fatalf("submit code=%d body=%v", code, body)
	}

	// 工人不能复查
	if code, _ = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/review", taskID), workerToken,
		map[string]any{"approved": true}); code != http.StatusForbidden {
		t.Fatalf("worker review code=%d, want 403", code)
	}

	// 退回但不填意见 -> 422
	if code, _ = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/review", taskID), managerToken,
		map[string]any{"approved": false, "review_note": ""}); code != http.StatusUnprocessableEntity {
		t.Fatalf("reject without note code=%d, want 422", code)
	}

	// 退回
	if code, body = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/review", taskID), managerToken,
		map[string]any{"approved": false, "review_note": "接地电阻未达标"}); code != http.StatusOK {
		t.Fatalf("reject code=%d body=%v", code, body)
	}

	// 责任人重新提交（同一项只保留一条任务）
	if code, _ = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/submit", taskID), workerToken,
		map[string]any{"rectification_note": "已加打接地极，电阻测试合格"}); code != http.StatusOK {
		t.Fatalf("resubmit code=%d", code)
	}

	// 复查通过
	if code, body = doJSON(t, r, "POST", fmt.Sprintf("/api/v1/rectification-tasks/%d/review", taskID), managerToken,
		map[string]any{"approved": true, "review_note": "合格"}); code != http.StatusOK {
		t.Fatalf("approve code=%d body=%v", code, body)
	}

	// 待整改筛选不含已通过任务
	code, body = doJSON(t, r, "GET", "/api/v1/rectification-tasks?status=pending_rectification", managerToken, nil)
	if code != http.StatusOK || body["data"].(map[string]any)["total"].(float64) != 0 {
		t.Fatalf("pending filter after approve: code=%d body=%v", code, body["data"])
	}

	// 详情含完整流转记录（5 条）
	code, body = doJSON(t, r, "GET", fmt.Sprintf("/api/v1/rectification-tasks/%d", taskID), managerToken, nil)
	if code != http.StatusOK {
		t.Fatalf("get code=%d", code)
	}
	histories := body["data"].(map[string]any)["histories"].([]any)
	if len(histories) != 5 {
		t.Fatalf("want 5 histories, got %d", len(histories))
	}

	// 非法筛选参数 -> 400
	if code, _ = doJSON(t, r, "GET", "/api/v1/rectification-tasks?status=bogus", managerToken, nil); code != http.StatusBadRequest {
		t.Fatalf("invalid filter code=%d, want 400", code)
	}
}

// TestRectAPIOverdueFilter 逾期任务筛选与醒目标记。
func TestRectAPIOverdueFilter(t *testing.T) {
	r, _, insID, itemID := setupRectRouter(t)

	// 登记一个期限在昨天的任务
	deadline := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	if code, body := doJSON(t, r, "POST", "/api/v1/rectification-tasks", managerToken, map[string]any{
		"inspection_id": insID, "item_id": itemID, "assignee_id": 2, "deadline": deadline,
	}); code != http.StatusOK {
		t.Fatalf("register code=%d body=%v", code, body)
	}

	code, body := doJSON(t, r, "GET", "/api/v1/rectification-tasks?status=overdue", managerToken, nil)
	if code != http.StatusOK {
		t.Fatalf("overdue list code=%d", code)
	}
	data := body["data"].(map[string]any)
	if data["total"].(float64) != 1 {
		t.Fatalf("overdue total=%v, want 1", data["total"])
	}
	row := data["list"].([]any)[0].(map[string]any)
	if row["overdue"] != true {
		t.Fatalf("overdue flag should be true")
	}
	if row["assignee_name"].(string) != "赵工" {
		t.Fatalf("assignee_name=%v", row["assignee_name"])
	}

	// 待整改筛选同样包含
	_, body = doJSON(t, r, "GET", "/api/v1/rectification-tasks?status=pending_rectification", managerToken, nil)
	if body["data"].(map[string]any)["total"].(float64) != 1 {
		t.Fatal("overdue task must still count as pending rectification")
	}
}
