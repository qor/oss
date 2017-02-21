package oss

import (
	"io"
	"os"
	"time"
)

// StorageInterface define common API to operate storage
type StorageInterface interface {
	Store(path string, reader io.Reader) (Object, error)
	Retrieve(path string) (*os.File, error)
	ListObjects(path string) ([]Object, error)
}

// Object content object
type Object struct {
	Path             string
	Name             string
	LastModified     *time.Time
	IsDir            bool
	StorageInterface StorageInterface
}

// Retrieve retrieve object's content
func (object Object) Retrieve() (*os.File, error) {
	return object.StorageInterface.Retrieve(object.Path)
}
