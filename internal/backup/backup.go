package backup

import (
	"bufio"
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

	// Generate backup filename with simplified format
	backupPath := bs.generateBackupPath(strategy, startTime)

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

	// Execute database-specific backup command
	if err := bs.executeBackup(timeoutCtx, strategy, backupPath, result); err != nil {
		result.Error = err
		result.Success = false
		return result, result.Error
	}

	// Compress if enabled (skip for MongoDB as it's already compressed)
	if bs.config.Global.S3.Compression == "gzip" && strategy.DatabaseType != "mongodb" {
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

	// For MongoDB, the backup path might have been changed to .tar.gz
	if strategy.DatabaseType == "mongodb" && !strings.HasSuffix(backupPath, ".tar.gz") {
		backupPath = backupPath + ".tar.gz"
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

// ExecuteBackupWithProgress performs a backup for a specific strategy with progress callbacks
func (bs *BackupService) ExecuteBackupWithProgress(ctx context.Context, strategy config.StrategyConfig, progressCallback ProgressCallback) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{
		Strategy:    strategy.Name,
		StartTime:   startTime,
		CommandLogs: make([]string, 0),
	}

	bs.logger.WithField("strategy", strategy.Name).Info("Starting backup")

	// Send initial progress
	if progressCallback != nil {
		progressCallback(strategy.Name, "Starting database backup...")
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(bs.config.Global.TempDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create temp directory: %w", err)
		result.Success = false
		return result, result.Error
	}

	// Generate backup filename with simplified format
	backupPath := bs.generateBackupPath(strategy, startTime)

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

	// Send progress update
	if progressCallback != nil {
		progressCallback(strategy.Name, fmt.Sprintf("Executing %s backup command...", strategy.DatabaseType))
	}

	// Execute database-specific backup command
	if err := bs.executeBackupWithProgress(timeoutCtx, strategy, backupPath, result, progressCallback); err != nil {
		result.Error = err
		result.Success = false
		return result, result.Error
	}

	// Send progress update
	if progressCallback != nil {
		progressCallback(strategy.Name, "Database backup completed, processing file...")
	}

	// Compress if enabled (skip for MongoDB as it's already compressed)
	if bs.config.Global.S3.Compression == "gzip" && strategy.DatabaseType != "mongodb" {
		if progressCallback != nil {
			progressCallback(strategy.Name, "Compressing backup file...")
		}
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

	// For MongoDB, the backup path might have been changed to .tar.gz
	if strategy.DatabaseType == "mongodb" && !strings.HasSuffix(backupPath, ".tar.gz") {
		backupPath = backupPath + ".tar.gz"
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

	// Send final progress update
	if progressCallback != nil {
		progressCallback(strategy.Name, fmt.Sprintf("Backup completed successfully (%s)", formatBytes(result.Size)))
	}

	bs.logger.WithFields(logrus.Fields{
		"strategy": strategy.Name,
		"size":     result.Size,
		"duration": result.Duration,
		"path":     backupPath,
	}).Info("Backup completed successfully")

	return result, nil
}

// executeBackup executes the appropriate backup command based on database type
func (bs *BackupService) executeBackup(ctx context.Context, strategy config.StrategyConfig, outputPath string, result *BackupResult) error {
	switch strategy.DatabaseType {
	case "postgres":
		return bs.executePostgresBackup(ctx, strategy.DatabaseURL, outputPath, result)
	case "mysql", "mariadb":
		return bs.executeMySQLBackup(ctx, strategy.DatabaseURL, outputPath, result)
	case "mongodb":
		return bs.executeMongoBackup(ctx, strategy.DatabaseURL, outputPath, result)
	default:
		return fmt.Errorf("unsupported database type: %s", strategy.DatabaseType)
	}
}

// executeBackupWithProgress executes the appropriate backup command based on database type with progress callbacks
func (bs *BackupService) executeBackupWithProgress(ctx context.Context, strategy config.StrategyConfig, outputPath string, result *BackupResult, progressCallback ProgressCallback) error {
	switch strategy.DatabaseType {
	case "postgres":
		return bs.executePostgresBackupWithProgress(ctx, strategy.DatabaseURL, outputPath, result, progressCallback)
	case "mysql", "mariadb":
		return bs.executeMySQLBackupWithProgress(ctx, strategy.DatabaseURL, outputPath, result, progressCallback)
	case "mongodb":
		return bs.executeMongoBackupWithProgress(ctx, strategy.DatabaseURL, outputPath, result, progressCallback)
	default:
		return fmt.Errorf("unsupported database type: %s", strategy.DatabaseType)
	}
}

// executePostgresBackup executes the pg_dump command (renamed from executePgDump)
func (bs *BackupService) executePostgresBackup(ctx context.Context, databaseURL, outputPath string, result *BackupResult) error {
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

// executePostgresBackupWithProgress executes the pg_dump command with progress callbacks
func (bs *BackupService) executePostgresBackupWithProgress(ctx context.Context, databaseURL, outputPath string, result *BackupResult, progressCallback ProgressCallback) error {
	if progressCallback != nil {
		progressCallback(result.Strategy, "Starting PostgreSQL dump...")
	}

	args := []string{
		databaseURL,
		"--no-password",
		"--verbose",
		"--format=custom",
		"--file=" + outputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

	// Create pipes to capture real-time output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}

	// Log the command execution
	commandLog := fmt.Sprintf("Command: pg_dump %s", strings.Join(args, " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	// Capture output in real-time
	go bs.captureOutput(stdout, "stdout", result, progressCallback)
	go bs.captureOutput(stderr, "stderr", result, progressCallback)

	// Wait for command to complete
	err = cmd.Wait()

	if err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	if progressCallback != nil {
		progressCallback(result.Strategy, "PostgreSQL dump completed successfully")
	}

	return nil
}

// executeMySQLBackup executes the mysqldump command
func (bs *BackupService) executeMySQLBackup(ctx context.Context, databaseURL, outputPath string, result *BackupResult) error {
	// Parse MySQL connection string: mysql://user:password@host:port/database
	// Remove the mysql:// prefix
	connStr := strings.TrimPrefix(databaseURL, "mysql://")

	// Parse user:password@host:port/database
	var host, port, user, password, database string

	if atIndex := strings.Index(connStr, "@"); atIndex != -1 {
		userPass := connStr[:atIndex]
		hostPortDB := connStr[atIndex+1:]

		// Parse user:password
		if colonIndex := strings.Index(userPass, ":"); colonIndex != -1 {
			user = userPass[:colonIndex]
			password = userPass[colonIndex+1:]
		} else {
			user = userPass
		}

		// Parse host:port/database
		if slashIndex := strings.Index(hostPortDB, "/"); slashIndex != -1 {
			hostPort := hostPortDB[:slashIndex]
			database = hostPortDB[slashIndex+1:]

			// Parse host:port
			if colonIndex := strings.Index(hostPort, ":"); colonIndex != -1 {
				host = hostPort[:colonIndex]
				port = hostPort[colonIndex+1:]
			} else {
				host = hostPort
				port = "3306" // default port
			}
		} else {
			host = hostPortDB
			port = "3306"
		}
	} else {
		return fmt.Errorf("invalid MySQL connection URL format: %s", databaseURL)
	}

	if host == "" || user == "" || database == "" {
		return fmt.Errorf("missing required connection parameters in URL: %s", databaseURL)
	}

	args := []string{
		"--host=" + host,
		"--port=" + port,
		"--user=" + user,
		"--password=" + password,
		"--skip-ssl",
		"--single-transaction",
		"--routines",
		"--triggers",
		"--verbose",
		"--result-file=" + outputPath,
		database,
	}

	cmd := exec.CommandContext(ctx, "mariadb-dump", args...)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Log the command execution (hide password)
	safeArgs := make([]string, len(args))
	copy(safeArgs, args)
	for i, arg := range safeArgs {
		if strings.HasPrefix(arg, "--password=") {
			safeArgs[i] = "--password=***"
		}
	}
	commandLog := fmt.Sprintf("Command: mariadb-dump %s", strings.Join(safeArgs, " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	if len(output) > 0 {
		outputLog := fmt.Sprintf("Output: %s", string(output))
		result.CommandLogs = append(result.CommandLogs, outputLog)
		bs.logger.WithField("strategy", result.Strategy).Info(outputLog)
	}

	if err != nil {
		return fmt.Errorf("mariadb-dump failed: %w", err)
	}

	return nil
}

// executeMySQLBackupWithProgress executes the mariadb-dump command with progress callbacks
func (bs *BackupService) executeMySQLBackupWithProgress(ctx context.Context, databaseURL, outputPath string, result *BackupResult, progressCallback ProgressCallback) error {
	// Parse MySQL connection string: mysql://user:password@host:port/database
	// Remove the mysql:// prefix
	connStr := strings.TrimPrefix(databaseURL, "mysql://")

	// Parse user:password@host:port/database
	var host, port, user, password, database string

	if atIndex := strings.Index(connStr, "@"); atIndex != -1 {
		userPass := connStr[:atIndex]
		hostPortDB := connStr[atIndex+1:]

		// Parse user:password
		if colonIndex := strings.Index(userPass, ":"); colonIndex != -1 {
			user = userPass[:colonIndex]
			password = userPass[colonIndex+1:]
		} else {
			user = userPass
		}

		// Parse host:port/database
		if slashIndex := strings.Index(hostPortDB, "/"); slashIndex != -1 {
			hostPort := hostPortDB[:slashIndex]
			database = hostPortDB[slashIndex+1:]

			// Parse host:port
			if colonIndex := strings.Index(hostPort, ":"); colonIndex != -1 {
				host = hostPort[:colonIndex]
				port = hostPort[colonIndex+1:]
			} else {
				host = hostPort
				port = "3306" // default port
			}
		} else {
			host = hostPortDB
			port = "3306"
		}
	} else {
		return fmt.Errorf("invalid MySQL connection URL format: %s", databaseURL)
	}

	if host == "" || user == "" || database == "" {
		return fmt.Errorf("missing required connection parameters in URL: %s", databaseURL)
	}

	if progressCallback != nil {
		progressCallback(result.Strategy, fmt.Sprintf("Connecting to MySQL database %s@%s:%s", user, host, port))
	}

	args := []string{
		"--host=" + host,
		"--port=" + port,
		"--user=" + user,
		"--password=" + password,
		"--skip-ssl",
		"--single-transaction",
		"--routines",
		"--triggers",
		"--verbose",
		"--result-file=" + outputPath,
		database,
	}

	cmd := exec.CommandContext(ctx, "mariadb-dump", args...)

	// Create pipes to capture real-time output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mariadb-dump: %w", err)
	}

	// Log the command execution (hide password)
	safeArgs := make([]string, len(args))
	copy(safeArgs, args)
	for i, arg := range safeArgs {
		if strings.HasPrefix(arg, "--password=") {
			safeArgs[i] = "--password=***"
		}
	}
	commandLog := fmt.Sprintf("Command: mariadb-dump %s", strings.Join(safeArgs, " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	// Capture output in real-time
	go bs.captureOutput(stdout, "stdout", result, progressCallback)
	go bs.captureOutput(stderr, "stderr", result, progressCallback)

	// Wait for command to complete
	err = cmd.Wait()

	if err != nil {
		return fmt.Errorf("mariadb-dump failed: %w", err)
	}

	if progressCallback != nil {
		progressCallback(result.Strategy, "MySQL dump completed successfully")
	}

	return nil
}

// captureOutput captures command output in real-time and sends it via progress callback
func (bs *BackupService) captureOutput(pipe io.ReadCloser, streamType string, result *BackupResult, progressCallback ProgressCallback) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	var outputBuffer strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		outputBuffer.WriteString(line)
		outputBuffer.WriteString("\n")

		// Only send error/warning lines to Slack in real-time
		if progressCallback != nil {
			lineLower := strings.ToLower(line)
			// Check if the line contains error/warning indicators
			if strings.Contains(lineLower, "error") ||
				strings.Contains(lineLower, "failed") ||
				strings.Contains(lineLower, "failure") ||
				strings.Contains(lineLower, "warning") ||
				strings.Contains(lineLower, "warn") ||
				strings.Contains(lineLower, "fatal") ||
				strings.Contains(lineLower, "critical") {
				progressCallback(result.Strategy, fmt.Sprintf("[%s] %s", streamType, line))
			}
		}
	}

	// Store the complete output in command logs
	if outputBuffer.Len() > 0 {
		outputLog := fmt.Sprintf("Output (%s): %s", streamType, outputBuffer.String())
		result.CommandLogs = append(result.CommandLogs, outputLog)
	}
}

// executeMongoBackup executes the mongodump command
func (bs *BackupService) executeMongoBackup(ctx context.Context, databaseURL, outputPath string, result *BackupResult) error {
	args := []string{
		"--uri=" + databaseURL,
		"--out=" + outputPath,
		"--verbose",
	}

	cmd := exec.CommandContext(ctx, "mongodump", args...)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Log the command execution (hide password in URI)
	safeArgs := make([]string, len(args))
	copy(safeArgs, args)
	for i, arg := range safeArgs {
		if strings.HasPrefix(arg, "--uri=") {
			// Hide password in URI
			uri := arg[6:] // Remove --uri= prefix
			if strings.Contains(uri, "@") && strings.Contains(uri, ":") {
				// mongodb://user:password@host:port/db -> mongodb://user:***@host:port/db
				parts := strings.Split(uri, "@")
				if len(parts) >= 2 {
					userPass := parts[0]
					if colonIndex := strings.Index(userPass, ":"); colonIndex != -1 {
						userPart := userPass[:colonIndex]
						safeUri := userPart + ":***@" + strings.Join(parts[1:], "@")
						safeArgs[i] = "--uri=" + safeUri
					}
				}
			}
		}
	}
	commandLog := fmt.Sprintf("Command: mongodump %s", strings.Join(safeArgs, " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	if len(output) > 0 {
		outputLog := fmt.Sprintf("Output: %s", string(output))
		result.CommandLogs = append(result.CommandLogs, outputLog)
		bs.logger.WithField("strategy", result.Strategy).Info(outputLog)
	}

	if err != nil {
		return fmt.Errorf("mongodump failed: %w", err)
	}

	// For MongoDB, we need to create a tar archive of the dump directory
	return bs.createMongoArchive(outputPath)
}

// executeMongoBackupWithProgress executes the mongodump command with progress callbacks
func (bs *BackupService) executeMongoBackupWithProgress(ctx context.Context, databaseURL, outputPath string, result *BackupResult, progressCallback ProgressCallback) error {
	if progressCallback != nil {
		progressCallback(result.Strategy, "Starting MongoDB dump...")
	}

	args := []string{
		"--uri=" + databaseURL,
		"--out=" + outputPath,
		"--verbose",
	}

	cmd := exec.CommandContext(ctx, "mongodump", args...)

	// Create pipes to capture real-time output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mongodump: %w", err)
	}

	// Log the command execution (hide password in URI)
	safeArgs := make([]string, len(args))
	copy(safeArgs, args)
	for i, arg := range safeArgs {
		if strings.HasPrefix(arg, "--uri=") {
			// Hide password in URI
			uri := arg[6:] // Remove --uri= prefix
			if strings.Contains(uri, "@") && strings.Contains(uri, ":") {
				// mongodb://user:password@host:port/db -> mongodb://user:***@host:port/db
				parts := strings.Split(uri, "@")
				if len(parts) >= 2 {
					userPass := parts[0]
					if colonIndex := strings.Index(userPass, ":"); colonIndex != -1 {
						userPart := userPass[:colonIndex]
						safeUri := userPart + ":***@" + strings.Join(parts[1:], "@")
						safeArgs[i] = "--uri=" + safeUri
					}
				}
			}
		}
	}
	commandLog := fmt.Sprintf("Command: mongodump %s", strings.Join(safeArgs, " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	// Capture output in real-time
	go bs.captureOutput(stdout, "stdout", result, progressCallback)
	go bs.captureOutput(stderr, "stderr", result, progressCallback)

	// Wait for command to complete
	err = cmd.Wait()

	if err != nil {
		return fmt.Errorf("mongodump failed: %w", err)
	}

	if progressCallback != nil {
		progressCallback(result.Strategy, "MongoDB dump completed, creating archive...")
	}

	// For MongoDB, we need to create a tar archive of the dump directory
	return bs.createMongoArchive(outputPath)
}

// createMongoArchive creates a tar archive from the mongodump output directory
func (bs *BackupService) createMongoArchive(outputPath string) error {
	// Create tar.gz archive from the mongodump directory
	archivePath := outputPath + ".tar.gz"

	cmd := exec.Command("tar", "-czf", archivePath, "-C", filepath.Dir(outputPath), filepath.Base(outputPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		bs.logger.WithError(err).WithField("output", string(output)).Error("Failed to create MongoDB archive")
		return fmt.Errorf("failed to create MongoDB archive: %w", err)
	}

	// Remove the original directory
	if err := os.RemoveAll(outputPath); err != nil {
		bs.logger.WithError(err).Warn("Failed to remove MongoDB dump directory")
	}

	bs.logger.WithField("archive", archivePath).Info("Created MongoDB archive")
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

// formatBytes formats bytes into human readable format
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

// generateBackupPath creates a simplified backup file path
func (bs *BackupService) generateBackupPath(strategy config.StrategyConfig, timestamp time.Time) string {
	// Simplified timestamp format: YYYYMMDD-HHMMSS
	timeStr := timestamp.Format("20060102-150405")

	// Simplified filename: strategy-YYYYMMDD-HHMMSS.ext
	var filename string
	switch strategy.DatabaseType {
	case "mongodb":
		// MongoDB creates a directory, no extension needed
		filename = fmt.Sprintf("%s-%s", strategy.Name, timeStr)
	default:
		// All SQL databases use .sql extension
		filename = fmt.Sprintf("%s-%s.sql", strategy.Name, timeStr)
	}

	return filepath.Join(bs.config.Global.TempDir, filename)
}
