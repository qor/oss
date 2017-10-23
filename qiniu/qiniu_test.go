package qiniu_test

import (
	"testing"

	"github.com/jinzhu/configor"
	"github.com/qor/oss/qiniu"
	"github.com/qor/oss/tests"
)

type Config struct {
	AccessID  string `env:"QOR_QINIU_ACCESS_KEY_ID"`
	AccessKey string `env:"QOR_QINIU_SECRET_ACCESS_KEY"`
	Region    string `env:"QOR_QINIU_REGION"`
	Bucket    string `env:"QOR_QINIU_BUCKET"`
	Endpoint  string `env:"QOR_QINIU_ENDPOINT"`
}

var client *qiniu.Client

func init() {
	config := Config{}
	configor.Load(&config)
	if len(config.AccessID) == 0 {
		return
	}

	client = qiniu.New(&qiniu.Config{
		AccessID:  config.AccessID,
		AccessKey: config.AccessKey,
		Region:    config.Region,
		Bucket:    config.Bucket,
		Endpoint:  config.Endpoint,
	})
}

func TestAll(t *testing.T) {
	if client == nil {
		t.Skip("skip because of no config for QOR_QINIU_ACCESS_KEY_ID")
	}
	tests.TestAll(client, t)
}
