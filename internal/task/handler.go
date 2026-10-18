package task

import (
	"encoding/json"
	"errors"
	"go-net-http/internal/auth"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
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
	return &Handler{
		service: service,
	}
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var request createTaskRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	createdTask, err := h.service.Create(userID, request.Title, request.Description)
	if errors.Is(err, ErrEmptyTitle) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}
	response := createTaskResponse{
		ID:          createdTask.ID,
		UserID:      userID,
		Title:       createdTask.Title,
		Description: createdTask.Description,
		Done:        createdTask.Done,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode create task response: %v", err)
	}
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var done *bool

	switch r.URL.Query().Get("done") {
	case "":

	case "true":
		value := true
		done = &value

	case "false":
		value := false
		done = &value

	default:
		http.Error(w, "done must be true or false", http.StatusBadRequest)
		return
	}

	tasks, err := h.service.List(userID, done)
	if err != nil {
		http.Error(w, "failed to get tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Printf("encode tasks response: %v", err)
	}
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	foundTask, err := h.service.Get(userId, taskID)
	if errors.Is(err, ErrTaskNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to get task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(foundTask); err != nil {
		log.Printf("encode task response: %v", err)
	}
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	var input UpdateInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	updatedTask, err := h.service.Update(userId, taskID, input)
	if errors.Is(err, ErrEmptyTitle) || errors.Is(err, ErrNoUpdateFields) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, ErrTaskNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "failed to update tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(updatedTask); err != nil {
		log.Printf("encode updated task response: %v", err)
	}
}

func (h *Handler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxAttachmentSize+(1<<20))

	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var MaxBytesError *http.MaxBytesError

		if errors.As(err, &MaxBytesError) {
			http.Error(w, "attachment is too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	if r.MultipartForm != nil {
		defer func() {
			if err := r.MultipartForm.RemoveAll(); err != nil {
				log.Printf("remove temporary multipart files: %v", err)
			}
		}()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field is required", http.StatusBadRequest)
		return
	}

	defer file.Close()

	buffer := make([]byte, 512)

	readBytes, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "failed to read attachment", http.StatusInternalServerError)
		return
	}

	contentType := http.DetectContentType(buffer[:readBytes])

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "failed to read attachment", http.StatusInternalServerError)
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
		http.Error(w, "task not found", http.StatusNotFound)
		return

	case errors.Is(err, ErrAttachmentExists):
		http.Error(w, "attachment already exists", http.StatusConflict)
		return

	case errors.Is(err, ErrAttachmentTooLarge):
		http.Error(w, "attachment is too large", http.StatusRequestEntityTooLarge)
		return

	case err != nil:
		http.Error(w, "failed to save attachment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(attachment); err != nil {
		log.Printf("encode attachment response: %v", err)
	}

}

func (h *Handler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	file, attachment, err := h.service.OpenAttachment(userID, taskID)

	switch {
	case errors.Is(err, ErrTaskNotFound):
		http.Error(w, "task not found", http.StatusNotFound)
		return

	case errors.Is(err, ErrAttachmentNotFound):
		http.Error(w, "attachment not found", http.StatusNotFound)
		return

	case err != nil:
		http.Error(w, "failed to open attachment", http.StatusInternalServerError)
		return
	}

	defer file.Close()

	fileInfo, err := file.Stat() //получение метаданных файла
	if err != nil {
		http.Error(w, "failed to read attachment", http.StatusInternalServerError)
		return
	}

	contentDisposition := mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Name}) //безопасное экраниование пробелов и тп
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Disposition", contentDisposition)

	http.ServeContent(
		w,
		r,
		attachment.Name,
		fileInfo.ModTime(),
		file,
	)
}
