package auth

import (
	"errors"
	"net/http"
	"time"

	"go-net-http/internal/user"

	"github.com/gin-gonic/gin"
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
	return &Handler{service: service}
}

func (h *Handler) Registration(c *gin.Context) {
	var request RegisterRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	createdUser, err := h.service.Register(request.Username, request.Password)
	switch {
	case errors.Is(err, user.ErrUsernameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrPasswordTooShort):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid username or password"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, RegisterResponse{
		ID:       createdUser.ID,
		Username: createdUser.Username,
		IsAdmin:  createdUser.IsAdmin,
	})
}

func (h *Handler) LoggingIn(c *gin.Context) {
	var request LoggingInRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	result, err := h.service.Login(request.Username, request.Password)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"session_id",
		result.SessionID,
		int(time.Until(result.ExpiresAt).Seconds()),
		"/",
		"",
		c.Request.TLS != nil,
		true,
	)

	c.JSON(http.StatusOK, LoggingInResponse{
		ID:       result.User.ID,
		Username: result.User.Username,
		IsAdmin:  result.User.IsAdmin,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	sessionID, err := c.Cookie("session_id")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	h.service.Logout(sessionID)

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session_id", "", -1, "/", "", c.Request.TLS != nil, true)
	c.Status(http.StatusNoContent)
}
