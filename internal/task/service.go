package task

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"sync"
)

type Service struct {
	tasks *Repository
	files *FileStorage

	attachmentMu sync.Mutex
}

var ErrEmptyTitle = errors.New("title empty")
var ErrTaskNotFound = errors.New("task not found")
var ErrNoUpdateFields = errors.New("no update fields")
var ErrAttachmentExists = errors.New("attachment already exists")

func NewService(
	tasks *Repository,
	files *FileStorage,
) *Service {
	return &Service{
		tasks: tasks,
		files: files,
	}
}

func (s *Service) Create(userID int64, title string, description string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrEmptyTitle
	}

	return s.tasks.Create(userID, title, description)
}

func (s *Service) List(
	userID int64,
	done *bool,
) ([]Task, error) {
	return s.tasks.List(userID, done)
}

func (s *Service) Get(userID int64, taskID int64) (Task, error) {
	return s.tasks.Get(userID, taskID)
}

func (s *Service) Update(userID int64, taskID int64, input UpdateInput) (Task, error) {
	if input.Title == nil && input.Description == nil && input.Done == nil {
		return Task{}, ErrNoUpdateFields
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return Task{}, ErrEmptyTitle
		}

		input.Title = &title
	}

	return s.tasks.Update(userID, taskID, input)
}

func (s *Service) UploadAttachment(userID int64, taskID int64, name string, contentType string, source io.Reader) (Attachment, error) {
	s.attachmentMu.Lock()
	defer s.attachmentMu.Unlock()

	currentTask, err := s.tasks.Get(userID, taskID)
	if err != nil {
		return Attachment{}, err
	}

	if currentTask.Attachment != nil {
		return Attachment{}, ErrAttachmentExists
	}

	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Base(name)

	if name == "." || name == "/" {
		name = "attachment"
	}

	size, err := s.files.Save(userID, taskID, source)
	if err != nil {
		return Attachment{}, err
	}

	attachment := Attachment{
		Name:        name,
		ContentType: contentType,
		Size:        size,
	}

	if _, err := s.tasks.SetAttachement(userID, taskID, attachment); err != nil {
		cleanupErr := s.files.Delete(userID, taskID)
		return Attachment{}, errors.Join(err, cleanupErr)
	}

	return attachment, nil
}

func (s *Service) OpenAttachment(
	userID int64,
	taskID int64,
) (*os.File, Attachment, error) {
	currentTask, err := s.tasks.Get(userID, taskID)
	if err != nil {
		return nil, Attachment{}, err
	}

	if currentTask.Attachment == nil {
		return nil, Attachment{}, ErrAttachmentNotFound
	}

	file, err := s.files.Open(userID, taskID)
	if err != nil {
		return nil, Attachment{}, err
	}

	return file, *currentTask.Attachment, nil
}
