package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"easy-backup/internal/config"
	"easy-backup/internal/logger"
)

// BackupResult represents the result of a backup operation
type BackupResult struct {
	Strategy    string
	Success     bool
	Error       error
	BackupPath  string
	Size        int64
	Duration    time.Duration
	StartTime   time.Time
	EndTime     time.Time
	CommandLogs []string
}

// BackupService handles database backup operations
type BackupService struct {
	config *config.Config
	logger *logrus.Logger
}

// NewBackupService creates a new backup service
func NewBackupService(cfg *config.Config) *BackupService {
	return &BackupService{
		config: cfg,
		logger: logger.GetLogger(),
	}
}

// ExecuteBackup performs a backup for a specific strategy
func (bs *BackupService) ExecuteBackup(ctx context.Context, strategy config.StrategyConfig) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{
		Strategy:    strategy.Name,
		StartTime:   startTime,
		CommandLogs: make([]string, 0),
	}

	bs.logger.WithField("strategy", strategy.Name).Info("Starting backup")

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(bs.config.Global.TempDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create temp directory: %w", err)
		result.Success = false
		return result, result.Error
	}

	// Generate backup filename
	timestamp := startTime.Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.sql", strategy.Name, timestamp)
	backupPath := filepath.Join(bs.config.Global.TempDir, filename)

	// Parse timeout
	timeout, err := config.ParseDuration(bs.config.Global.Timeout.Backup)
	if err != nil {
		result.Error = fmt.Errorf("invalid backup timeout: %w", err)
		result.Success = false
		return result, result.Error
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute pg_dump
	if err := bs.executePgDump(timeoutCtx, strategy.DatabaseURL, backupPath, result); err != nil {
		result.Error = err
		result.Success = false
		return result, result.Error
	}

	// Compress if enabled
	if bs.config.Global.S3.Compression == "gzip" {
		compressedPath := backupPath + ".gz"
		if err := bs.compressFile(backupPath, compressedPath); err != nil {
			result.Error = fmt.Errorf("failed to compress backup: %w", err)
			result.Success = false
			return result, result.Error
		}
		// Remove original uncompressed file
		os.Remove(backupPath)
		backupPath = compressedPath
	}

	// Get file size
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to get backup file info: %w", err)
		result.Success = false
		return result, result.Error
	}

	result.BackupPath = backupPath
	result.Size = fileInfo.Size()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = true

	bs.logger.WithFields(logrus.Fields{
		"strategy": strategy.Name,
		"size":     result.Size,
		"duration": result.Duration,
		"path":     backupPath,
	}).Info("Backup completed successfully")

	return result, nil
}

// executePgDump executes the pg_dump command
func (bs *BackupService) executePgDump(ctx context.Context, databaseURL, outputPath string, result *BackupResult) error {
	args := []string{
		databaseURL,
		"--no-password",
		"--verbose",
		"--format=custom",
		"--file=" + outputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Log the command execution
	commandLog := fmt.Sprintf("Command: pg_dump %s", strings.Join(args, " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	if len(output) > 0 {
		outputLog := fmt.Sprintf("Output: %s", string(output))
		result.CommandLogs = append(result.CommandLogs, outputLog)
		bs.logger.WithField("strategy", result.Strategy).Debug(outputLog)
	}

	if err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	return nil
}

// compressFile compresses a file using gzip
func (bs *BackupService) compressFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	gzipWriter := gzip.NewWriter(destFile)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to compress file: %w", err)
	}

	return nil
}

// CleanupTempFiles removes temporary backup files
func (bs *BackupService) CleanupTempFiles(backupPath string) error {
	if backupPath != "" {
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			bs.logger.WithError(err).WithField("path", backupPath).Warn("Failed to cleanup temp file")
			return err
		}
		bs.logger.WithField("path", backupPath).Debug("Cleaned up temp file")
	}
	return nil
}
