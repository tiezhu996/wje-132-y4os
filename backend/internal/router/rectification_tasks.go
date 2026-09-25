package router

import (
	"safetyplatform/internal/constants"
	"safetyplatform/internal/middleware"

	"github.com/gin-gonic/gin"
)

// registerRectTaskRoutes 整改任务路由。
func (r *Router) registerRectTaskRoutes(g *gin.RouterGroup) {
	tasks := g.Group("/rectification-tasks")
	tasks.Use(middleware.AuthRequired(r.cfg))
	manager := middleware.RequireRole(constants.RoleAdmin, constants.RoleSafetyManager, constants.RoleInspector)
	reviewer := middleware.RequireRole(constants.RoleAdmin, constants.RoleSafetyManager, constants.RoleInspector)
	tasks.GET("", r.rectTask.List)
	tasks.POST("", manager, r.rectTask.Register)
	tasks.GET("/by-inspection/:id", r.rectTask.ListByInspection)
	tasks.GET("/:id", r.rectTask.Get)
	tasks.POST("/:id/submit", r.rectTask.Submit)
	tasks.POST("/:id/review", reviewer, r.rectTask.Review)
}
