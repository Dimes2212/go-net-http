package server

import (
	"go-net-http/internal/auth"
	"go-net-http/internal/task"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, authHandler *auth.Handler, authService *auth.Service, taskHandler *task.Handler) {
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /register", authHandler.Registration)
	mux.HandleFunc("POST /login", authHandler.LoggingIn)

	mux.Handle(
		"POST /logout",
		authService.RequireAuth(
			http.HandlerFunc(authHandler.Logout),
		),
	)

	mux.Handle(
		"POST /tasks",
		authService.RequireAuth(
			http.HandlerFunc(taskHandler.CreateTask),
		),
	)

	mux.Handle(
		"GET /tasks",
		authService.RequireAuth(
			http.HandlerFunc(taskHandler.ListTasks),
		),
	)

	mux.Handle(
		"GET /tasks/{id}",
		authService.RequireAuth(
			http.HandlerFunc(taskHandler.GetTask),
		),
	)

	mux.Handle(
		"PATCH /tasks/{id}",
		authService.RequireAuth(
			http.HandlerFunc(taskHandler.UpdateTask),
		),
	)

	mux.Handle(
		"POST /tasks/{id}/attachment",
		authService.RequireAuth(
			http.HandlerFunc(taskHandler.UploadAttachment),
		),
	)

	mux.Handle(
		"GET /tasks/{id}/attachment",
		authService.RequireAuth(
			http.HandlerFunc(taskHandler.DownloadAttachment),
		),
	)
}
