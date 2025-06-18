package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sirupsen/logrus"

	"easy-backup/internal/config"
	"easy-backup/internal/logger"
)

// S3Service handles S3 storage operations
type S3Service struct {
	config   *config.Config
	logger   *logrus.Logger
	session  *session.Session
	uploader *s3manager.Uploader
	s3Client *s3.S3
}

// NewS3Service creates a new S3 service
func NewS3Service(cfg *config.Config) (*S3Service, error) {
	// Create AWS config
	awsConfig := &aws.Config{
		Region: aws.String(cfg.Global.S3.Credentials.Region),
		Credentials: credentials.NewStaticCredentials(
			cfg.Global.S3.Credentials.AccessKey,
			cfg.Global.S3.Credentials.SecretKey,
			"",
		),
	}

	// Set custom endpoint if provided (for MinIO compatibility)
	if cfg.Global.S3.Endpoint != "" {
		awsConfig.Endpoint = aws.String(cfg.Global.S3.Endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(true) // Required for MinIO
	}

	// Create AWS session
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3Service{
		config:   cfg,
		logger:   logger.GetLogger(),
		session:  sess,
		uploader: s3manager.NewUploader(sess),
		s3Client: s3.New(sess),
	}, nil
}

// UploadBackup uploads a backup file to S3
func (s3s *S3Service) UploadBackup(ctx context.Context, strategy string, localPath string) (string, error) {
	// Parse timeout
	timeout, err := config.ParseDuration(s3s.config.Global.Timeout.Upload)
	if err != nil {
		return "", fmt.Errorf("invalid upload timeout: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Open the file
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Generate S3 key
	filename := filepath.Base(localPath)
	s3Key := filepath.Join(s3s.config.Global.S3.BasePath, strategy, time.Now().Format("2006/01/02"), filename)

	s3s.logger.WithFields(logrus.Fields{
		"strategy": strategy,
		"bucket":   s3s.config.Global.S3.Bucket,
		"key":      s3Key,
		"file":     localPath,
	}).Info("Starting S3 upload")

	// Upload to S3
	result, err := s3s.uploader.UploadWithContext(timeoutCtx, &s3manager.UploadInput{
		Bucket: aws.String(s3s.config.Global.S3.Bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	s3s.logger.WithFields(logrus.Fields{
		"strategy": strategy,
		"location": result.Location,
	}).Info("S3 upload completed successfully")

	return result.Location, nil
}

// TestConnection tests the S3 connection
func (s3s *S3Service) TestConnection(ctx context.Context) error {
	// Try to list objects in the bucket (limit to 1)
	_, err := s3s.s3Client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s3s.config.Global.S3.Bucket),
		MaxKeys: aws.Int64(1),
	})
	if err != nil {
		return fmt.Errorf("S3 connection test failed: %w", err)
	}

	return nil
}

// CleanupOldBackups removes old backups based on retention policy
func (s3s *S3Service) CleanupOldBackups(ctx context.Context, strategy string, retention string) error {
	// Parse retention duration
	retentionDuration, err := config.ParseDuration(retention)
	if err != nil {
		return fmt.Errorf("invalid retention duration: %w", err)
	}

	cutoffTime := time.Now().Add(-retentionDuration)
	prefix := filepath.Join(s3s.config.Global.S3.BasePath, strategy) + "/"

	s3s.logger.WithFields(logrus.Fields{
		"strategy": strategy,
		"cutoff":   cutoffTime,
		"prefix":   prefix,
	}).Info("Starting cleanup of old backups")

	// List objects with the strategy prefix
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s3s.config.Global.S3.Bucket),
		Prefix: aws.String(prefix),
	}

	var objectsToDelete []*s3.ObjectIdentifier
	err = s3s.s3Client.ListObjectsV2PagesWithContext(ctx, listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			if obj.LastModified.Before(cutoffTime) {
				objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
					Key: obj.Key,
				})
			}
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("failed to list S3 objects: %w", err)
	}

	// Delete old objects
	if len(objectsToDelete) > 0 {
		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(s3s.config.Global.S3.Bucket),
			Delete: &s3.Delete{
				Objects: objectsToDelete,
				Quiet:   aws.Bool(true),
			},
		}

		_, err = s3s.s3Client.DeleteObjectsWithContext(ctx, deleteInput)
		if err != nil {
			return fmt.Errorf("failed to delete old backups: %w", err)
		}

		s3s.logger.WithFields(logrus.Fields{
			"strategy": strategy,
			"count":    len(objectsToDelete),
		}).Info("Cleaned up old backups")
	} else {
		s3s.logger.WithField("strategy", strategy).Info("No old backups to clean up")
	}

	return nil
}
