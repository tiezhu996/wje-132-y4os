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

// List 整改任务列表：filter=pending 待整改 / filter=overdue 已逾期。
func (h *RectificationTaskHandler) List(c *gin.Context) {
	var q dto.PageQuery
	_ = c.ShouldBindQuery(&q)
	q.Normalize()
	var inspectionID uint64
	if s := c.Query("inspection_id"); s != "" {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			inspectionID = v
		}
	}
	list, total, err := h.svc.List(q.Page, q.PageSize, c.Query("filter"), c.Query("status"), inspectionID)
	if err != nil {
		h.wrapError(c, err, "RectificationTask list failed")
		return
	}
	OK(c, pageResponse(list, total, q.Page, q.PageSize))
}

// Get 整改任务详情（含办理记录）。
func (h *RectificationTaskHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id] get: invalid id")
		return
	}
	task, records, err := h.svc.Get(id)
	if err != nil {
		h.wrapError(c, err, "RectificationTask get failed")
		return
	}
	OK(c, gin.H{"task": task, "records": records})
}

// Assign 登记责任人和整改期限。
func (h *RectificationTaskHandler) Assign(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id] assign: invalid id")
		return
	}
	var req dto.RectificationAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id="+strconv.FormatUint(id, 10)+"] assign: "+err.Error())
		return
	}
	task, err := h.svc.Assign(middleware.GetUserID(c), id, req.AssigneeID, req.Deadline)
	if err != nil {
		h.wrapError(c, err, "RectificationTask assign failed")
		return
	}
	OKWithMessage(c, constants.MsgRectificationAssigned, task)
}

// Submit 责任人提交整改说明，等待复查。
func (h *RectificationTaskHandler) Submit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id] submit: invalid id")
		return
	}
	var req dto.RectificationSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id="+strconv.FormatUint(id, 10)+"] submit: "+err.Error())
		return
	}
	task, err := h.svc.Submit(middleware.GetUserID(c), middleware.GetRole(c), id, req.Note)
	if err != nil {
		h.wrapError(c, err, "RectificationTask submit failed")
		return
	}
	OKWithMessage(c, constants.MsgRectificationSubmitted, task)
}

// Review 复查：通过则闭环，不通过则退回并保留原记录。
func (h *RectificationTaskHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id] review: invalid id")
		return
	}
	var req dto.RectificationReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "RectificationTask[id="+strconv.FormatUint(id, 10)+"] review: "+err.Error())
		return
	}
	task, err := h.svc.Review(middleware.GetUserID(c), id, req.Passed, req.Note)
	if err != nil {
		h.wrapError(c, err, "RectificationTask review failed")
		return
	}
	OKWithMessage(c, constants.MsgRectificationReviewed, task)
}

func (h *RectificationTaskHandler) wrapError(c *gin.Context, err error, ctx string) {
	var appErr *util.AppError
	if errors.As(err, &appErr) {
		c.Set("audit_detail", appErr.Message)
		h.logger.Warn("rectification handler error", "context", ctx, "error", appErr.Error())
		Fail(c, appErrorStatus(appErr.Code), appErr.Code, appErr.Message)
		return
	}
	h.logger.Error("rectification handler error", "context", ctx, "error", err.Error())
	Fail(c, http.StatusInternalServerError, constants.CodeInternalError, constants.MsgInternalError)
}
