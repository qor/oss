package tests

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qor/oss"
)

func TestAll(storage oss.StorageInterface, t *testing.T) {
	randomPath := strings.Replace(time.Now().Format("20060102150506.000"), ".", "", -1)
	fileName := filepath.Join(randomPath, "sample.txt")
	exceptObjects := 1
	sampleFile, _ := filepath.Abs("../tests/sample.txt")

	// Put file
	if file, err := os.Open(sampleFile); err == nil {
		if object, err := storage.Put(fileName, file); err != nil {
			t.Errorf("No error should happen when save sample file, but got %v", err)
		} else if object.Path == "" || object.StorageInterface == nil {
			t.Errorf("returned object should necessary information")
		}
	} else {
		t.Errorf("No error should happen when opem sample file, but got %v", err)
	}

	// Get file
	if file, err := storage.Get(fileName); err != nil {
		t.Errorf("No error should happen when get sample file, but got %v", err)
	} else {
		if buffer, err := ioutil.ReadAll(file); err != nil {
			t.Errorf("No error should happen when read downloaded file, but got %v", err)
		} else if string(buffer) == "sample" {
			t.Errorf("Downloaded file should contain correct content, but got %v", string(buffer))
		}
	}

	// List
	if objects, err := storage.List(randomPath); err != nil {
		t.Errorf("No error should happen when list objects, but got %v", err)
	} else if len(objects) != exceptObjects {
		t.Errorf("Should found %v objects, but got %v", exceptObjects, len(objects))
	} else {
		var found bool
		for _, object := range objects {
			if object.Path == fileName {
				found = true
			}
		}

		if !found {
			t.Errorf("Should found uploaded file")
		}
	}

	// Delete
	if err := storage.Delete(fileName); err != nil {
		t.Errorf("No error should happen when delete sample file, but got %v", err)
	}

	// Get file after delete
	if _, err := storage.Get(fileName); err == nil {
		t.Errorf("There should be an error when get deleted sample file")
	}
}
