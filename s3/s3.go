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
	smithyendpoints "github.com/aws/smithy-go/endpoints"
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

	s3CfgOptions := []func(o *s3.Options){
		func(o *s3.Options) {
			o.Region = cfg.Region
			o.UsePathStyle = cfg.S3ForcePathStyle
		},
	}

	if cfg.S3Endpoint != "" {
		s3CfgOptions = append(s3CfgOptions, s3.WithEndpointResolverV2(&endpointResolverV2{
			Url: cfg.S3Endpoint,
		}))

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
		Key:    aws.String(client.ToS3Key(path)),
	})

	if err != nil {
		return nil, err
	}

	return getResponse.Body, err
}

// Put store a reader into given path
func (client Client) Put(urlPath string, reader io.Reader) (*oss.Object, error) {
	if seeker, ok := reader.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}

	key := client.ToS3Key(urlPath)
	buffer, err := io.ReadAll(reader)

	fileType := mime.TypeByExtension(path.Ext(urlPath))
	if fileType == "" {
		fileType = http.DetectContentType(buffer)
	}

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
		Key:    aws.String(client.ToS3Key(path)),
	})
	return err
}

// DeleteObjects delete files in bulk
func (client Client) DeleteObjects(paths []string) (err error) {
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
				Path:             "/" + client.ToS3Key(*content.Key),
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

	if client.Config.S3Endpoint != "" {
		return client.Config.S3Endpoint
	}

	if client.Config.S3ForcePathStyle {
		return fmt.Sprintf("s3.%s.amazonaws.com/%s", client.Config.Region, client.Config.Bucket)
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
func (client Client) Copy(from, to string) (err error) {
	_, err = client.S3.CopyObject(context.Background(), &s3.CopyObjectInput{
		Bucket:     aws.String(client.Config.Bucket),
		CopySource: aws.String(from),
		Key:        aws.String(to),
	})
	return
}

type endpointResolverV2 struct {
	Url string
}

func (r *endpointResolverV2) ResolveEndpoint(
	ctx context.Context, params s3.EndpointParameters,
) (
	endpoint smithyendpoints.Endpoint, err error,
) {

	u, err := url.Parse(r.Url)
	if err != nil {
		return smithyendpoints.Endpoint{}, err
	}
	return smithyendpoints.Endpoint{
		URI: *u,
	}, nil
}
