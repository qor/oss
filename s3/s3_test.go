package s3_test

import (
	"testing"

	"github.com/jinzhu/configor"
	"github.com/qor/oss/s3"
	"github.com/qor/oss/tests"
)

type Config struct {
	AccessID  string `env:"QOR_AWS_ACCESS_KEY_ID"`
	AccessKey string `env:"QOR_AWS_SECRET_ACCESS_KEY"`
	Region    string `env:"QOR_AWS_REGION"`
	Bucket    string `env:"QOR_AWS_BUCKET"`
}

func TestAll(t *testing.T) {
	config := Config{}
	configor.Load(&config)

	client := s3.New(s3.Config{AccessID: config.AccessID, AccessKey: config.AccessKey, Region: config.Region, Bucket: config.Bucket})
	tests.TestAll(client, t)
}
