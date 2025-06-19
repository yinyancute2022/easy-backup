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

// MySQLStrategy implements DatabaseStrategy for MySQL/MariaDB databases
type MySQLStrategy struct {
	logger *logrus.Logger
}

// NewMySQLStrategy creates a new MySQL backup strategy
func NewMySQLStrategy(logger *logrus.Logger) *MySQLStrategy {
	return &MySQLStrategy{logger: logger}
}

// GetType returns the database type
func (ms *MySQLStrategy) GetType() string {
	return "mysql"
}

// ValidateConnection validates the MySQL connection
func (ms *MySQLStrategy) ValidateConnection(databaseURL string) error {
	if !strings.HasPrefix(databaseURL, "mysql://") {
		return fmt.Errorf("invalid MySQL URL format")
	}
	return nil
}

// Backup performs a MySQL backup using mariadb-dump
func (ms *MySQLStrategy) Backup(ctx context.Context, databaseURL, outputPath string, callback ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		CommandLogs: make([]string, 0),
	}

	if callback != nil {
		callback("mysql", "Starting MySQL/MariaDB backup (tables and data only, excluding routines/triggers)...")
	}

	// Parse MySQL connection parameters
	params, err := ms.parseConnectionURL(databaseURL)
	if err != nil {
		if callback != nil {
			callback("mysql", fmt.Sprintf("❌ Invalid connection URL: %s", err.Error()))
		}
		return result, fmt.Errorf("invalid MySQL connection URL: %w", err)
	}

	args := []string{
		"--host=" + params.Host,
		"--port=" + params.Port,
		"--user=" + params.User,
		"--password=" + params.Password,
		"--protocol=TCP",
		"--ssl=0", // Disable SSL to avoid certificate issues in Docker
		"--single-transaction",
		"--add-drop-table",
		"--disable-keys",
		"--extended-insert",
		"--quick",
		"--lock-tables=false",
		"--no-tablespaces", // Avoid privilege issues with tablespaces
		"--skip-add-locks",
		"--result-file=" + outputPath,
		params.Database,
	}

	cmd := exec.CommandContext(ctx, "mariadb-dump", args...)

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
		commandLog := fmt.Sprintf("Command failed to start: mariadb-dump %s - Error: %s", strings.Join(ms.sanitizeArgs(args), " "), err.Error())
		result.CommandLogs = append(result.CommandLogs, commandLog)
		if callback != nil {
			callback("mysql", fmt.Sprintf("❌ Command failed to start: %s", err.Error()))
		}
		return result, fmt.Errorf("mariadb-dump failed to start: %w", err)
	}

	// Log the command execution (with masked password)
	commandLog := fmt.Sprintf("Command: mariadb-dump %s", strings.Join(ms.sanitizeArgs(args), " "))
	result.CommandLogs = append(result.CommandLogs, commandLog)

	// Capture output in real-time
	go ms.captureOutput(stdout, "stdout", result, callback)
	go ms.captureOutput(stderr, "stderr", result, callback)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		errorLog := fmt.Sprintf("Command failed: mariadb-dump %s - Error: %s", strings.Join(ms.sanitizeArgs(args), " "), err.Error())
		result.CommandLogs = append(result.CommandLogs, errorLog)
		if callback != nil {
			callback("mysql", fmt.Sprintf("❌ MySQL backup failed: %s", err.Error()))
		}
		return result, fmt.Errorf("mariadb-dump failed: %w", err)
	}

	// Check if output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		errorMsg := "backup file was not created"
		result.CommandLogs = append(result.CommandLogs, fmt.Sprintf("Error: %s", errorMsg))
		if callback != nil {
			callback("mysql", fmt.Sprintf("❌ %s", errorMsg))
		}
		return result, fmt.Errorf("%s", errorMsg)
	}

	result.BackupPath = outputPath
	if callback != nil {
		callback("mysql", "MySQL/MariaDB backup completed successfully")
	}

	return result, nil
}

// ConnectionParams holds MySQL connection parameters
type ConnectionParams struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// parseConnectionURL parses a MySQL connection URL
func (ms *MySQLStrategy) parseConnectionURL(databaseURL string) (*ConnectionParams, error) {
	// Parse MySQL connection string: mysql://user:password@host:port/database
	connStr := strings.TrimPrefix(databaseURL, "mysql://")

	var params ConnectionParams
	params.Port = "3306" // default port

	if atIndex := strings.Index(connStr, "@"); atIndex != -1 {
		userPass := connStr[:atIndex]
		hostPortDB := connStr[atIndex+1:]

		// Parse user:password
		if colonIndex := strings.Index(userPass, ":"); colonIndex != -1 {
			params.User = userPass[:colonIndex]
			params.Password = userPass[colonIndex+1:]
		} else {
			params.User = userPass
		}

		// Parse host:port/database
		if slashIndex := strings.Index(hostPortDB, "/"); slashIndex != -1 {
			hostPort := hostPortDB[:slashIndex]
			params.Database = hostPortDB[slashIndex+1:]

			// Parse host:port
			if colonIndex := strings.Index(hostPort, ":"); colonIndex != -1 {
				params.Host = hostPort[:colonIndex]
				params.Port = hostPort[colonIndex+1:]
			} else {
				params.Host = hostPort
			}
		} else {
			params.Host = hostPortDB
		}
	} else {
		return nil, fmt.Errorf("invalid MySQL connection URL format: %s", databaseURL)
	}

	if params.Host == "" || params.User == "" || params.Database == "" {
		return nil, fmt.Errorf("missing required connection parameters in URL: %s", databaseURL)
	}

	return &params, nil
}

// captureOutput captures command output in real-time
func (ms *MySQLStrategy) captureOutput(pipe io.ReadCloser, streamType string, result *BackupResult, callback ProgressCallback) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	var outputBuffer strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		outputBuffer.WriteString(line)
		outputBuffer.WriteString("\n")

		// Check for MySQL error patterns
		if ms.containsError(line) && callback != nil {
			callback("mysql", fmt.Sprintf("❌ MYSQL ERROR: %s", line))
		}

		// Send other relevant lines
		if callback != nil && ms.shouldReportLine(line) {
			callback("mysql", fmt.Sprintf("[%s] %s", streamType, line))
		}
	}

	// Store the complete output in command logs
	if outputBuffer.Len() > 0 {
		outputLog := fmt.Sprintf("Output (%s): %s", streamType, outputBuffer.String())
		result.CommandLogs = append(result.CommandLogs, outputLog)
	}
}

// containsError checks if the output contains MySQL error patterns
func (ms *MySQLStrategy) containsError(output string) bool {
	outputLower := strings.ToLower(output)

	errorPatterns := []string{
		"access denied",
		"error:",
		"failed to",
		"permission denied",
		"connection refused",
		"authentication failed",
		"unknown database",
		"table doesn't exist",
		"privilege",
		"got error",
		"can't connect",
		"timeout",
		"host is not allowed",
		"too many connections",
		"you need (at least one of)",
		"the process privilege",
		"couldn't execute",
		"unknown table",
		"information_schema",
		"libraries",
	}

	for _, pattern := range errorPatterns {
		if strings.Contains(outputLower, pattern) {
			return true
		}
	}

	return false
}

// shouldReportLine determines if a line should be reported to the callback
func (ms *MySQLStrategy) shouldReportLine(line string) bool {
	lineLower := strings.ToLower(line)
	return strings.Contains(lineLower, "error") ||
		strings.Contains(lineLower, "warning") ||
		strings.Contains(lineLower, "failed") ||
		strings.Contains(lineLower, "success")
}

// sanitizeArgs removes sensitive information from command arguments
func (ms *MySQLStrategy) sanitizeArgs(args []string) []string {
	sanitized := make([]string, len(args))
	copy(sanitized, args)

	for i, arg := range sanitized {
		if strings.HasPrefix(arg, "--password=") {
			sanitized[i] = "--password=***"
		}
	}
	return sanitized
}
