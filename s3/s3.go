package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/qor/oss"
)

// Client S3 storage
type Client struct {
	S3     *s3.Client
	Config *Config
}

// Config S3 client config
type Config struct {
	AccessID         string
	AccessKey        string
	Region           string
	Bucket           string
	SessionToken     string
	ACL              types.ObjectCannedACL
	Endpoint         string
	S3Endpoint       string
	S3ForcePathStyle bool
	CacheControl     string

	AwsConfig        *aws.Config
	RoleARN          string
	EnableEC2IAMRole bool
}

// New initialize S3 storage
func New(cfg *Config) *Client {
	if cfg.ACL == "" {
		cfg.ACL = types.ObjectCannedACLPublicRead // default ACL
	}

	client := &Client{Config: cfg}

	// use role ARN to fetch credentials
	if cfg.RoleARN != "" {
		awsCfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			panic(err)
		}

		provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(awsCfg), cfg.RoleARN)
		creds := aws.NewCredentialsCache(provider)

		s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.Region = cfg.Region
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = cfg.S3ForcePathStyle

			o.Credentials = creds
		})

		client.S3 = s3Client
		return client
	}

	// use alreay configured aws config
	if cfg.AwsConfig != nil {
		s3Client := s3.NewFromConfig(*cfg.AwsConfig, func(o *s3.Options) {
			o.Region = cfg.Region
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = cfg.S3ForcePathStyle
		})

		client.S3 = s3Client
		return client
	}

	cfgOptions := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// use EC2 IAM role
	if cfg.EnableEC2IAMRole {
		cfgOptions = append(cfgOptions, config.WithCredentialsProvider(
			ec2rolecreds.New(),
		))
	}

	// use static credentials
	if cfg.AccessID != "" && cfg.AccessKey != "" {
		cfgOptions = append(cfgOptions, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessID, cfg.AccessKey, cfg.SessionToken),
		))
	}

	awsConfig, err := config.LoadDefaultConfig(context.TODO(), cfgOptions...)
	if err != nil {
		panic(err)
	}

	s3Client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.Region = cfg.Region
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = cfg.S3ForcePathStyle
	})

	client.S3 = s3Client
	return client
}

// Get receive file with given path
func (client Client) Get(path string) (file *os.File, err error) {
	readCloser, err := client.GetStream(path)

	ext := filepath.Ext(path)
	pattern := fmt.Sprintf("s3*%s", ext)

	if err == nil {
		if file, err = os.CreateTemp("/tmp", pattern); err == nil {
			defer readCloser.Close()
			_, err = io.Copy(file, readCloser)
			file.Seek(0, 0)
		}
	}

	return file, err
}

// GetStream get file as stream
func (client Client) GetStream(path string) (io.ReadCloser, error) {
	getResponse, err := client.S3.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(client.Config.Bucket),
		Key:    aws.String(client.ToRelativePath(path)),
	})

	return getResponse.Body, err
}

// Put store a reader into given path
func (client Client) Put(urlPath string, reader io.Reader) (*oss.Object, error) {
	if seeker, ok := reader.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}

	urlPath = client.ToRelativePath(urlPath)
	buffer, err := io.ReadAll(reader)

	fileType := mime.TypeByExtension(path.Ext(urlPath))
	if fileType == "" {
		fileType = http.DetectContentType(buffer)
	}

	params := &s3.PutObjectInput{
		Bucket:        aws.String(client.Config.Bucket), // required
		Key:           aws.String(urlPath),              // required
		ACL:           client.Config.ACL,
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(int64(len(buffer))),
		ContentType:   aws.String(fileType),
	}
	if client.Config.CacheControl != "" {
		params.CacheControl = aws.String(client.Config.CacheControl)
	}

	_, err = client.S3.PutObject(context.Background(), params)

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
	_, err := client.S3.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(client.Config.Bucket),
		Key:    aws.String(client.ToRelativePath(path)),
	})
	return err
}

// DeleteObjects delete files in bulk
func (client Client) DeleteObjects(paths []string) (err error) {
	var objs []types.ObjectIdentifier
	for _, v := range paths {
		var obj types.ObjectIdentifier
		obj.Key = aws.String(strings.TrimPrefix(client.ToRelativePath(v), "/"))
		objs = append(objs, obj)
	}
	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(client.Config.Bucket),
		Delete: &types.Delete{
			Objects: objs,
		},
	}

	_, err = client.S3.DeleteObjects(context.Background(), input)
	if err != nil {
		return
	}
	return
}

// List list all objects under current path
func (client Client) List(path string) ([]*oss.Object, error) {
	var objects []*oss.Object
	var prefix string

	if path != "" {
		prefix = strings.Trim(path, "/") + "/"
	}

	listObjectsResponse, err := client.S3.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(client.Config.Bucket),
		Prefix: aws.String(prefix),
	})

	if err == nil {
		for _, content := range listObjectsResponse.Contents {
			objects = append(objects, &oss.Object{
				Path:             client.ToRelativePath(*content.Key),
				Name:             filepath.Base(*content.Key),
				LastModified:     content.LastModified,
				StorageInterface: client,
			})
		}
	}

	return objects, err
}

// GetEndpoint get endpoint, FileSystem's endpoint is /
func (client Client) GetEndpoint() string {
	if client.Config.Endpoint != "" {
		return client.Config.Endpoint
	}

	endpoint := *client.S3.Options().BaseEndpoint
	for _, prefix := range []string{"https://", "http://"} {
		endpoint = strings.TrimPrefix(endpoint, prefix)
	}

	return client.Config.Bucket + "." + endpoint
}

var urlRegexp = regexp.MustCompile(`(https?:)?//((\w+).)+(\w+)/`)

// ToRelativePath process path to relative path
func (client Client) ToRelativePath(urlPath string) string {
	if urlRegexp.MatchString(urlPath) {
		if u, err := url.Parse(urlPath); err == nil {
			if client.Config.S3ForcePathStyle { // First part of path will be bucket name
				return strings.TrimPrefix(u.Path, "/"+client.Config.Bucket)
			}
			return u.Path
		}
	}

	if client.Config.S3ForcePathStyle { // First part of path will be bucket name
		return "/" + strings.TrimPrefix(urlPath, "/"+client.Config.Bucket+"/")
	}
	return "/" + strings.TrimPrefix(urlPath, "/")
}

// GetURL get public accessible URL
func (client Client) GetURL(path string) (url string, err error) {
	if client.Config.Endpoint == "" {

		if client.Config.ACL == types.ObjectCannedACLPrivate || client.Config.ACL == types.ObjectCannedACLAuthenticatedRead {

			presignClient := s3.NewPresignClient(client.S3)
			presignedGetURL, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
				Bucket: aws.String(client.Config.Bucket),
				Key:    aws.String(client.ToRelativePath(path)),
			}, func(opts *s3.PresignOptions) {
				opts.Expires = 1 * time.Hour
			})

			if err == nil && presignedGetURL != nil {
				return presignedGetURL.URL, nil
			}
		}
	}

	return path, nil
}

// Copy copy s3 file from "from" to "to"
func (client Client) Copy(from, to string) (err error) {
	_, err = client.S3.CopyObject(context.Background(), &s3.CopyObjectInput{
		Bucket:     aws.String(client.Config.Bucket),
		CopySource: aws.String(from),
		Key:        aws.String(to),
	})
	return
}
