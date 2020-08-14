package gcs_test

import (
	"bufio"
	"os"
	"testing"

	"github.com/dilip640/oss/gcs"
	"github.com/jinzhu/configor"
)

type Config struct {
	Bucket   string `env:"QOR_GCS_BUCKET"`
	Endpoint string `env:"QOR_GCS_ENDPOINT"`
}

var (
	client *gcs.Client
	config = Config{}
)

func init() {
	configor.Load(&config)

	client = gcs.New(&gcs.Config{Bucket: config.Bucket, Endpoint: config.Endpoint})
}

// func TestToRelativePath(t *testing.T) {
// 	urlMap := map[string]string{
// 		"https://mybucket.s3.amazonaws.com/myobject.ext": "/myobject.ext",
// 		"https://qor-example.com/myobject.ext":           "/myobject.ext",
// 		"//mybucket.s3.amazonaws.com/myobject.ext":       "/myobject.ext",
// 		"http://mybucket.s3.amazonaws.com/myobject.ext":  "/myobject.ext",
// 		"myobject.ext": "/myobject.ext",
// 	}

// 	for url, path := range urlMap {
// 		if client.ToRelativePath(url) != path {
// 			t.Errorf("%v's relative path should be %v, but got %v", url, path, client.ToRelativePath(url))
// 		}
// 	}
// }

func TestUpload(t *testing.T) {
	file, err := os.Open("sample.txt")
	if err != nil {
		t.Error(err)
		return
	}

	_, err = client.Put(file.Name(), bufio.NewReader(file))
	if err != nil {
		t.Error(err)
	}
}
