package backup

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"easy-backup/internal/config"
)

func TestBackupService(t *testing.T) {
	// Setup test config
	cfg := &config.Config{
		Global: config.GlobalConfig{
			TempDir: "/tmp/backup-test",
			Timeout: config.TimeoutConfig{
				Backup: "5m",
			},
			S3: config.S3Config{
				Compression: "gzip",
			},
		},
	}

	// Create temp directory
	err := os.MkdirAll(cfg.Global.TempDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(cfg.Global.TempDir)

	service := NewBackupService(cfg)

	t.Run("NewBackupService", func(t *testing.T) {
		assert.NotNil(t, service)
		assert.NotNil(t, service.config)
		assert.NotNil(t, service.logger)
		assert.NotNil(t, service.strategies)

		// Check that strategies are registered
		assert.Contains(t, service.strategies, "postgres")
		assert.Contains(t, service.strategies, "mysql")
		assert.Contains(t, service.strategies, "mariadb")
		assert.Contains(t, service.strategies, "mongodb")
	})

	t.Run("GenerateBackupPath", func(t *testing.T) {
		strategyConfig := config.StrategyConfig{
			Name:         "test-strategy",
			DatabaseType: "postgres",
		}

		startTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		path := service.generateBackupPath(strategyConfig, startTime)

		assert.Contains(t, path, "test-strategy-20230101-120000")
		assert.Contains(t, path, ".dump")
		assert.Contains(t, path, cfg.Global.TempDir)
	})

	t.Run("GenerateBackupPath_MySQL", func(t *testing.T) {
		strategyConfig := config.StrategyConfig{
			Name:         "mysql-test",
			DatabaseType: "mysql",
		}

		startTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		path := service.generateBackupPath(strategyConfig, startTime)

		assert.Contains(t, path, "mysql-test-20230101-120000")
		assert.Contains(t, path, ".sql")
	})

	t.Run("GenerateBackupPath_MongoDB", func(t *testing.T) {
		strategyConfig := config.StrategyConfig{
			Name:         "mongo-test",
			DatabaseType: "mongodb",
		}

		startTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		path := service.generateBackupPath(strategyConfig, startTime)

		assert.Contains(t, path, "mongo-test-20230101-120000")
		assert.Contains(t, path, ".archive")
	})

	t.Run("ExecuteBackup_UnsupportedDatabase", func(t *testing.T) {
		strategyConfig := config.StrategyConfig{
			Name:         "unsupported-test",
			DatabaseType: "unsupported",
		}

		result, err := service.ExecuteBackup(context.Background(), strategyConfig)

		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, err.Error(), "unsupported database type")
	})

	t.Run("ExecuteBackup_InvalidConnectionURL", func(t *testing.T) {
		strategyConfig := config.StrategyConfig{
			Name:         "postgres-test",
			DatabaseType: "postgres",
			DatabaseURL:  "invalid-url",
		}

		result, err := service.ExecuteBackup(context.Background(), strategyConfig)

		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
	})

	t.Run("CleanupTempFiles", func(t *testing.T) {
		// Create a test file
		testFile := cfg.Global.TempDir + "/test-cleanup.txt"
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(testFile)
		require.NoError(t, err)

		// Cleanup
		err = service.CleanupTempFiles(testFile)
		assert.NoError(t, err)

		// Verify file is gone
		_, err = os.Stat(testFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("CleanupTempFiles_NonExistentFile", func(t *testing.T) {
		err := service.CleanupTempFiles("/non/existent/file")
		assert.NoError(t, err) // Should not error for non-existent files
	})

	t.Run("CleanupTempFiles_EmptyPath", func(t *testing.T) {
		err := service.CleanupTempFiles("")
		assert.NoError(t, err) // Should handle empty path gracefully
	})
}

func TestBackupResult(t *testing.T) {
	t.Run("BackupResult_Initialization", func(t *testing.T) {
		result := &BackupResult{
			Strategy:    "test-strategy",
			Success:     false,
			CommandLogs: make([]string, 0),
			StartTime:   time.Now(),
		}

		assert.Equal(t, "test-strategy", result.Strategy)
		assert.False(t, result.Success)
		assert.Empty(t, result.CommandLogs)
		assert.Nil(t, result.Error)
	})

	t.Run("BackupResult_WithError", func(t *testing.T) {
		testError := assert.AnError
		result := &BackupResult{
			Strategy: "test-strategy",
			Success:  false,
			Error:    testError,
		}

		assert.Equal(t, testError, result.Error)
		assert.False(t, result.Success)
	})
}

func TestFormatBytes(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tc := range testCases {
		result := formatBytes(tc.bytes)
		assert.Equal(t, tc.expected, result, "Failed for %d bytes", tc.bytes)
	}
}

// TestProgressCallback tests the progress callback functionality
func TestProgressCallback(t *testing.T) {
	var receivedStrategy, receivedMessage string

	callback := func(strategy, message string) {
		receivedStrategy = strategy
		receivedMessage = message
	}

	callback("test-strategy", "test message")

	assert.Equal(t, "test-strategy", receivedStrategy)
	assert.Equal(t, "test message", receivedMessage)
}

// Mock strategy for testing
type mockStrategy struct {
	shouldFail bool
	dbType     string
}

func (ms *mockStrategy) Backup(ctx context.Context, databaseURL, outputPath string, callback ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		CommandLogs: []string{"mock command executed"},
		BackupPath:  outputPath,
	}

	if ms.shouldFail {
		return result, assert.AnError
	}

	// Create a mock backup file
	err := os.WriteFile(outputPath, []byte("mock backup data"), 0644)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (ms *mockStrategy) ValidateConnection(databaseURL string) error {
	if databaseURL == "invalid-url" {
		return assert.AnError
	}
	return nil
}

func (ms *mockStrategy) GetType() string {
	return ms.dbType
}

func TestBackupService_WithMockStrategy(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			TempDir: "/tmp/backup-test-mock",
			Timeout: config.TimeoutConfig{
				Backup: "5m",
			},
			S3: config.S3Config{
				Compression: "none",
			},
		},
	}

	// Create temp directory
	err := os.MkdirAll(cfg.Global.TempDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(cfg.Global.TempDir)

	service := NewBackupService(cfg)

	t.Run("ExecuteBackup_Success", func(t *testing.T) {
		// Replace strategy with mock
		service.strategies["test"] = &mockStrategy{shouldFail: false, dbType: "test"}

		strategyConfig := config.StrategyConfig{
			Name:         "test-strategy",
			DatabaseType: "test",
			DatabaseURL:  "test://localhost:5432/testdb",
		}

		result, err := service.ExecuteBackup(context.Background(), strategyConfig)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, "test-strategy", result.Strategy)
		assert.NotEmpty(t, result.CommandLogs)
		assert.Greater(t, result.Size, int64(0))
	})

	t.Run("ExecuteBackup_StrategyFailure", func(t *testing.T) {
		// Replace strategy with failing mock
		service.strategies["test"] = &mockStrategy{shouldFail: true, dbType: "test"}

		strategyConfig := config.StrategyConfig{
			Name:         "test-strategy",
			DatabaseType: "test",
			DatabaseURL:  "test://localhost:5432/testdb",
		}

		result, err := service.ExecuteBackup(context.Background(), strategyConfig)

		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Equal(t, "test-strategy", result.Strategy)
	})

	t.Run("ExecuteBackupWithProgress", func(t *testing.T) {
		service.strategies["test"] = &mockStrategy{shouldFail: false, dbType: "test"}

		strategyConfig := config.StrategyConfig{
			Name:         "test-strategy",
			DatabaseType: "test",
			DatabaseURL:  "test://localhost:5432/testdb",
		}

		var progressMessages []string
		progressCallback := func(strategy, message string) {
			progressMessages = append(progressMessages, message)
		}

		result, err := service.ExecuteBackupWithProgress(context.Background(), strategyConfig, progressCallback)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.NotEmpty(t, progressMessages)
		assert.Contains(t, progressMessages[0], "Starting database backup")
	})
}
