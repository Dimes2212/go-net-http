package server

import (
	"go-net-http/internal/auth"
	"go-net-http/internal/task"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	router *gin.Engine,
	authHandler *auth.Handler,
	authService *auth.Service,
	taskHandler *task.Handler,
) {
	router.GET("/health", healthHandler)
	router.POST("/register", authHandler.Registration)
	router.POST("/login", authHandler.LoggingIn)

	protected := router.Group("")
	protected.Use(authService.RequireAuth())

	protected.POST("/logout", authHandler.Logout)
	protected.POST("/tasks", taskHandler.CreateTask)
	protected.GET("/tasks", taskHandler.ListTasks)
	protected.GET("/tasks/:id", taskHandler.GetTask)
	protected.PATCH("/tasks/:id", taskHandler.UpdateTask)
	protected.POST("/tasks/:id/attachment", taskHandler.UploadAttachment)
	protected.GET("/tasks/:id/attachment", taskHandler.DownloadAttachment)
}
