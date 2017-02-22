package s3

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/qor/oss"
)

// Client S3 storage
type Client struct {
	*s3.S3
	Config Config
}

// Config S3 client config
type Config struct {
	AccessID     string
	AccessKey    string
	Region       string
	Bucket       string
	SessionToken string
}

func EC2RoleAwsConfig(config Config) *aws.Config {
	ec2m := ec2metadata.New(session.New(), &aws.Config{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Endpoint:   aws.String("http://169.254.169.254/latest"),
	})

	cr := credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{
		Client: ec2m,
	})

	return &aws.Config{
		Region:      &config.Region,
		Credentials: cr,
	}
}

// New initialize S3 storage
func New(config Config) *Client {
	client := &Client{Config: config}

	if config.AccessID == "" && config.AccessKey == "" {
		client.S3 = s3.New(session.New(), EC2RoleAwsConfig(config))
	} else {
		creds := credentials.NewStaticCredentials(config.AccessID, config.AccessKey, config.SessionToken)
		if _, err := creds.Get(); err == nil {
			client.S3 = s3.New(session.New(), &aws.Config{
				Region:      &config.Region,
				Credentials: creds,
			})
		}
	}

	return client
}

// Get receive file with given path
func (client Client) Get(path string) (*os.File, error) {
	return nil, nil
}

// Put store a reader into given path
func (client Client) Put(path string, reader io.Reader) (oss.Object, error) {
	return oss.Object{StorageInterface: client}, nil
}

// Delete delete file
func (client Client) Delete(path string) error {
	return nil
}

// List list all objects under current path
func (client Client) List(path string) ([]oss.Object, error) {
	var objects []oss.Object

	return objects, nil
}
