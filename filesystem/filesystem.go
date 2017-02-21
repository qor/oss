package filesystem

import (
	"io"
	"os"
	"path/filepath"

	"github.com/qor/oss"
)

// FileSystem file system storage
type FileSystem struct {
	Base string
}

// New initialize FileSystem storage
func New(base string) *FileSystem {
	return &FileSystem{Base: base}
}

// GetFullPath get full path from absolute/relative path
func (fileSystem FileSystem) GetFullPath(path string) string {
	fullpath := path
	if !filepath.IsAbs(path) {
		fullpath, _ = filepath.Rel(fileSystem.Base, path)
	}
	return fullpath
}

// Store store a reader into given path
func (fileSystem FileSystem) Store(path string, reader io.Reader) (oss.Object, error) {
	fullpath := fileSystem.GetFullPath(path)
	if dst, err := os.Create(fullpath); err == nil {
		_, err = io.Copy(dst, reader)
	}

	return oss.Object{Path: path, Name: filepath.Base(path), StorageInterface: fileSystem}, err
}

// Retrieve receive file with given path
func (fileSystem FileSystem) Retrieve(path string) (*os.File, error) {
	return os.Open(fileSystem.GetFullPath(path))
}

// ListObjects list all objects under current path
func (fileSystem FileSystem) ListObjects(path string) ([]oss.Object, error) {
	return nil, nil
}
