package auth

import (
	"encoding/json"
	"errors"
	"go-net-http/internal/user"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	service *Service
}
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type RegisterResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}
type LoggingInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type LoggingInResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Registration(w http.ResponseWriter, r *http.Request) {
	var request RegisterRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	createdUser, err := h.service.Register(request.Username, request.Password)
	if errors.Is(err, user.ErrUsernameTaken) {
		http.Error(
			w,
			"username already taken",
			http.StatusConflict,
		)
		return
	}
	if errors.Is(err, ErrInvalidUsername) || errors.Is(err, ErrPasswordTooShort) {
		http.Error(w, "invalid username or password", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	response := RegisterResponse{
		ID:       createdUser.ID,
		Username: createdUser.Username,
		IsAdmin:  createdUser.IsAdmin,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode register response: %v", err)
	}
}

func (h *Handler) LoggingIn(w http.ResponseWriter, r *http.Request) {
	var request LoggingInRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.service.Login(request.Username, request.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "failed to login", http.StatusInternalServerError)
		return
	}
	response := LoggingInResponse{
		ID:       result.User.ID,
		Username: result.User.Username,
		IsAdmin:  result.User.IsAdmin,
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    result.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  result.ExpiresAt,
		MaxAge:   int(time.Until(result.ExpiresAt).Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode login response: %v", err)
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	h.service.Logout(cookie.Value)

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}
