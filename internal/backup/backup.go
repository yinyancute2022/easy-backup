package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"easy-backup/internal/config"
	"easy-backup/internal/logger"
)

// ProgressCallback defines a function type for progress updates
type ProgressCallback func(strategy, message string)

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

// DatabaseStrategy interface defines the contract for database backup strategies
type DatabaseStrategy interface {
	Backup(ctx context.Context, databaseURL, outputPath string, callback ProgressCallback) (*BackupResult, error)
	ValidateConnection(databaseURL string) error
	GetType() string
}

// BackupService handles database backup operations using the Strategy pattern
type BackupService struct {
	config     *config.Config
	logger     *logrus.Logger
	strategies map[string]DatabaseStrategy
}

// NewBackupService creates a new backup service with all database strategies
func NewBackupService(cfg *config.Config) *BackupService {
	service := &BackupService{
		config:     cfg,
		logger:     logger.GetLogger(),
		strategies: make(map[string]DatabaseStrategy),
	}

	// Register all database strategies
	service.registerStrategies()
	return service
}

// registerStrategies registers all available database backup strategies
func (bs *BackupService) registerStrategies() {
	bs.strategies["postgres"] = NewPostgresStrategy(bs.logger)
	bs.strategies["mysql"] = NewMySQLStrategy(bs.logger)
	bs.strategies["mariadb"] = NewMySQLStrategy(bs.logger) // MySQL strategy handles MariaDB too
	bs.strategies["mongodb"] = NewMongoStrategy(bs.logger)
}

// ExecuteBackup performs a backup for a specific strategy
func (bs *BackupService) ExecuteBackup(ctx context.Context, strategy config.StrategyConfig) (*BackupResult, error) {
	return bs.executeBackup(ctx, strategy, nil)
}

// ExecuteBackupWithProgress performs a backup with progress callbacks
func (bs *BackupService) ExecuteBackupWithProgress(ctx context.Context, strategy config.StrategyConfig, progressCallback ProgressCallback) (*BackupResult, error) {
	return bs.executeBackup(ctx, strategy, progressCallback)
}

// executeBackup is the main backup execution logic
func (bs *BackupService) executeBackup(ctx context.Context, strategyConfig config.StrategyConfig, progressCallback ProgressCallback) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{
		Strategy:    strategyConfig.Name,
		StartTime:   startTime,
		CommandLogs: make([]string, 0),
	}

	// Send initial progress
	if progressCallback != nil {
		progressCallback(strategyConfig.Name, "Starting database backup...")
	}

	// Get the appropriate database strategy
	dbStrategy, exists := bs.strategies[strategyConfig.DatabaseType]
	if !exists {
		err := fmt.Errorf("unsupported database type: %s", strategyConfig.DatabaseType)
		result.Error = err
		result.Success = false
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Unsupported database type: %s", strategyConfig.DatabaseType))
		}
		return result, err
	}

	// Validate connection URL
	if err := dbStrategy.ValidateConnection(strategyConfig.DatabaseURL); err != nil {
		result.Error = err
		result.Success = false
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Invalid connection URL: %s", err.Error()))
		}
		return result, err
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(bs.config.Global.TempDir, 0755); err != nil {
		err = fmt.Errorf("failed to create temp directory: %w", err)
		result.Error = err
		result.Success = false
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Failed to create temp directory: %s", err.Error()))
		}
		return result, err
	}

	// Generate backup filename
	backupPath := bs.generateBackupPath(strategyConfig, startTime)

	// Parse timeout
	timeout, err := config.ParseDuration(bs.config.Global.Timeout.Backup)
	if err != nil {
		err = fmt.Errorf("invalid backup timeout: %w", err)
		result.Error = err
		result.Success = false
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Invalid backup timeout: %s", err.Error()))
		}
		return result, err
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute backup using the strategy
	if progressCallback != nil {
		progressCallback(strategyConfig.Name, fmt.Sprintf("Executing %s backup command...", strategyConfig.DatabaseType))
	}

	backupResult, err := dbStrategy.Backup(timeoutCtx, strategyConfig.DatabaseURL, backupPath, progressCallback)
	if err != nil {
		result.Error = err
		result.Success = false
		if backupResult != nil {
			result.CommandLogs = backupResult.CommandLogs
		}
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Backup failed: %s", err.Error()))
		}
		return result, err
	}

	// Copy result data
	if backupResult != nil {
		result.CommandLogs = backupResult.CommandLogs
		result.BackupPath = backupResult.BackupPath
	}

	// Handle compression
	if err := bs.handleCompression(strategyConfig, &backupPath, progressCallback); err != nil {
		result.Error = err
		result.Success = false
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Compression failed: %s", err.Error()))
		}
		return result, err
	}

	// Finalize result
	if err := bs.finalizeResult(result, backupPath, progressCallback); err != nil {
		result.Error = err
		result.Success = false
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, fmt.Sprintf("❌ Failed to finalize backup: %s", err.Error()))
		}
		return result, err
	}

	result.Success = true
	bs.logger.WithFields(logrus.Fields{
		"strategy": strategyConfig.Name,
		"size":     result.Size,
		"duration": result.Duration,
		"path":     backupPath,
	}).Info("Backup completed successfully")

	return result, nil
}

// handleCompression handles file compression if enabled
func (bs *BackupService) handleCompression(strategyConfig config.StrategyConfig, backupPath *string, progressCallback ProgressCallback) error {
	// Compress if enabled (skip for MongoDB as it's already compressed)
	if bs.config.Global.S3.Compression == "gzip" && strategyConfig.DatabaseType != "mongodb" {
		if progressCallback != nil {
			progressCallback(strategyConfig.Name, "Compressing backup file...")
		}
		compressedPath := *backupPath + ".gz"
		if err := bs.compressFile(*backupPath, compressedPath); err != nil {
			return fmt.Errorf("failed to compress backup: %w", err)
		}
		// Remove original uncompressed file
		os.Remove(*backupPath)
		*backupPath = compressedPath
	}

	// For MongoDB, the backup path might have been changed to .tar.gz
	if strategyConfig.DatabaseType == "mongodb" && !strings.HasSuffix(*backupPath, ".tar.gz") {
		*backupPath = *backupPath + ".tar.gz"
	}

	return nil
}

// finalizeResult finalizes the backup result with file information
func (bs *BackupService) finalizeResult(result *BackupResult, backupPath string, progressCallback ProgressCallback) error {
	// Get file size
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("failed to get backup file info: %w", err)
	}

	result.BackupPath = backupPath
	result.Size = fileInfo.Size()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Send final progress update
	if progressCallback != nil {
		progressCallback(result.Strategy, fmt.Sprintf("Backup completed successfully (%s)", formatBytes(result.Size)))
	}

	return nil
}

// generateBackupPath generates a backup file path
func (bs *BackupService) generateBackupPath(strategy config.StrategyConfig, startTime time.Time) string {
	// Generate backup filename with simplified format
	timestamp := startTime.Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s", strategy.Name, timestamp)

	// Add appropriate extension based on database type
	switch strategy.DatabaseType {
	case "postgres":
		filename += ".dump"
	case "mysql", "mariadb":
		filename += ".sql"
	case "mongodb":
		filename += ".archive"
	default:
		filename += ".backup"
	}

	return filepath.Join(bs.config.Global.TempDir, filename)
}

// CleanupTempFiles removes temporary backup files
func (bs *BackupService) CleanupTempFiles(filePath string) error {
	if filePath == "" {
		return nil
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove temporary file %s: %w", filePath, err)
	}

	bs.logger.WithField("file", filePath).Debug("Cleaned up temporary backup file")
	return nil
}

// compressFile compresses a file using gzip
func (bs *BackupService) compressFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	gzipWriter := gzip.NewWriter(dstFile)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, srcFile)
	if err != nil {
		return fmt.Errorf("failed to compress file: %w", err)
	}

	return nil
}

// formatBytes formats byte size to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
