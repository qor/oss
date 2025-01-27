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
	AccessID     string
	AccessKey    string
	Region       string
	Bucket       string
	SessionToken string
	ACL          types.ObjectCannedACL
	Endpoint     string

	S3Endpoint             string
	CustomEndpointResolver s3.EndpointResolverV2

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

	s3CfgOptions := []func(o *s3.Options){
		func(o *s3.Options) {
			o.Region = cfg.Region
			o.UsePathStyle = cfg.S3ForcePathStyle
		},
	}

	if cfg.S3Endpoint != "" {
		s3CfgOptions = append(s3CfgOptions,
			func(o *s3.Options) {
				o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			},
		)
	}

	if cfg.CustomEndpointResolver != nil {
		s3CfgOptions = append(s3CfgOptions,
			s3.WithEndpointResolverV2(cfg.CustomEndpointResolver),
		)
	}

	// use role ARN to fetch credentials
	if cfg.RoleARN != "" {
		awsCfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			panic(err)
		}

		provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(awsCfg), cfg.RoleARN)
		creds := aws.NewCredentialsCache(provider)

		s3CfgOptions = append(s3CfgOptions, func(o *s3.Options) {
			o.Credentials = creds
		})

		s3Client := s3.NewFromConfig(awsCfg, s3CfgOptions...)
		client.S3 = s3Client
		return client
	}

	// use alreay configured aws config
	if cfg.AwsConfig != nil {
		s3Client := s3.NewFromConfig(*cfg.AwsConfig, s3CfgOptions...)
		client.S3 = s3Client
		return client
	}

	aswCfgOptions := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// use EC2 IAM role
	if cfg.EnableEC2IAMRole {
		aswCfgOptions = append(aswCfgOptions, config.WithCredentialsProvider(
			ec2rolecreds.New(),
		))
	}

	// use static credentials
	if cfg.AccessID != "" && cfg.AccessKey != "" {
		aswCfgOptions = append(aswCfgOptions, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessID, cfg.AccessKey, cfg.SessionToken),
		))
	}

	awsConfig, err := config.LoadDefaultConfig(context.TODO(), aswCfgOptions...)
	if err != nil {
		panic(err)
	}

	s3Client := s3.NewFromConfig(awsConfig, s3CfgOptions...)
	client.S3 = s3Client
	return client
}

// Get receive file with given path
func (client Client) Get(path string) (file *os.File, err error) {
	// already retried in GetStream
	readCloser, err := client.GetStream(path)

	ext := filepath.Ext(path)
	pattern := fmt.Sprintf("s3*%s", ext)

	if err == nil {
		// can't "defer file.Close()" here, because it will be used after client.Get
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
	var body io.ReadCloser
	err := Retry(3, time.Second, func() error {
		var retryErr error
		getResponse, retryErr := client.S3.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(client.Config.Bucket),
			Key:    aws.String(client.ToS3Key(path)),
		})

		if retryErr != nil {
			return retryErr
		}

		body = getResponse.Body
		return nil
	})

	return body, err
}

// Put store a reader into given path
func (client Client) Put(urlPath string, reader io.Reader) (*oss.Object, error) {
	if seeker, ok := reader.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}

	buffer, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	fileType := mime.TypeByExtension(path.Ext(urlPath))
	if fileType == "" {
		fileType = http.DetectContentType(buffer)
	}

	key := client.ToS3Key(urlPath)
	params := &s3.PutObjectInput{
		Bucket:        aws.String(client.Config.Bucket), // required
		Key:           aws.String(key),                  // required
		ACL:           client.Config.ACL,
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(int64(len(buffer))),
		ContentType:   aws.String(fileType),
	}

	if client.Config.CacheControl != "" {
		params.CacheControl = aws.String(client.Config.CacheControl)
	}

	err = Retry(3, time.Second, func() error {
		var retryErr error
		_, retryErr = client.S3.PutObject(context.Background(), params)
		return retryErr
	})

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
	return Retry(3, time.Second, func() error {
		var retryErr error
		_, retryErr = client.S3.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(client.Config.Bucket),
			Key:    aws.String(client.ToS3Key(path)),
		})
		return retryErr
	})
}

// DeleteObjects delete files in bulk
func (client Client) DeleteObjects(paths []string) error {
	var objs []types.ObjectIdentifier
	for _, v := range paths {
		var obj types.ObjectIdentifier
		obj.Key = aws.String(strings.TrimPrefix(client.ToS3Key(v), "/"))
		objs = append(objs, obj)
	}
	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(client.Config.Bucket),
		Delete: &types.Delete{
			Objects: objs,
		},
	}

	return Retry(3, time.Second, func() error {
		var retryErr error
		_, retryErr = client.S3.DeleteObjects(context.Background(), input)
		return retryErr
	})
}

// List list all objects under current path
func (client Client) List(path string) ([]*oss.Object, error) {
	var objects []*oss.Object
	var prefix string
	var continuationToken *string

	if path != "" {
		prefix = strings.Trim(path, "/") + "/"
	}

	for {
		var listObjectsResponse *s3.ListObjectsV2Output
		err := Retry(3, time.Second, func() error {
			var retryErr error
			listObjectsResponse, retryErr = client.S3.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
				Bucket:            aws.String(client.Config.Bucket),
				Prefix:            aws.String(prefix),
				ContinuationToken: continuationToken,
			})
			return retryErr
		})

		if err != nil {
			return nil, err
		}

		for _, content := range listObjectsResponse.Contents {
			objects = append(objects, &oss.Object{
				Path:             "/" + client.ToS3Key(*content.Key),
				Name:             filepath.Base(*content.Key),
				LastModified:     content.LastModified,
				StorageInterface: client,
			})
		}

		if listObjectsResponse.IsTruncated != nil && *listObjectsResponse.IsTruncated {
			continuationToken = listObjectsResponse.NextContinuationToken
		} else {
			break
		}
	}

	return objects, nil
}

// GetEndpoint get endpoint, FileSystem's endpoint is /
func (client Client) GetEndpoint() string {
	if client.Config.Endpoint != "" {
		return client.Config.Endpoint
	}

	if re, err := client.S3.Options().EndpointResolverV2.ResolveEndpoint(context.Background(), s3.EndpointParameters{
		Region: aws.String(client.Config.Region),
	}); err == nil {
		endpoint := re.URI.String()
		for _, prefix := range []string{"https://", "http://"} {
			endpoint = strings.TrimPrefix(endpoint, prefix)
		}
		return client.Config.Bucket + "." + endpoint
	}

	return fmt.Sprintf("%s.s3.%s.amazonaws.com", client.Config.Bucket, client.Config.Region)
}

var urlRegexp = regexp.MustCompile(`(https?:)?//((\w+).)+(\w+)/`)

// ToS3Key convert URL path to S3 key
func (client Client) ToS3Key(urlPath string) string {
	if urlRegexp.MatchString(urlPath) {
		if u, err := url.Parse(urlPath); err == nil {
			if client.Config.S3ForcePathStyle { // First part of path will be bucket name
				return strings.TrimPrefix(u.Path, "/"+client.Config.Bucket)
			}
			return strings.TrimPrefix(u.Path, "/")
		}
	}

	if client.Config.S3ForcePathStyle { // First part of path will be bucket name
		return strings.TrimPrefix(urlPath, "/"+client.Config.Bucket+"/")
	}
	return strings.TrimPrefix(urlPath, "/")
}

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
				Key:    aws.String(client.ToS3Key(path)),
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
func (client Client) Copy(from, to string) error {
	return Retry(3, time.Second, func() error {
		var retryErr error
		_, retryErr = client.S3.CopyObject(context.Background(), &s3.CopyObjectInput{
			Bucket:     aws.String(client.Config.Bucket),
			CopySource: aws.String(from),
			Key:        aws.String(to),
		})
		return retryErr
	})
}
