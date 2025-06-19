package backup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// MongoStrategy implements DatabaseStrategy for MongoDB databases
type MongoStrategy struct {
	logger *logrus.Logger
}

// NewMongoStrategy creates a new MongoDB backup strategy
func NewMongoStrategy(logger *logrus.Logger) *MongoStrategy {
	return &MongoStrategy{logger: logger}
}

// GetType returns the database type
func (ms *MongoStrategy) GetType() string {
	return "mongodb"
}

// ValidateConnection validates the MongoDB connection
func (ms *MongoStrategy) ValidateConnection(databaseURL string) error {
	if !strings.HasPrefix(databaseURL, "mongodb://") && !strings.HasPrefix(databaseURL, "mongodb+srv://") {
		return fmt.Errorf("invalid MongoDB URL format")
	}
	return nil
}

// Backup performs a MongoDB backup using mongodump
func (ms *MongoStrategy) Backup(ctx context.Context, databaseURL, outputPath string, callback ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		CommandLogs: make([]string, 0),
	}

	if callback != nil {
		callback("mongodb", "Starting MongoDB backup...")
	}

	args := []string{
		"--uri=" + databaseURL,
		"--out=" + outputPath,
		"--verbose",
	}

	cmd := exec.CommandContext(ctx, "mongodump", args...)

	// Set up pipes for real-time output capture
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		commandLog := fmt.Sprintf("Command failed to start: mongodump %s - Error: %s", strings.Join(ms.sanitizeArgs(args), " "), err.Error())
		result.CommandLogs = append(result.CommandLogs, commandLog)
		if callback != nil {
			callback("mongodb", fmt.Sprintf("❌ Command failed to start: %s", err.Error()))
		}
		return result, fmt.Errorf("mongodump failed to start: %w", err)
	}

	// Log the command execution (with masked credentials)
	commandLog := fmt.Sprintf("Command: mongodump %s", strings.Join(ms.sanitizeArgs(args), " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	// Capture output in real-time
	go ms.captureOutput(stdout, "stdout", result, callback)
	go ms.captureOutput(stderr, "stderr", result, callback)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		errorLog := fmt.Sprintf("Command failed: mongodump %s - Error: %s", strings.Join(ms.sanitizeArgs(args), " "), err.Error())
		result.CommandLogs = append(result.CommandLogs, errorLog)
		if callback != nil {
			callback("mongodb", fmt.Sprintf("❌ MongoDB backup failed: %s", err.Error()))
		}
		return result, fmt.Errorf("mongodump failed: %w", err)
	}

	// For MongoDB, we need to create a tar.gz archive from the dump directory
	tarPath := outputPath + ".tar.gz"
	if err := ms.createTarArchive(outputPath, tarPath); err != nil {
		if callback != nil {
			callback("mongodb", fmt.Sprintf("❌ Failed to create archive: %s", err.Error()))
		}
		return result, fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Clean up the dump directory
	if err := os.RemoveAll(outputPath); err != nil {
		ms.logger.WithError(err).Warn("Failed to clean up MongoDB dump directory")
	}

	result.BackupPath = tarPath
	if callback != nil {
		callback("mongodb", "MongoDB backup completed successfully")
	}

	return result, nil
}

// createTarArchive creates a tar.gz archive from a directory
func (ms *MongoStrategy) createTarArchive(sourceDir, targetPath string) error {
	cmd := exec.Command("tar", "-czf", targetPath, "-C", sourceDir, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar command failed: %w, output: %s", err, string(output))
	}
	return nil
}

// captureOutput captures command output in real-time
func (ms *MongoStrategy) captureOutput(pipe io.ReadCloser, streamType string, result *BackupResult, callback ProgressCallback) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	var outputBuffer strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		outputBuffer.WriteString(line)
		outputBuffer.WriteString("\n")

		// Check for MongoDB error patterns
		if ms.containsError(line) && callback != nil {
			callback("mongodb", fmt.Sprintf("❌ MONGODB ERROR: %s", line))
		}

		// Send other relevant lines
		if callback != nil && ms.shouldReportLine(line) {
			callback("mongodb", fmt.Sprintf("[%s] %s", streamType, line))
		}
	}

	// Store the complete output in command logs
	if outputBuffer.Len() > 0 {
		outputLog := fmt.Sprintf("Output (%s): %s", streamType, outputBuffer.String())
		result.CommandLogs = append(result.CommandLogs, outputLog)
	}
}

// containsError checks if the output contains MongoDB error patterns
func (ms *MongoStrategy) containsError(output string) bool {
	outputLower := strings.ToLower(output)

	errorPatterns := []string{
		"error:",
		"failed to",
		"connection refused",
		"authentication failed",
		"database does not exist",
		"could not connect",
		"timeout",
		"access denied",
		"unauthorized",
		"too many connections",
	}

	for _, pattern := range errorPatterns {
		if strings.Contains(outputLower, pattern) {
			return true
		}
	}

	return false
}

// shouldReportLine determines if a line should be reported to the callback
func (ms *MongoStrategy) shouldReportLine(line string) bool {
	lineLower := strings.ToLower(line)
	return strings.Contains(lineLower, "error") ||
		strings.Contains(lineLower, "warning") ||
		strings.Contains(lineLower, "failed") ||
		strings.Contains(lineLower, "success")
}

// sanitizeArgs removes sensitive information from command arguments
func (ms *MongoStrategy) sanitizeArgs(args []string) []string {
	sanitized := make([]string, len(args))
	copy(sanitized, args)

	for i, arg := range sanitized {
		if strings.HasPrefix(arg, "--uri=") {
			// Sanitize MongoDB URI by replacing password
			uri := strings.TrimPrefix(arg, "--uri=")
			if strings.Contains(uri, "@") {
				parts := strings.Split(uri, "@")
				if len(parts) == 2 {
					userPart := parts[0]
					if strings.Contains(userPart, ":") {
						userParts := strings.Split(userPart, ":")
						if len(userParts) >= 3 { // mongodb://user:password
							userParts[2] = "***"
							sanitized[i] = "--uri=" + strings.Join(userParts, ":") + "@" + parts[1]
						}
					}
				}
			}
		}
	}
	return sanitized
}
