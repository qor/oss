# OSS

QOR OSS provides common interface to operate files in cloud storage, ftp, filesystem etc.

# Usage

Currently, QOR OSS only support file system, S3 and Qiniu. But you can easily implement your own storage strategies by implementing the interface.

```go
// StorageInterface define common API to operate storage
type StorageInterface interface {
  Get(path string) (*os.File, error)
  GetStream(path string) (io.ReadCloser, error)
  Put(path string, reader io.Reader) (*Object, error)
  Delete(path string) error
  List(path string) ([]*Object, error)
  GetEndpoint() string
  GetURL(path string) (string, error)
}
```

Here's an example about how to use [QOR OSS](https://github.com/qor/oss) with S3. After initialized the s3 storage, The functions in the interface are available.

```go
import (
	"github.com/oss/filesystem"
	"github.com/oss/s3"
	awss3 "github.com/aws/aws-sdk-go/s3"
)

func main() {
	storage := s3.New(s3.Config{AccessID: "access_id", AccessKey: "access_key", Region: "region", Bucket: "bucket", Endpoint: "cdn.getqor.com", ACL: awss3.BucketCannedACLPublicRead})
	// storage := filesystem.New("/tmp")

	// Save a reader interface into storage
	storage.Put("/sample.txt", reader)

	// Get file with path
	storage.Get("/sample.txt")

	// Delete file with path
	storage.Delete("/sample.txt")

	// List all objects under path
	storage.List("/")

	// Get Public Accessible URL (useful if current file saved privately)
	storage.GetURL("/sample.txt")
}
```

## License

Released under the [MIT License](http://opensource.org/licenses/MIT).
