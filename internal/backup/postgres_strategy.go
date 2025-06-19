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

// PostgresStrategy implements DatabaseStrategy for PostgreSQL databases
type PostgresStrategy struct {
	logger *logrus.Logger
}

// NewPostgresStrategy creates a new PostgreSQL backup strategy
func NewPostgresStrategy(logger *logrus.Logger) *PostgresStrategy {
	return &PostgresStrategy{logger: logger}
}

// GetType returns the database type
func (ps *PostgresStrategy) GetType() string {
	return "postgres"
}

// ValidateConnection validates the PostgreSQL connection
func (ps *PostgresStrategy) ValidateConnection(databaseURL string) error {
	// Simple validation - could be enhanced with actual connection test
	if !strings.HasPrefix(databaseURL, "postgres://") && !strings.HasPrefix(databaseURL, "postgresql://") {
		return fmt.Errorf("invalid PostgreSQL URL format")
	}
	return nil
}

// Backup performs a PostgreSQL backup using pg_dump
func (ps *PostgresStrategy) Backup(ctx context.Context, databaseURL, outputPath string, callback ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		CommandLogs: make([]string, 0),
	}

	if callback != nil {
		callback("postgres", "Starting PostgreSQL backup...")
	}

	args := []string{
		databaseURL,
		"--no-password",
		"--verbose",
		"--format=custom",
		"--file=" + outputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

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
		commandLog := fmt.Sprintf("Command failed to start: pg_dump %s - Error: %s", strings.Join(ps.sanitizeArgs(args), " "), err.Error())
		result.CommandLogs = append(result.CommandLogs, commandLog)
		if callback != nil {
			callback("postgres", fmt.Sprintf("❌ Command failed to start: %s", err.Error()))
		}
		return result, fmt.Errorf("pg_dump failed to start: %w", err)
	}

	// Log the command execution
	commandLog := fmt.Sprintf("Command: pg_dump %s", strings.Join(ps.sanitizeArgs(args), " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	// Capture output in real-time
	go ps.captureOutput(stdout, "stdout", result, callback)
	go ps.captureOutput(stderr, "stderr", result, callback)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		errorLog := fmt.Sprintf("Command failed: pg_dump %s - Error: %s", strings.Join(ps.sanitizeArgs(args), " "), err.Error())
		result.CommandLogs = append(result.CommandLogs, errorLog)
		if callback != nil {
			callback("postgres", fmt.Sprintf("❌ PostgreSQL backup failed: %s", err.Error()))
		}
		return result, fmt.Errorf("pg_dump failed: %w", err)
	}

	// Check if output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		errorMsg := "backup file was not created"
		result.CommandLogs = append(result.CommandLogs, fmt.Sprintf("Error: %s", errorMsg))
		if callback != nil {
			callback("postgres", fmt.Sprintf("❌ %s", errorMsg))
		}
		return result, fmt.Errorf("%s", errorMsg)
	}

	result.BackupPath = outputPath
	if callback != nil {
		callback("postgres", "PostgreSQL backup completed successfully")
	}

	return result, nil
}

// captureOutput captures command output in real-time
func (ps *PostgresStrategy) captureOutput(pipe io.ReadCloser, streamType string, result *BackupResult, callback ProgressCallback) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	var outputBuffer strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		outputBuffer.WriteString(line)
		outputBuffer.WriteString("\n")

		// Check for PostgreSQL error patterns
		if ps.containsError(line) && callback != nil {
			callback("postgres", fmt.Sprintf("❌ PostgreSQL ERROR: %s", line))
		}

		// Send other relevant lines
		if callback != nil && ps.shouldReportLine(line) {
			callback("postgres", fmt.Sprintf("[%s] %s", streamType, line))
		}
	}

	// Store the complete output in command logs
	if outputBuffer.Len() > 0 {
		outputLog := fmt.Sprintf("Output (%s): %s", streamType, outputBuffer.String())
		result.CommandLogs = append(result.CommandLogs, outputLog)
	}
}

// containsError checks if the output contains PostgreSQL error patterns
func (ps *PostgresStrategy) containsError(output string) bool {
	outputLower := strings.ToLower(output)

	errorPatterns := []string{
		"fatal:",
		"error:",
		"failed to",
		"permission denied",
		"connection refused",
		"authentication failed",
		"database does not exist",
		"role does not exist",
		"could not connect",
		"timeout",
		"password authentication failed",
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
func (ps *PostgresStrategy) shouldReportLine(line string) bool {
	lineLower := strings.ToLower(line)
	return strings.Contains(lineLower, "error") ||
		strings.Contains(lineLower, "warning") ||
		strings.Contains(lineLower, "failed") ||
		strings.Contains(lineLower, "success")
}

// sanitizeArgs removes sensitive information from command arguments
func (ps *PostgresStrategy) sanitizeArgs(args []string) []string {
	sanitized := make([]string, len(args))
	copy(sanitized, args)

	// PostgreSQL URLs contain credentials, so we need to sanitize the first argument
	if len(sanitized) > 0 && strings.Contains(sanitized[0], "://") {
		// Replace password in URL format: postgresql://user:password@host:port/database
		url := sanitized[0]
		if strings.Contains(url, "@") {
			parts := strings.Split(url, "@")
			if len(parts) == 2 {
				userPart := parts[0]
				if strings.Contains(userPart, ":") {
					userParts := strings.Split(userPart, ":")
					if len(userParts) >= 3 { // scheme://user:password
						userParts[2] = "***"
						sanitized[0] = strings.Join(userParts, ":") + "@" + parts[1]
					}
				}
			}
		}
	}
	return sanitized
}
