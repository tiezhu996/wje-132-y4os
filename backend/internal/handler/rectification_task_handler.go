package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/dto"
	"safetyplatform/internal/middleware"
	"safetyplatform/internal/service"
	"safetyplatform/internal/util"

	"github.com/gin-gonic/gin"
)

// RectificationTaskHandler 整改任务 HTTP 处理器。
type RectificationTaskHandler struct {
	svc    *service.RectificationTaskService
	logger *slog.Logger
}

// NewRectificationTaskHandler 构造整改任务处理器。
func NewRectificationTaskHandler(svc *service.RectificationTaskService, logger *slog.Logger) *RectificationTaskHandler {
	return &RectificationTaskHandler{svc: svc, logger: logger}
}

// List 整改任务列表（支持待整改/已逾期/状态筛选、我的任务筛选）。
func (h *RectificationTaskHandler) List(c *gin.Context) {
	var q dto.RectTaskListQuery
	_ = c.ShouldBindQuery(&q)
	q.Normalize()
	if q.Status != "" {
		valid := false
		for _, v := range constants.RectTaskFilterValues {
			if q.Status == v {
				valid = true
				break
			}
		}
		if !valid {
			Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask list: invalid status filter")
			return
		}
	}
	mine := q.Mine == "1" || q.Mine == "true"
	list, total, err := h.svc.List(q.Page, q.PageSize, q.Status, middleware.GetUserID(c), mine)
	if err != nil {
		h.wrapError(c, err, "RectificationTask list failed")
		return
	}
	OK(c, pageResponse(list, total, q.Page, q.PageSize))
}

// Get 整改任务详情与流转记录。
func (h *RectificationTaskHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask get: invalid id")
		return
	}
	task, histories, err := h.svc.Get(id)
	if err != nil {
		h.wrapError(c, err, "RectificationTask get failed")
		return
	}
	OK(c, gin.H{"task": task, "histories": histories})
}

// Register 登记整改任务。
func (h *RectificationTaskHandler) Register(c *gin.Context) {
	var req dto.RectTaskRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask register: "+err.Error())
		return
	}
	task, err := h.svc.Register(req.InspectionID, req.ItemID, req.AssigneeID, req.Deadline)
	if err != nil {
		h.wrapError(c, err, "RectificationTask register failed")
		return
	}
	OKWithMessage(c, constants.MsgRectTaskRegistered, task)
}

// Submit 责任人提交整改说明。
func (h *RectificationTaskHandler) Submit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask submit: invalid id")
		return
	}
	var req dto.RectTaskSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id="+strconv.FormatUint(id, 10)+"] submit: "+err.Error())
		return
	}
	task, err := h.svc.Submit(id, middleware.GetUserID(c), req.RectificationNote, req.RectificationPhoto)
	if err != nil {
		h.wrapError(c, err, "RectificationTask submit failed")
		return
	}
	OKWithMessage(c, constants.MsgRectTaskSubmitted, task)
}

// Review 复查整改任务。
func (h *RectificationTaskHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask review: invalid id")
		return
	}
	var req dto.RectTaskReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id="+strconv.FormatUint(id, 10)+"] review: "+err.Error())
		return
	}
	task, err := h.svc.Review(id, middleware.GetUserID(c), req.Approved, req.ReviewNote)
	if err != nil {
		h.wrapError(c, err, "RectificationTask review failed")
		return
	}
	msg := constants.MsgRectTaskApproved
	if !req.Approved {
		msg = constants.MsgRectTaskRejected
	}
	OKWithMessage(c, msg, task)
}

// ListByInspection 某检查下的整改任务。
func (h *RectificationTaskHandler) ListByInspection(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask list by inspection: invalid inspection id")
		return
	}
	list, err := h.svc.ListByInspection(id)
	if err != nil {
		h.wrapError(c, err, "RectificationTask list by inspection failed")
		return
	}
	OK(c, list)
}

func (h *RectificationTaskHandler) wrapError(c *gin.Context, err error, ctx string) {
	var appErr *util.AppError
	if errors.As(err, &appErr) {
		c.Set("audit_detail", appErr.Message)
		h.logger.Warn("rectification task handler error", "context", ctx, "error", appErr.Error())
		Fail(c, appErrorStatus(appErr.Code), appErr.Code, appErr.Message)
		return
	}
	h.logger.Error("rectification task handler error", "context", ctx, "error", err.Error())
	Fail(c, http.StatusInternalServerError, constants.CodeInternalError, constants.MsgInternalError)
}
