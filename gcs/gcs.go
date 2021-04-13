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
	bkt := client.Client.Bucket(client.Config.Bucket)
	obj := bkt.Object(client.ToRelativePath(path))
	r, err := obj.NewReader(context.TODO())
	_, err = obj.Attrs(context.TODO())
	return r, err
}

// Put store a reader into given path
func (client Client) Put(urlPath string, reader io.Reader) (*oss.Object, error) {
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
	path = strings.TrimPrefix(path, "/")
	bkt := client.Client.Bucket(client.Config.Bucket)
	obj := bkt.Object(client.ToRelativePath(path))
	err := obj.Delete(context.TODO())
	return err
}

// List list all objects under current path
func (client Client) List(path string) ([]*oss.Object, error) {
	var objects []*oss.Object
	var prefix string

	if path != "" {
		prefix = strings.Trim(path, "/")
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
			Path:             "/" + client.ToRelativePath(attrs.Name),
			Name:             filepath.Base(attrs.Name),
			LastModified:     &attrs.Created,
			StorageInterface: client,
		})
	}

	return objects, nil
}

// GetEndpoint get endpoint, FileSystem's endpoint is /
func (client Client) GetEndpoint() string {
	u, err := url.Parse(client.Config.Endpoint)
	if err != nil {
		log.Println(err)
	}
	u.Path = path.Join(u.Path, client.Config.Bucket)
	return u.String()
}

var urlRegexp = regexp.MustCompile(`(https?:)?//((\w+).)+(\w+)/`)

// ToRelativePath process path to relative path
func (client Client) ToRelativePath(urlPath string) string {
	if urlRegexp.MatchString(urlPath) {
		if u, err := url.Parse(urlPath); err == nil {
			urlPath = strings.TrimPrefix(u.Path, "/"+client.Config.Bucket+"/")
			urlPath = strings.TrimPrefix(urlPath, "/")
			return urlPath
		}
	}
	return strings.TrimPrefix(urlPath, "/")
}

// GetURL get public accessible URL
func (client Client) GetURL(path string) (string, error) {
	return path, nil
}
