package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"go-net-http/internal/user"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	users *user.Repository

	sessionsMu sync.RWMutex
	sessions   map[string]Session
	sessionTTL time.Duration
}

type Session struct {
	UserID    int64
	ExpiresAt time.Time
}

type LoginResult struct {
	User      user.User
	SessionID string
	ExpiresAt time.Time
}

func NewService(users *user.Repository) *Service {
	return &Service{
		users:      users,
		sessions:   make(map[string]Session),
		sessionTTL: 24 * time.Hour,
	}
}

var ErrInvalidUsername = errors.New("username invalid")
var ErrPasswordTooShort = errors.New("password too short")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidSession = errors.New("invalid session")

func (s *Service) Register(username string, password string) (user.User, error) {
	username = strings.TrimSpace(username)

	if username == "" {
		return user.User{}, ErrInvalidUsername
	}

	if len(password) < 8 {
		return user.User{}, ErrPasswordTooShort
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return user.User{}, fmt.Errorf("hash password: %w", err)
	}
	person, err := s.users.Create(username, string(hashedPassword))
	if err != nil {
		return user.User{}, fmt.Errorf("create user: %w", err)
	}
	return person, nil
}

func (s *Service) Login(
	username string,
	password string,
) (LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 8 {
		return LoginResult{}, ErrInvalidCredentials
	}

	person, err := s.users.FindByUsername(username)
	if errors.Is(err, user.ErrUserNotFound) {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("find user: %w", err)
	}

	if err := s.users.CheckPassword(person, password); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"generate session ID: %w",
			err,
		)
	}

	expiresAt := time.Now().Add(s.sessionTTL)

	session := Session{
		UserID:    person.ID,
		ExpiresAt: expiresAt,
	}

	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()

	return LoginResult{
		User:      person,
		SessionID: sessionID,
		ExpiresAt: expiresAt,
	}, nil
}

func generateSessionID() (string, error) {
	buffer := make([]byte, 32)

	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random session ID: %w", err)
	}
	// rand.Read(buffer) позволяет в рандомном порядке заполнить наш массив байтов числами, которые позже станут одной строкой
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (s *Service) ValidateSession(sessionID string) (int64, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return 0, ErrInvalidSession
	}

	if time.Now().After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return 0, ErrInvalidSession
	}

	return session.UserID, nil

}

func (s *Service) Logout(sessionID string) {
	s.sessionsMu.Lock()
	delete(s.sessions, sessionID)
	s.sessionsMu.Unlock()
}
