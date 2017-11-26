package aliyun_test

import (
	"fmt"
	"testing"

	"github.com/jinzhu/configor"
	"github.com/qor/oss/aliyun"
	"github.com/qor/oss/tests"
)

type Config struct {
	AccessID  string
	AccessKey string
	Bucket    string
	Endpoint  string
}

type AppConfig struct {
	Private Config
	Public  Config
}

var client, privateClient *aliyun.Client

func init() {
	config := AppConfig{}
	configor.New(&configor.Config{ENVPrefix: "ALIYUN"}).Load(&config)

	if len(config.Private.AccessID) == 0 {
		fmt.Println("No aliyun configuration")
		return
	}

	client = aliyun.New(&aliyun.Config{
		AccessID:  config.Public.AccessID,
		AccessKey: config.Public.AccessKey,
		Bucket:    config.Public.Bucket,
		Endpoint:  config.Public.Endpoint,
	})
	privateClient = aliyun.New(&aliyun.Config{
		AccessID:  config.Private.AccessID,
		AccessKey: config.Private.AccessKey,
		Bucket:    config.Private.Bucket,
		Endpoint:  config.Private.Endpoint,
	})
}

func TestAll(t *testing.T) {
	if client == nil {
		t.Skip(`skip because of no config: `)
	}
	clis := []*aliyun.Client{client, privateClient}
	for _, cli := range clis {
		tests.TestAll(cli, t)
	}
}
