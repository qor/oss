package gcs_test

import (
	"fmt"
	"testing"

	"github.com/dilip640/oss/gcs"
	"github.com/dilip640/oss/tests"
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

func TestAll(t *testing.T) {
	fmt.Println("testing GCS with public ACL")
	tests.TestAll(client, t)

	fmt.Println("testing GCS with private ACL")
	privateClient := gcs.New(&gcs.Config{Bucket: config.Bucket, Endpoint: config.Endpoint})
	tests.TestAll(privateClient, t)
}

func TestToRelativePath(t *testing.T) {
	urlMap := map[string]string{
		"https://storage.googleapis.com/pelto-test/myobject.ext": "myobject.ext",
		"//storage.googleapis.com/pelto-test/myobject.ext":       "myobject.ext",
		"gs://pelt-test/myobject.ext":                            "myobject.ext",
		"myobject.ext":                                           "myobject.ext",
	}

	for url, path := range urlMap {
		if client.ToRelativePath(url) != path {
			t.Errorf("%v's relative path should be %v, but got %v", url, path, client.ToRelativePath(url))
		}
	}
}
