package task

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

type Repository struct {
	mu     sync.RWMutex
	nextID atomic.Int64
	path   string
}

func NewRepository(path string) (*Repository, error) {
	repository := &Repository{path: path}

	tasks, err := repository.readTasks()
	if err != nil {
		return nil, err
	}

	var lastID int64

	for _, currentTask := range tasks {
		if currentTask.ID > lastID {
			lastID = currentTask.ID
		}
	}

	repository.nextID.Store(lastID)

	return repository, nil
}

func (r *Repository) readTasks() ([]Task, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, fmt.Errorf("read tasks: %w", err)
	}

	var tasks []Task

	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	return tasks, nil
}

func (r *Repository) writeTasks(tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tasks: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0o600); err != nil {
		return fmt.Errorf("write tasks: %w", err)
	}

	return nil
}

func (r *Repository) Create(
	userID int64,
	title string,
	description string,
) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tasks, err := r.readTasks()
	if err != nil {
		return Task{}, err
	}

	newTask := Task{
		ID:          r.nextID.Add(1),
		UserID:      userID,
		Title:       title,
		Description: description,
		Done:        false,
	}

	tasks = append(tasks, newTask)

	if err := r.writeTasks(tasks); err != nil {
		return Task{}, err
	}

	return newTask, nil
}

func (r *Repository) List(userID int64, done *bool) ([]Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tasks, err := r.readTasks()
	if err != nil {
		return nil, err
	}

	result := make([]Task, 0)

	for _, currentTask := range tasks {
		if currentTask.UserID != userID {
			continue
		}
		if done != nil && currentTask.Done != *done {
			continue
		}

		result = append(result, currentTask)
	}

	return result, nil
}

func (r *Repository) Get(userID int64, taskID int64) (Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tasks, err := r.readTasks()
	if err != nil {
		return Task{}, err
	}

	for _, currentTask := range tasks {
		if currentTask.ID == taskID && currentTask.UserID == userID {
			return currentTask, nil
		}
	}

	return Task{}, ErrTaskNotFound
}

func (r *Repository) Update(userID int64, taskID int64, input UpdateInput) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tasks, err := r.readTasks()
	if err != nil {
		return Task{}, err
	}

	for index := range tasks {
		currentTask := &tasks[index]

		if currentTask.ID != taskID || currentTask.UserID != userID {
			continue
		}

		if input.Title != nil {
			currentTask.Title = *input.Title
		}

		if input.Description != nil {
			currentTask.Description = *input.Description
		}

		if input.Done != nil {
			currentTask.Done = *input.Done
		}
		if err := r.writeTasks(tasks); err != nil {
			return Task{}, err
		}

		return *currentTask, nil
	}
	return Task{}, ErrTaskNotFound
}

func (r *Repository) SetAttachement(userId int64, taskID int64, attachment Attachment) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tasks, err := r.readTasks()
	if err != nil {
		return Task{}, err
	}

	for index := range tasks {
		currentTask := &tasks[index]

		if currentTask.ID != taskID || currentTask.UserID != userId {
			continue
		}

		currentTask.Attachment = &attachment

		if err := r.writeTasks(tasks); err != nil {
			return Task{}, err
		}

		return *currentTask, nil
	}

	return Task{}, ErrTaskNotFound
}
