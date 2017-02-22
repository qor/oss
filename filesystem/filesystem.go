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

// Get receive file with given path
func (fileSystem FileSystem) Get(path string) (*os.File, error) {
	return os.Open(fileSystem.GetFullPath(path))
}

// Put store a reader into given path
func (fileSystem FileSystem) Put(path string, reader io.ReadSeeker) (*oss.Object, error) {
	fullpath := fileSystem.GetFullPath(path)
	dst, err := os.Create(fullpath)

	if err == nil {
		reader.Seek(0, 0)
		_, err = io.Copy(dst, reader)
	}

	return oss.Object{Path: path, Name: filepath.Base(path), StorageInterface: fileSystem}, err
}

// Delete delete file
func (fileSystem FileSystem) Delete(path string) error {
	return os.Remove(fileSystem.GetFullPath(path))
}

// List list all objects under current path
func (fileSystem FileSystem) List(path string) ([]*oss.Object, error) {
	var objects []*oss.Object

	filepath.Walk(fileSystem.GetFullPath(path), func(path string, info os.FileInfo, err error) error {
		if err == nil {
			modTime := info.ModTime()
			objects = append(objects, &oss.Object{
				Path:             path,
				Name:             info.Name(),
				LastModified:     &modTime,
				IsDir:            info.IsDir(),
				StorageInterface: fileSystem,
			})
		}
		return nil
	})

	return objects, nil
}
