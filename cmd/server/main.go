package main

import (
	"errors"
	"go-net-http/internal/auth"
	appserver "go-net-http/internal/server"
	"go-net-http/internal/task"
	"go-net-http/internal/user"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var Version = "1.0.0"

func main() {
	router := gin.Default()
	router.MaxMultipartMemory = 1 << 20

	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	taskRepository, err := task.NewRepository("data/tasks.json")
	if err != nil {
		log.Fatal(err)
	}

	fileStorage, err := task.NewFileStorage("data/attachments")
	if err != nil {
		log.Fatal(err)
	}

	taskService := task.NewService(taskRepository, fileStorage)
	taskHandler := task.NewHandler(taskService)

	userRepository, err := user.NewRepository("data/users.json")
	if err != nil {
		log.Fatal(err)
	}

	authService := auth.NewService(userRepository)
	authHandler := auth.NewHandler(authService)

	appserver.RegisterRoutes(router, authHandler, authService, taskHandler)

	log.Printf("server started on http://localhost:8080")

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("server failed: ", err)
	}
}
