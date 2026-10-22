package task

import (
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"

	"go-net-http/internal/auth"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type createTaskResponse struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateTask(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var request createTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	createdTask, err := h.service.Create(userID, request.Title, request.Description)
	switch {
	case errors.Is(err, ErrEmptyTitle):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	c.JSON(http.StatusCreated, createTaskResponse{
		ID:          createdTask.ID,
		UserID:      createdTask.UserID,
		Title:       createdTask.Title,
		Description: createdTask.Description,
		Done:        createdTask.Done,
	})
}

func (h *Handler) ListTasks(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var done *bool
	switch c.Query("done") {
	case "":
	case "true":
		value := true
		done = &value
	case "false":
		value := false
		done = &value
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "done must be true or false"})
		return
	}

	tasks, err := h.service.List(userID, done)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tasks"})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) GetTask(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	taskID, ok := taskIDFromPath(c)
	if !ok {
		return
	}

	foundTask, err := h.service.Get(userID, taskID)
	switch {
	case errors.Is(err, ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get task"})
		return
	}

	c.JSON(http.StatusOK, foundTask)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	taskID, ok := taskIDFromPath(c)
	if !ok {
		return
	}

	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	updatedTask, err := h.service.Update(userID, taskID, input)
	switch {
	case errors.Is(err, ErrEmptyTitle), errors.Is(err, ErrNoUpdateFields):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update task"})
		return
	}

	c.JSON(http.StatusOK, updatedTask)
}

func (h *Handler) UploadAttachment(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	taskID, ok := taskIDFromPath(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		MaxAttachmentSize+(1<<20),
	)

	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "attachment is too large"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}

	if c.Request.MultipartForm != nil {
		defer func() {
			if err := c.Request.MultipartForm.RemoveAll(); err != nil {
				log.Printf("remove temporary multipart files: %v", err)
			}
		}()
	}

	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}

	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open attachment"})
		return
	}
	defer file.Close()

	buffer := make([]byte, 512)
	readBytes, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read attachment"})
		return
	}

	contentType := http.DetectContentType(buffer[:readBytes])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read attachment"})
		return
	}

	attachment, err := h.service.UploadAttachment(
		userID,
		taskID,
		header.Filename,
		contentType,
		file,
	)
	switch {
	case errors.Is(err, ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	case errors.Is(err, ErrAttachmentExists):
		c.JSON(http.StatusConflict, gin.H{"error": "attachment already exists"})
		return
	case errors.Is(err, ErrAttachmentTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "attachment is too large"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save attachment"})
		return
	}

	c.JSON(http.StatusCreated, attachment)
}

func (h *Handler) DownloadAttachment(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	taskID, ok := taskIDFromPath(c)
	if !ok {
		return
	}

	file, attachment, err := h.service.OpenAttachment(userID, taskID)
	switch {
	case errors.Is(err, ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	case errors.Is(err, ErrAttachmentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "attachment not found"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open attachment"})
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read attachment"})
		return
	}

	c.Header("Content-Type", attachment.ContentType)
	c.Header("Content-Disposition", mime.FormatMediaType(
		"attachment",
		map[string]string{"filename": attachment.Name},
	))

	http.ServeContent(
		c.Writer,
		c.Request,
		attachment.Name,
		fileInfo.ModTime(),
		file,
	)
}

func authenticatedUserID(c *gin.Context) (int64, bool) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}

	return userID, true
}

func taskIDFromPath(c *gin.Context) (int64, bool) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return 0, false
	}

	return taskID, true
}
