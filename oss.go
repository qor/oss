package oss

import (
	"io"
	"os"
	"time"
)

// Interface define common API to operate storage
type Interface interface {
	Store(path string, reader io.Reader) (Object, error)
	Retrieve(path string) (*os.File, error)
	ListObjects(path string) ([]Object, error)
}

// Object content object
type Object struct {
	Path         string
	Name         string
	LastModified *time.Time
	IsDir        bool
}

// Retrieve retrieve object's content
func (object Object) Retrieve(i Interface) (*os.File, error) {
	return i.Retrieve(object.Path)
}
