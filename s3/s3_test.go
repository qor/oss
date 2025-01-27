package s3_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jinzhu/configor"
	"github.com/qor/oss/s3"
	"github.com/qor/oss/tests"
)

type Config struct {
	AccessID  string `env:"QOR_AWS_ACCESS_KEY_ID"`
	AccessKey string `env:"QOR_AWS_SECRET_ACCESS_KEY"`
	Region    string `env:"QOR_AWS_REGION"`
	Bucket    string `env:"QOR_AWS_BUCKET"`
	Endpoint  string `env:"QOR_AWS_ENDPOINT"`
}

var (
	client *s3.Client
	config = Config{}
)

func init() {
	configor.Load(&config)

	client = s3.New(&s3.Config{AccessID: config.AccessID, AccessKey: config.AccessKey, Region: config.Region, Bucket: config.Bucket, Endpoint: config.Endpoint})
}

func TestAll(t *testing.T) {
	fmt.Println("testing S3 with public ACL")
	tests.TestAll(client, t)

	fmt.Println("testing S3 with private ACL")
	privateClient := s3.New(&s3.Config{AccessID: config.AccessID, AccessKey: config.AccessKey, Region: config.Region, Bucket: config.Bucket, ACL: types.ObjectCannedACLPrivate, Endpoint: config.Endpoint})
	tests.TestAll(privateClient, t)

	fmt.Println("testing S3 with AuthenticatedRead ACL")
	authenticatedReadClient := s3.New(&s3.Config{AccessID: config.AccessID, AccessKey: config.AccessKey, Region: config.Region, Bucket: config.Bucket, ACL: types.ObjectCannedACLAuthenticatedRead, Endpoint: config.Endpoint})
	tests.TestAll(authenticatedReadClient, t)
}

func TestToRelativePath(t *testing.T) {
	urlMap := map[string]string{
		"https://mybucket.s3.amazonaws.com/myobject.ext": "/myobject.ext",
		"https://qor-example.com/myobject.ext":           "/myobject.ext",
		"//mybucket.s3.amazonaws.com/myobject.ext":       "/myobject.ext",
		"http://mybucket.s3.amazonaws.com/myobject.ext":  "/myobject.ext",
		"myobject.ext": "/myobject.ext",
	}

	for url, path := range urlMap {
		if client.ToRelativePath(url) != path {
			t.Errorf("%v's relative path should be %v, but got %v", url, path, client.ToRelativePath(url))
		}
	}
}

func TestToS3Key(t *testing.T) {
	urlMap := map[string]string{
		"https://mybucket.s3.amazonaws.com/test/myobject.ext": "test/myobject.ext",
		"https://qor-example.com/myobject.ext":                "myobject.ext",
		"//mybucket.s3.amazonaws.com/myobject.ext":            "myobject.ext",
		"http://mybucket.s3.amazonaws.com/myobject.ext":       "myobject.ext",
		"/test/myobject.ext":                                  "test/myobject.ext",
	}

	for url, path := range urlMap {
		if client.ToS3Key(url) != path {
			t.Errorf("%v's relative path should be %v, but got %v", url, path, client.ToRelativePath(url))
		}
	}
}

func TestToRelativePathWithS3ForcePathStyle(t *testing.T) {
	urlMap := map[string]string{
		"https://s3.amazonaws.com/mybucket/myobject.ext": "/myobject.ext",
		"https://qor-example.com/myobject.ext":           "/myobject.ext",
		"//s3.amazonaws.com/mybucket/myobject.ext":       "/myobject.ext",
		"http://s3.amazonaws.com/mybucket/myobject.ext":  "/myobject.ext",
		"/mybucket/myobject.ext":                         "/myobject.ext",
		"myobject.ext":                                   "/myobject.ext",
	}

	client := s3.New(&s3.Config{AccessID: config.AccessID, AccessKey: config.AccessKey, Region: config.Region, Bucket: "mybucket", S3ForcePathStyle: true, Endpoint: config.Endpoint})

	for url, path := range urlMap {
		if client.ToRelativePath(url) != path {
			t.Errorf("%v's relative path should be %v, but got %v", url, path, client.ToRelativePath(url))
		}
	}
}

const testDir = "/unit_test_dir"
const testPath = testDir + "/testfile.txt"
const testPath2 = testDir + "/testfile2.txt"
const testPath3 = testDir + "/testfile3.txt"

const testContent = "test content"

func TestAllOperations(t *testing.T) {
	client := s3.New(&s3.Config{AccessID: config.AccessID, AccessKey: config.AccessKey, Region: config.Region, Bucket: config.Bucket, Endpoint: config.Endpoint})

	// Step 1: Put testPath
	reader := strings.NewReader(testContent)
	_, err := client.Put(testPath, reader)
	if err != nil {
		t.Errorf("Put testPath failed: %v", err)
	}

	// Step 2: Get testPath
	file, err := client.Get(testPath)
	if err != nil {
		t.Errorf("Get testPath failed: %v", err)
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		t.Errorf("Expected no error reading from file, got %v", err)
	}

	if string(fileContent) != testContent {
		t.Error("File content does not match the original content")
	}

	// Step 3: Put testPath2
	reader2 := strings.NewReader(testContent)
	_, err = client.Put(testPath2, reader2)
	if err != nil {
		t.Errorf("Put testPath2 failed: %v", err)
	}

	// Step 4: GetStream testPath2
	stream, err := client.GetStream(testPath2)
	if err != nil {
		t.Errorf("GetStream testPath2 failed: %v", err)
	}
	if stream == nil {
		t.Error("Expected stream to be non-nil")
	}

	retrievedContent, err := io.ReadAll(stream)
	if err != nil {
		t.Errorf("Expected no error reading from stream, got %v", err)
	}

	if string(retrievedContent) != testContent {
		t.Error("Retrieved content does not match the original content")
	}

	// Step 5: Copy testPath2 to testPath3
	err = client.Copy(testPath2, testPath3)
	if err != nil {
		t.Errorf("Copy testPath2 to testPath3 failed: %v", err)
	}

	// Step 6: List
	objects, err := client.List(testDir)
	if err != nil {
		t.Errorf("List failed: %v", err)
	}
	if len(objects) != 3 {
		t.Error("Expected at 3 objects in the list, got", len(objects))
	}

	// Step 7: Delete testPath
	err = client.Delete(testPath)
	if err != nil {
		t.Errorf("Delete testPath failed: %v", err)
	}

	// Step 8: List
	objects, err = client.List(testDir)
	if err != nil {
		t.Errorf("List after delete testPath failed: %v", err)
	}

	if len(objects) != 2 {
		t.Error("Expected at 2 objects in the list, got", len(objects))
	}

	// Step 9: DeleteObjects (delete testPath2 and testPath3)
	err = client.DeleteObjects([]string{testPath2, testPath3})
	if err != nil {
		t.Errorf("DeleteObjects failed: %v", err)
	}

	// Step 10: List
	objects, err = client.List(testDir)
	if err != nil {
		t.Errorf("List after DeleteObjects failed: %v", err)
	}
	if len(objects) != 0 {
		t.Error("Expected no objects in the list after deletion, got", len(objects))
	}
}
