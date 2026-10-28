package server

import (
	"go-net-http/internal/auth"
	"go-net-http/internal/task"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(
	router chi.Router,
	authHandler *auth.Handler,
	authService *auth.Service,
	taskHandler *task.Handler,
) {
	router.Get("/health", healthHandler)
	router.Post("/register", authHandler.Registration)
	router.Post("/login", authHandler.LoggingIn)

	router.Group(func(protected chi.Router) {
		protected.Use(authService.RequireAuth)

		protected.Post("/logout", authHandler.Logout)
		protected.Post("/tasks", taskHandler.CreateTask)
		protected.Get("/tasks", taskHandler.ListTasks)
		protected.Get("/tasks/{id}", taskHandler.GetTask)
		protected.Patch("/tasks/{id}", taskHandler.UpdateTask)
		protected.Post("/tasks/{id}/attachment", taskHandler.UploadAttachment)
		protected.Get("/tasks/{id}/attachment", taskHandler.DownloadAttachment)
	})
}
