package task

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

type FileStorage struct {
	root string
}

const MaxAttachmentSize int64 = 10 << 20 // 10 МБ

var ErrAttachmentTooLarge = errors.New("attachment is too large")
var ErrAttachmentNotFound = errors.New("attachment not found")

func NewFileStorage(root string) (*FileStorage, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create attachements directory: %w", err)
	}

	return &FileStorage{
		root: root,
	}, nil
}

func (s *FileStorage) Save(userID int64, taskID int64, source io.Reader) (int64, error) {
	userDirectory := filepath.Join(s.root, strconv.FormatInt(userID, 10))

	if err := os.MkdirAll(userDirectory, 0o700); err != nil {
		return 0, fmt.Errorf("create user attachement directory: %w", err)
	}

	temporaryFile, err := os.CreateTemp(userDirectory, ".upload-*")
	if err != nil {
		return 0, fmt.Errorf("create directory attachement: %w", err)
	}

	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	written, copyErr := io.Copy(temporaryFile, io.LimitReader(source, MaxAttachmentSize+1))

	closeErr := temporaryFile.Close()

	if copyErr != nil {
		return 0, fmt.Errorf("write attachment: %w", copyErr)
	}

	if closeErr != nil {
		return 0, fmt.Errorf("close attachment: %w", closeErr)
	}

	if written > MaxAttachmentSize {
		return 0, ErrAttachmentTooLarge
	}

	finalPath := s.path(userID, taskID)

	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return 0, fmt.Errorf("save attachment: %w", err)
	}

	return written, nil
}

func (s *FileStorage) path(userID int64, taskID int64) string {
	return filepath.Join(
		s.root,
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(taskID, 10),
	)
}

func (s *FileStorage) Open(userID int64, taskID int64) (*os.File, error) {
	file, err := os.Open(s.path(userID, taskID))

	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrAttachmentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}

	return file, nil
}

func (s *FileStorage) Delete(userID int64, taskID int64) error {
	err := os.Remove(s.path(userID, taskID))

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}

	return nil
}
