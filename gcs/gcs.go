package gcs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/qor/oss"
	"google.golang.org/api/iterator"
)

// Client GCS storage
type Client struct {
	*storage.Client
	Config *Config
}

// Config GCS client config
type Config struct {
	Bucket   string
	Endpoint string
}

// New initialize GCS storage
func New(config *Config) *Client {

	client := &Client{Config: config}
	ctx := context.Background()
	gcsclient, err := storage.NewClient(ctx)
	if err != nil {
		log.Println(err)
	}

	client.Client = gcsclient

	return client
}

// Get receive file with given path
func (client Client) Get(path string) (file *os.File, err error) {
	readCloser, err := client.GetStream(path)

	ext := filepath.Ext(path)
	pattern := fmt.Sprintf("gcs*%s", ext)

	if err == nil {
		if file, err = ioutil.TempFile("/tmp", pattern); err == nil {
			defer readCloser.Close()
			_, err = io.Copy(file, readCloser)
			file.Seek(0, 0)
		}
	}

	return file, err
}

// GetStream get file as stream
func (client Client) GetStream(path string) (io.ReadCloser, error) {
	log.Println("GetStream " + path)
	bkt := client.Client.Bucket(client.Config.Bucket)
	obj := bkt.Object(client.ToRelativePath(path))
	r, err := obj.NewReader(context.TODO())
	return r, err
}

// Put store a reader into given path
func (client Client) Put(urlPath string, reader io.Reader) (*oss.Object, error) {
	log.Println("Put " + urlPath)
	if seeker, ok := reader.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}

	urlPath = client.ToRelativePath(urlPath)
	buffer, err := ioutil.ReadAll(reader)

	fileType := mime.TypeByExtension(path.Ext(urlPath))
	if fileType == "" {
		fileType = http.DetectContentType(buffer)
	}

	bkt := client.Client.Bucket(client.Config.Bucket)
	obj := bkt.Object(urlPath)
	w := obj.NewWriter(context.TODO())
	w.ContentType = fileType
	w.Write(buffer)

	if err := w.Close(); err != nil {
		return nil, err
	}

	now := time.Now()
	return &oss.Object{
		Path:             urlPath,
		Name:             filepath.Base(urlPath),
		LastModified:     &now,
		StorageInterface: client,
	}, err
}

// Delete delete file
func (client Client) Delete(path string) error {
	log.Println("Delete " + path)
	bkt := client.Client.Bucket(client.Config.Bucket)
	obj := bkt.Object(client.ToRelativePath(path))
	return obj.Delete(context.TODO())
}

// List list all objects under current path
func (client Client) List(path string) ([]*oss.Object, error) {
	log.Println("List " + path)
	var objects []*oss.Object
	var prefix string

	if path != "" {
		prefix = strings.Trim(path, "/") + "/"
	}

	query := &storage.Query{Prefix: prefix}
	bkt := client.Client.Bucket(client.Config.Bucket)
	it := bkt.Objects(context.TODO(), query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		objects = append(objects, &oss.Object{
			Path:             client.ToRelativePath(attrs.MediaLink),
			Name:             filepath.Base(attrs.Name),
			LastModified:     &attrs.Created,
			StorageInterface: client,
		})
	}

	return objects, nil
}

// GetEndpoint get endpoint, FileSystem's endpoint is /
func (client Client) GetEndpoint() string {
	endpoint := filepath.Join(client.Config.Endpoint, client.Config.Bucket)
	return endpoint
}

var urlRegexp = regexp.MustCompile("")

// ToRelativePath process path to relative path
func (client Client) ToRelativePath(urlPath string) string {
	log.Println("ToRelativePath " + urlPath)
	if urlRegexp.MatchString(urlPath) {
		if u, err := url.Parse(urlPath); err == nil {
			return strings.TrimPrefix(u.Path, "/"+client.Config.Bucket)
		}
	}

	return "/" + strings.TrimPrefix(urlPath, "/")
}

// GetURL get public accessible URL
func (client Client) GetURL(path string) (url string, err error) {
	log.Println("GetURL " + path)
	// if client.Endpoint == "" {
	// 	if client.Config.ACL == s3.BucketCannedACLPrivate || client.Config.ACL == s3.BucketCannedACLAuthenticatedRead {
	// 		getResponse, _ := client.S3.GetObjectRequest(&s3.GetObjectInput{
	// 			Bucket: aws.String(client.Config.Bucket),
	// 			Key:    aws.String(client.ToRelativePath(path)),
	// 		})

	// 		return getResponse.Presign(1 * time.Hour)
	// 	}
	// }

	return path, nil
}
