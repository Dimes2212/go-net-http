package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/bcrypt"
)

type Repository struct {
	mu     sync.RWMutex
	nextID atomic.Int64
	path   string
}

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	IsAdmin      bool   `json:"is_admin"`
}

var ErrUsernameTaken = errors.New("username already taken")
var ErrUserNotFound = errors.New("user not found")

func NewRepository(path string) (*Repository, error) {
	repository := &Repository{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}

	var users []User

	if err = json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("decode users %w", err)
	}

	var lastID int64 = 0

	for _, currentUser := range users {
		if currentUser.ID > lastID {
			lastID = currentUser.ID
		}
	}

	repository.nextID.Store(lastID)

	return repository, nil
}

func (r *Repository) readUsers() ([]User, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}

	var users []User

	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	return users, nil
}

func (r *Repository) writeUsers(users []User) error {
	data, err := json.MarshalIndent(users, "", " ")
	if err != nil {
		return fmt.Errorf("encode users: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0o600); err != nil {
		return fmt.Errorf("write users %w", err)
	}

	return nil
}

func (r *Repository) Create(username string, passwordHash string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	users, err := r.readUsers()
	if err != nil {
		return User{}, err
	}

	for _, currentUser := range users {
		if currentUser.Username == username {
			return User{}, ErrUsernameTaken
		}
	}

	newUser := User{
		ID:           r.nextID.Add(1),
		Username:     username,
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}

	users = append(users, newUser)

	if err := r.writeUsers(users); err != nil {
		return User{}, err
	}

	return newUser, nil
}

func (r *Repository) FindByUsername(
	username string,
) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users, err := r.readUsers()
	if err != nil {
		return User{}, err
	}

	for _, currentUser := range users {
		if currentUser.Username == username {
			return currentUser, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *Repository) CheckPassword(person User, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(person.PasswordHash), []byte(password))
	if err != nil {
		return fmt.Errorf("password incorrect %w", err)
	}
	return nil
}
