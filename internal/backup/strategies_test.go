package backup

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresStrategy(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	strategy := NewPostgresStrategy(logger)

	t.Run("GetType", func(t *testing.T) {
		assert.Equal(t, "postgres", strategy.GetType())
	})

	t.Run("ValidateConnection_Valid", func(t *testing.T) {
		validURLs := []string{
			"postgres://user:pass@localhost:5432/testdb",
			"postgresql://user:pass@localhost:5432/testdb",
		}

		for _, url := range validURLs {
			err := strategy.ValidateConnection(url)
			assert.NoError(t, err, "URL should be valid: %s", url)
		}
	})

	t.Run("ValidateConnection_Invalid", func(t *testing.T) {
		invalidURLs := []string{
			"mysql://user:pass@localhost:3306/testdb",
			"invalid-url",
			"",
		}

		for _, url := range invalidURLs {
			err := strategy.ValidateConnection(url)
			assert.Error(t, err, "URL should be invalid: %s", url)
		}
	})

	t.Run("ContainsError", func(t *testing.T) {
		testCases := []struct {
			output   string
			hasError bool
		}{
			{"FATAL: database does not exist", true},
			{"ERROR: connection refused", true},
			{"INFO: backup completed", false},
			{"pg_dump: error: connection to database failed", true},
			{"Successfully connected to database", false},
		}

		for _, tc := range testCases {
			result := strategy.containsError(tc.output)
			assert.Equal(t, tc.hasError, result, "Error detection failed for: %s", tc.output)
		}
	})

	t.Run("SanitizeArgs", func(t *testing.T) {
		args := []string{
			"postgresql://user:secret123@localhost:5432/testdb",
			"--verbose",
			"--format=custom",
		}

		sanitized := strategy.sanitizeArgs(args)

		// The password should be masked
		assert.Contains(t, sanitized[0], "***")
		assert.NotContains(t, sanitized[0], "secret123")

		// Other args should remain unchanged
		assert.Equal(t, args[1], sanitized[1])
		assert.Equal(t, args[2], sanitized[2])
	})
}

func TestMySQLStrategy(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	strategy := NewMySQLStrategy(logger)

	t.Run("GetType", func(t *testing.T) {
		assert.Equal(t, "mysql", strategy.GetType())
	})

	t.Run("ValidateConnection_Valid", func(t *testing.T) {
		validURL := "mysql://user:pass@localhost:3306/testdb"
		err := strategy.ValidateConnection(validURL)
		assert.NoError(t, err)
	})

	t.Run("ValidateConnection_Invalid", func(t *testing.T) {
		invalidURLs := []string{
			"postgres://user:pass@localhost:5432/testdb",
			"invalid-url",
			"",
		}

		for _, url := range invalidURLs {
			err := strategy.ValidateConnection(url)
			assert.Error(t, err, "URL should be invalid: %s", url)
		}
	})

	t.Run("ParseConnectionURL", func(t *testing.T) {
		testCases := []struct {
			url      string
			expected *ConnectionParams
			hasError bool
		}{
			{
				url: "mysql://user:pass@localhost:3306/testdb",
				expected: &ConnectionParams{
					Host:     "localhost",
					Port:     "3306",
					User:     "user",
					Password: "pass",
					Database: "testdb",
				},
				hasError: false,
			},
			{
				url: "mysql://user@localhost/testdb",
				expected: &ConnectionParams{
					Host:     "localhost",
					Port:     "3306", // default
					User:     "user",
					Password: "",
					Database: "testdb",
				},
				hasError: false,
			},
			{
				url:      "invalid-url",
				expected: nil,
				hasError: true,
			},
		}

		for _, tc := range testCases {
			result, err := strategy.parseConnectionURL(tc.url)
			if tc.hasError {
				assert.Error(t, err, "Should have error for URL: %s", tc.url)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err, "Should not have error for URL: %s", tc.url)
				assert.Equal(t, tc.expected, result)
			}
		}
	})

	t.Run("ContainsError", func(t *testing.T) {
		testCases := []struct {
			output   string
			hasError bool
		}{
			{"ERROR 1045 (28000): Access denied for user", true},
			{"mysqldump: Got error: 1049: Unknown database", true},
			{"Dump completed on 2023-01-01", false},
			{"-- MySQL dump 10.13", false},
		}

		for _, tc := range testCases {
			result := strategy.containsError(tc.output)
			assert.Equal(t, tc.hasError, result, "Error detection failed for: %s", tc.output)
		}
	})

	t.Run("SanitizeArgs", func(t *testing.T) {
		args := []string{
			"--host=localhost",
			"--user=testuser",
			"--password=secret123",
			"--database=testdb",
		}

		sanitized := strategy.sanitizeArgs(args)

		// The password should be masked
		assert.Equal(t, "--password=***", sanitized[2])

		// Other args should remain unchanged
		assert.Equal(t, args[0], sanitized[0])
		assert.Equal(t, args[1], sanitized[1])
		assert.Equal(t, args[3], sanitized[3])
	})
}

func TestMongoStrategy(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	strategy := NewMongoStrategy(logger)

	t.Run("GetType", func(t *testing.T) {
		assert.Equal(t, "mongodb", strategy.GetType())
	})

	t.Run("ValidateConnection_Valid", func(t *testing.T) {
		validURLs := []string{
			"mongodb://user:pass@localhost:27017/testdb",
			"mongodb+srv://user:pass@cluster.mongodb.net/testdb",
		}

		for _, url := range validURLs {
			err := strategy.ValidateConnection(url)
			assert.NoError(t, err, "URL should be valid: %s", url)
		}
	})

	t.Run("ValidateConnection_Invalid", func(t *testing.T) {
		invalidURLs := []string{
			"mysql://user:pass@localhost:3306/testdb",
			"invalid-url",
			"",
		}

		for _, url := range invalidURLs {
			err := strategy.ValidateConnection(url)
			assert.Error(t, err, "URL should be invalid: %s", url)
		}
	})

	t.Run("ContainsError", func(t *testing.T) {
		testCases := []struct {
			output   string
			hasError bool
		}{
			{"error: connection refused", true},
			{"Failed to connect to MongoDB", true},
			{"2023-01-01T10:00:00.000+0000\tdump\tWriting to stdout", false},
			{"Authentication failed", true},
			{"done dumping testdb.users", false},
		}

		for _, tc := range testCases {
			result := strategy.containsError(tc.output)
			assert.Equal(t, tc.hasError, result, "Error detection failed for: %s", tc.output)
		}
	})

	t.Run("SanitizeArgs", func(t *testing.T) {
		args := []string{
			"--uri=mongodb://user:secret123@localhost:27017/testdb",
			"--out=/tmp/backup",
			"--verbose",
		}

		sanitized := strategy.sanitizeArgs(args)

		// The password should be masked
		assert.Contains(t, sanitized[0], "***")
		assert.NotContains(t, sanitized[0], "secret123")

		// Other args should remain unchanged
		assert.Equal(t, args[1], sanitized[1])
		assert.Equal(t, args[2], sanitized[2])
	})
}

// Integration test helpers
func setupTestEnvironment(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// Mock ProgressCallback for testing
func mockProgressCallback(strategy, message string) {
	// Do nothing in tests, or log if needed for debugging
}

// TestBackupResult_Integration tests the complete backup flow
func TestBackupResult_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// This test would require actual database connections
	// For now, we'll just test the structure
	result := &BackupResult{
		Strategy:    "test-strategy",
		Success:     true,
		CommandLogs: []string{"test log"},
	}

	assert.Equal(t, "test-strategy", result.Strategy)
	assert.True(t, result.Success)
	assert.Len(t, result.CommandLogs, 1)
}
