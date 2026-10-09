package ingest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage хранит распакованные демки по sha256. Локальная реализация — LocalStorage;
// позже её можно заменить на S3/MinIO, не меняя воркер и API.
type Storage interface {
	// Save забирает файл srcPath (он может быть перемещён) и сохраняет его под ключом sha256.
	Save(sha256, srcPath string) error
	Open(sha256 string) (io.ReadCloser, error)
	Delete(sha256 string) error
}

// LocalStorage хранит демки в каталоге на диске: <Dir>/<sha256>.dem.
type LocalStorage struct{ Dir string }

func NewLocalStorage(dir string) (*LocalStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &LocalStorage{Dir: dir}, nil
}

func (s *LocalStorage) path(sha256 string) string { return filepath.Join(s.Dir, sha256+".dem") }

func (s *LocalStorage) Save(sha256, srcPath string) error {
	// rename атомарен в пределах одной ФС: временные файлы лежат рядом, в DATA_DIR
	if err := os.Rename(srcPath, s.path(sha256)); err != nil {
		return fmt.Errorf("сохранение демки: %w", err)
	}
	return nil
}

func (s *LocalStorage) Open(sha256 string) (io.ReadCloser, error) { return os.Open(s.path(sha256)) }

func (s *LocalStorage) Delete(sha256 string) error {
	err := os.Remove(s.path(sha256))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
