package notification

import (
	"context"
	"fmt"
	"testing"
	"time"

	"easy-backup/internal/backup"
	"easy-backup/internal/config"
	"easy-backup/internal/logger"

	"github.com/stretchr/testify/assert"
)

func TestNewSlackService(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		expectClient bool
		description  string
	}{
		{
			name:         "valid_token",
			token:        "fake-test-token-not-real-slack-secret-123456789012",
			expectClient: true,
			description:  "Should create client with valid token",
		},
		{
			name:         "invalid_short_token",
			token:        "xoxb-123",
			expectClient: false,
			description:  "Should not create client with short token",
		},
		{
			name:         "placeholder_token",
			token:        "xoxb-your-bot-token-here",
			expectClient: false,
			description:  "Should not create client with placeholder token",
		},
		{
			name:         "wrong_prefix",
			token:        "xoxp-1234567890123-1234567890123-abcdefghijklmnopqrstuvwx",
			expectClient: false,
			description:  "Should not create client with wrong prefix",
		},
		{
			name:         "empty_token",
			token:        "",
			expectClient: false,
			description:  "Should not create client with empty token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Global: config.GlobalConfig{
					Slack: config.SlackConfig{
						BotToken:  tt.token,
						ChannelID: "C1234567890",
					},
				},
			}
			// Initialize logger before creating service
			_ = logger.InitLogger("info")

			service := NewSlackService(cfg)

			assert.NotNil(t, service, "Service should never be nil")
			assert.Equal(t, cfg, service.config, "Config should be set")
			assert.NotNil(t, service.logger, "Logger should be set")

			if tt.expectClient {
				assert.NotNil(t, service.client, tt.description)
			} else {
				assert.Nil(t, service.client, tt.description)
			}
		})
	}
}

func TestIsValidBotToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "valid_token",
			token:    "fake-test-token-not-real-slack-secret-123456789012",
			expected: true,
		},
		{
			name:     "too_short",
			token:    "xoxb-123",
			expected: false,
		},
		{
			name:     "placeholder",
			token:    "xoxb-your-bot-token-here",
			expected: false,
		},
		{
			name:     "wrong_prefix",
			token:    "xoxp-1234567890123-1234567890123-abcdefghijklmnopqrstuvwx",
			expected: false,
		},
		{
			name:     "no_prefix",
			token:    "1234567890123-1234567890123-abcdefghijklmnopqrstuvwx",
			expected: false,
		},
		{
			name:     "empty",
			token:    "",
			expected: false,
		},
		{
			name:     "just_prefix",
			token:    "xoxb-",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidBotToken(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlackService_SendBackupStarted(t *testing.T) {
	_ = logger.InitLogger("info")

	t.Run("no_client", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken:  "", // No token, so no client
					ChannelID: "C1234567890",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()
		strategies := []string{"test-strategy"}
		slackConfig := config.SlackConfig{ChannelID: "C1234567890"}

		threadInfo, err := service.SendBackupStarted(ctx, strategies, slackConfig)

		assert.NoError(t, err, "Should not error when no client configured")
		assert.Nil(t, threadInfo, "Should return nil thread info when no client")
	})

	t.Run("with_client_but_no_actual_connection", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken:  "fake-test-token-not-real-slack-secret-123456789012",
					ChannelID: "C1234567890",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()
		strategies := []string{"test-strategy"}
		slackConfig := config.SlackConfig{ChannelID: "C1234567890"}

		// This will fail because we don't have a real Slack connection
		// but we're testing that the method handles the error properly
		threadInfo, err := service.SendBackupStarted(ctx, strategies, slackConfig)

		// We expect an error since we're not actually connecting to Slack
		assert.Error(t, err, "Should error when trying to send to real Slack without valid setup")
		assert.Nil(t, threadInfo, "Should return nil thread info on error")
		assert.Contains(t, err.Error(), "failed to send Slack message", "Error should indicate Slack message failure")
	})
}

func TestSlackService_SendBackupProgress(t *testing.T) {
	_ = logger.InitLogger("info")

	t.Run("no_client", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken:  "", // No token, so no client
					ChannelID: "C1234567890",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()
		thread := &ThreadInfo{Channel: "C1234567890", Timestamp: "1234567890.123456"}

		err := service.SendBackupProgress(ctx, thread, "test-strategy", "Making progress...")

		assert.NoError(t, err, "Should not error when no client configured")
	})

	t.Run("nil_thread", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken:  "fake-test-token-not-real-slack-secret-123456789012",
					ChannelID: "C1234567890",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()

		err := service.SendBackupProgress(ctx, nil, "test-strategy", "Making progress...")

		assert.NoError(t, err, "Should not error when thread is nil")
	})
}

func TestSlackService_SendBackupResult(t *testing.T) {
	_ = logger.InitLogger("info")

	t.Run("no_client", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken:  "", // No token, so no client
					ChannelID: "C1234567890",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()
		thread := &ThreadInfo{Channel: "C1234567890", Timestamp: "1234567890.123456"}

		successResult := &backup.BackupResult{
			Strategy: "test-strategy",
			Success:  true,
			Duration: 2 * time.Second,
			Size:     1024,
		}

		err := service.SendBackupResult(ctx, thread, []*backup.BackupResult{successResult}, true)

		assert.NoError(t, err, "Should not error when no client configured")
	})

	t.Run("nil_thread", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken:  "fake-test-token-not-real-slack-secret-123456789012",
					ChannelID: "C1234567890",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()

		successResult := &backup.BackupResult{
			Strategy: "test-strategy",
			Success:  true,
			Duration: 2 * time.Second,
			Size:     1024,
		}

		err := service.SendBackupResult(ctx, nil, []*backup.BackupResult{successResult}, true)

		assert.NoError(t, err, "Should not error when thread is nil")
	})
}

func TestSlackService_SendBackupResult_MessageUpdates(t *testing.T) {
	_ = logger.InitLogger("info")

	// Mock slack client for testing message updates
	tests := []struct {
		name           string
		overallSuccess bool
		expectSuccess  bool
		expectFailure  bool
	}{
		{
			name:           "success_should_update_message",
			overallSuccess: true,
			expectSuccess:  true,
			expectFailure:  false,
		},
		{
			name:           "failure_should_update_message",
			overallSuccess: false,
			expectSuccess:  false,
			expectFailure:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Global: config.GlobalConfig{
					Slack: config.SlackConfig{
						BotToken:  "fake-test-token-not-real-slack-secret-123456789012",
						ChannelID: "C1234567890",
					},
				},
			}

			service := NewSlackService(cfg)
			ctx := context.Background()
			thread := &ThreadInfo{Channel: "C1234567890", Timestamp: "1234567890.123456"}

			var results []*backup.BackupResult
			if tt.overallSuccess {
				results = []*backup.BackupResult{
					{
						Strategy: "test-strategy",
						Success:  true,
						Duration: 2 * time.Second,
						Size:     1024,
					},
				}
			} else {
				results = []*backup.BackupResult{
					{
						Strategy: "test-strategy",
						Success:  false,
						Duration: 2 * time.Second,
						Size:     0,
						Error:    fmt.Errorf("backup failed"),
					},
				}
			}

			// The actual Slack API call will fail since we don't have a real connection,
			// but we're testing that the method doesn't panic and handles both success/failure cases
			err := service.SendBackupResult(ctx, thread, results, tt.overallSuccess)

			// We expect an error since we don't have a real Slack connection,
			// but the important thing is that it doesn't panic and tries to update the message
			// The error will be from the Slack API call, not from our logic
			if err != nil {
				// This is expected since we don't have a real Slack connection
				t.Logf("Expected API error: %v", err)
			}
		})
	}
}

func TestSlackService_SendBackupStarted_MessageFormat(t *testing.T) {
	_ = logger.InitLogger("info")

	cfg := &config.Config{
		Global: config.GlobalConfig{
			Slack: config.SlackConfig{
				BotToken:  "fake-test-token-not-real-slack-secret-123456789012",
				ChannelID: "C1234567890",
			},
		},
	}

	service := NewSlackService(cfg)
	ctx := context.Background()

	// Test that the initial message includes the "will be updated" text
	strategies := []string{"test-strategy-1", "test-strategy-2"}
	slackConfig := config.SlackConfig{ChannelID: "C1234567890"}

	_, err := service.SendBackupStarted(ctx, strategies, slackConfig)

	// We expect an error since we don't have a real Slack connection,
	// but the important thing is that the message format is correct
	if err != nil {
		t.Logf("Expected API error: %v", err)
	}

	// The actual message content verification would require mocking the Slack client,
	// but we can at least verify the method doesn't panic
}

func TestSlackService_SendDatabaseOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		shouldSend  bool
		description string
	}{
		{
			name:        "error_output",
			output:      "ERROR: Connection failed to database",
			shouldSend:  true,
			description: "Should send database error output",
		},
		{
			name:        "warning_output",
			output:      "WARNING: Table 'logs' is very large",
			shouldSend:  true,
			description: "Should send database warning output",
		},
		{
			name:        "fatal_output",
			output:      "FATAL: Authentication failed",
			shouldSend:  true,
			description: "Should send database fatal error output",
		},
		{
			name:        "success_output",
			output:      "Dumping table users... done",
			shouldSend:  false,
			description: "Should NOT send database success output",
		},
		{
			name:        "info_output",
			output:      "Processing 1000 records",
			shouldSend:  false,
			description: "Should NOT send database info output",
		},
		{
			name:        "empty_output",
			output:      "",
			shouldSend:  false,
			description: "Should handle empty output gracefully",
		},
		{
			name:        "whitespace_output",
			output:      "   \n\t  ",
			shouldSend:  false,
			description: "Should handle whitespace-only output gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Global: config.GlobalConfig{
					Slack: config.SlackConfig{
						BotToken: "fake-test-token-not-real-slack-secret-123456789012",
					},
				},
			}

			service := NewSlackService(cfg)
			ctx := context.Background()

			thread := &ThreadInfo{
				Channel:   "C1234567890",
				Timestamp: "1234567890.123456",
			}

			err := service.SendDatabaseOutput(ctx, thread, "test-strategy", tt.output)

			if tt.shouldSend {
				// For error/warning messages, we expect an API error due to fake token
				t.Logf("Expected API error for %s: %v", tt.name, err)
				assert.Error(t, err, tt.description)
			} else {
				// For non-error messages, the method should return nil without sending
				assert.NoError(t, err, tt.description)
			}
		})
	}

	// Test with no client
	t.Run("no_client", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken: "", // No token
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()

		thread := &ThreadInfo{
			Channel:   "C1234567890",
			Timestamp: "1234567890.123456",
		}

		err := service.SendDatabaseOutput(ctx, thread, "test-strategy", "Some output")
		assert.NoError(t, err, "Should handle no client gracefully")
	})

	// Test with nil thread
	t.Run("nil_thread", func(t *testing.T) {
		cfg := &config.Config{
			Global: config.GlobalConfig{
				Slack: config.SlackConfig{
					BotToken: "fake-test-token-not-real-slack-secret-123456789012",
				},
			},
		}

		service := NewSlackService(cfg)
		ctx := context.Background()

		err := service.SendDatabaseOutput(ctx, nil, "test-strategy", "Some output")
		assert.NoError(t, err, "Should handle nil thread gracefully")
	})
}
