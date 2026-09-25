package router

import (
	"safetyplatform/internal/constants"
	"safetyplatform/internal/middleware"

	"github.com/gin-gonic/gin"
)

// registerRectificationRoutes 整改任务路由。
func (r *Router) registerRectificationRoutes(g *gin.RouterGroup) {
	tasks := g.Group("/rectification-tasks")
	tasks.Use(middleware.AuthRequired(r.cfg))
	tasks.GET("", r.rectification.List)
	tasks.GET("/:id", r.rectification.Get)
	tasks.POST("/:id/assign", middleware.RequireRole(constants.RoleAdmin, constants.RoleSafetyManager, constants.RoleInspector), r.rectification.Assign)
	tasks.POST("/:id/submit", r.rectification.Submit)
	tasks.POST("/:id/review", middleware.RequireRole(constants.RoleAdmin, constants.RoleSafetyManager, constants.RoleInspector), r.rectification.Review)
}
