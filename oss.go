package oss

import (
	"io"
	"os"
	"time"
)

// StorageInterface define common API to operate storage
type StorageInterface interface {
	Get(path string) (*os.File, error)
	Put(path string, reader io.ReadSeeker) (*Object, error)
	Delete(path string) error
	List(path string) ([]*Object, error)
}

// Object content object
type Object struct {
	Path             string
	Name             string
	LastModified     *time.Time
	IsDir            bool
	StorageInterface StorageInterface
}

// Get retrieve object's content
func (object Object) Get() (*os.File, error) {
	return object.StorageInterface.Get(object.Path)
}
