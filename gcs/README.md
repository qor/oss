# Google Cloud Storage

[GCS](https://pkg.go.dev/cloud.google.com/go/storage) backend for [QOR OSS](https://github.com/qor/oss)

## Usage

> Set ENV `GOOGLE_APPLICATION_CREDENTIALS` to service account

```go
import "github.com/qor/oss/gcs"

func main() {
  storage := gcs.New(gcs.Config{
    Bucket: "bucket",
    Endpoint: "https://storage.googleapis.com/",
  })

  // Save a reader interface into storage
  storage.Put("/sample.txt", reader)

  // Get file with path
  storage.Get("/sample.txt")

  // Get object as io.ReadCloser
  storage.GetStream("/sample.txt")

  // Delete file with path
  storage.Delete("/sample.txt")

  // List all objects under path
  storage.List("/")

  // Get Public Accessible URL (useful if current file saved privately)
  storage.GetURL("/sample.txt")
}
```


