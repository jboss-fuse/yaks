package publisher

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/container-tools/snap/pkg/util/log"
	minio "github.com/minio/minio-go"
	"github.com/pkg/errors"
)

type PublishDestination struct {
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	useSSL          bool
}

var (
	logger = log.WithName("publisher")
)

func NewPublishDestination(endpoint, accessKeyID, accessKeySecret string, useSSL bool) PublishDestination {
	return PublishDestination{
		endpoint:        endpoint,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		useSSL:          useSSL,
	}
}

type Publisher struct {
}

func NewPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) Publish(dir, dest string, options PublishDestination) error {
	logger.Info("Deploying artifacts to server...")
	// Initialize the minio client
	client, err := minio.New(options.endpoint, options.accessKeyID, options.accessKeySecret, options.useSSL)
	if err != nil {
		errors.Wrap(err, "cannot connect to minio server using provided credentials")
	}

	parts := strings.FieldsFunc(dest, func(c rune) bool { return c == '/' })
	if len(parts) < 1 {
		return errors.New("invalid destination: " + dest)
	}

	bucket := parts[0]
	if err := p.getOrCreateBucket(client, bucket); err != nil {
		return err
	}

	bucketPath := ""
	if len(parts) > 1 {
		bucketPath = strings.Join(parts[1:], "/")
	}

	err = filepath.Walk(dir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		var relative string
		relative, err = filepath.Rel(dir, filePath)
		if err != nil {
			return err
		}
		relative = path.Join(bucketPath, relative)
		if err := p.publish(client, bucket, filePath, relative); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	logger.Info("Deployment complete")
	return nil
}

func (p *Publisher) publish(client *minio.Client, bucket, sourceFile, destinationFile string) error {
	_, err := client.FPutObject(bucket, destinationFile, sourceFile, minio.PutObjectOptions{})
	return err
}

func (p *Publisher) getOrCreateBucket(client *minio.Client, bucket string) error {
	if exists, err := client.BucketExists(bucket); err != nil {
		return err
	} else if !exists {
		if err := client.MakeBucket(bucket, ""); err != nil {
			return err
		}
	}
	return nil
}
