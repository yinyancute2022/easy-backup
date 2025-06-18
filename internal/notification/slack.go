package notification

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"easy-backup/internal/backup"
	"easy-backup/internal/config"
	"easy-backup/internal/logger"
)

// SlackService handles Slack notifications
type SlackService struct {
	config *config.Config
	logger *logrus.Logger
	client *slack.Client
}

// ThreadInfo stores information about a Slack thread
type ThreadInfo struct {
	Channel   string
	Timestamp string
}

// NewSlackService creates a new Slack service
func NewSlackService(cfg *config.Config) *SlackService {
	var client *slack.Client

	// Use bot token from config (which can be loaded from environment)
	if cfg.Global.Slack.BotToken != "" && isValidBotToken(cfg.Global.Slack.BotToken) {
		// Configure Slack client with bot token
		client = slack.New(
			cfg.Global.Slack.BotToken,
			slack.OptionDebug(false), // Set to true for debugging
		)
	} else if cfg.Global.Slack.BotToken != "" {
		// Log warning for invalid token format
		logger.GetLogger().Warn("Invalid Slack bot token format. Expected format: xoxb-... (real token, not placeholder)")
	}

	return &SlackService{
		config: cfg,
		logger: logger.GetLogger(),
		client: client,
	}
}

// isValidBotToken validates that the token is a bot token
func isValidBotToken(token string) bool {
	// Allow test tokens for testing
	if strings.HasPrefix(token, "fake-test-token-") {
		return true
	}

	// Check if it starts with xoxb- and has a reasonable length
	// Real Slack bot tokens are typically much longer than the placeholder
	if !strings.HasPrefix(token, "xoxb-") {
		return false
	}

	// Check if it's not a placeholder token
	if strings.Contains(token, "your-bot-token-here") || len(token) < 50 {
		return false
	}

	return true
}

// SendBackupStarted sends the initial backup started message
func (ss *SlackService) SendBackupStarted(ctx context.Context, strategies []string, slackConfig config.SlackConfig) (*ThreadInfo, error) {
	if ss.client == nil {
		ss.logger.Warn("Slack client not configured, skipping notification")
		return nil, nil
	}

	var message string
	if len(strategies) == 1 {
		message = fmt.Sprintf("🔄 **Database Backup Started**\n\n"+
			"**Strategy:** %s\n"+
			"**Started at:** %s\n\n"+
			"_This message will be updated with the final status..._",
			strategies[0],
			time.Now().Format("2006-01-02 15:04:05 UTC"))
	} else {
		message = fmt.Sprintf("🔄 **Database Backups Started**\n\n"+
			"**Total Strategies:** %d\n"+
			"**Strategies:** %s\n"+
			"**Started at:** %s\n\n"+
			"_This message will be updated with the final status..._",
			len(strategies),
			strings.Join(strategies, ", "),
			time.Now().Format("2006-01-02 15:04:05 UTC"))
	}

	timestamp, err := ss.sendMessage(ctx, slackConfig.ChannelID, message)
	if err != nil {
		return nil, err
	}

	return &ThreadInfo{
		Channel:   slackConfig.ChannelID,
		Timestamp: timestamp,
	}, nil
}

// SendBackupProgress sends a progress update in the thread
func (ss *SlackService) SendBackupProgress(ctx context.Context, thread *ThreadInfo, strategy string, message string) error {
	if ss.client == nil || thread == nil {
		return nil
	}

	// Determine the icon based on message content
	var icon string
	messageLower := strings.ToLower(message)
	if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "failed") || strings.Contains(messageLower, "failure") {
		icon = "❌"
	} else if strings.Contains(messageLower, "retry") || strings.Contains(messageLower, "retrying") {
		icon = "🔄"
	} else if strings.Contains(messageLower, "uploading") {
		icon = "📤"
	} else if strings.Contains(messageLower, "cleaning") || strings.Contains(messageLower, "cleanup") {
		icon = "🧹"
	} else if strings.Contains(messageLower, "completed") || strings.Contains(messageLower, "success") {
		icon = "✅"
	} else {
		icon = "📊"
	}

	progressMessage := fmt.Sprintf("%s **%s**: %s", icon, strategy, message)
	_, err := ss.sendThreadMessage(ctx, thread.Channel, thread.Timestamp, progressMessage)
	return err
}

// SendBackupResult sends the final backup result
func (ss *SlackService) SendBackupResult(ctx context.Context, thread *ThreadInfo, results []*backup.BackupResult, overallSuccess bool) error {
	if ss.client == nil || thread == nil {
		return nil
	}

	// Create summary message
	var message string
	if overallSuccess {
		message = "✅ **Database Backup Completed Successfully**\n\n"
	} else {
		message = "❌ **Database Backup Failed**\n\n"
	}

	// Add details for each strategy
	for _, result := range results {
		var status, icon string
		if result.Success {
			status = "Success"
			icon = "✅"
		} else {
			status = "Failed"
			icon = "❌"
		}

		message += fmt.Sprintf("%s **%s**: %s\n", icon, result.Strategy, status)

		if result.Success {
			message += fmt.Sprintf("   • Duration: %v\n", result.Duration.Round(time.Second))
			message += fmt.Sprintf("   • Size: %s\n", formatBytes(result.Size))
			if result.BackupPath != "" {
				message += fmt.Sprintf("   • File: %s\n", result.BackupPath)
			}
			// Note: Database output is only shown for failed backups
		} else {
			// Enhanced error information for failed backups
			if result.Error != nil {
				message += fmt.Sprintf("   • **Error**: %s\n", result.Error.Error())
			}

			if result.Duration > 0 {
				message += fmt.Sprintf("   • Duration before failure: %v\n", result.Duration.Round(time.Second))
			}

			if !result.StartTime.IsZero() {
				message += fmt.Sprintf("   • Started at: %s\n", result.StartTime.Format("15:04:05 UTC"))
			}

			if !result.EndTime.IsZero() {
				message += fmt.Sprintf("   • Failed at: %s\n", result.EndTime.Format("15:04:05 UTC"))
			}

			// Include command logs if available
			if len(result.CommandLogs) > 0 {
				message += "   • **Command Details**:\n"
				for _, cmdLog := range result.CommandLogs {
					// Truncate very long output to avoid Slack message limits
					if len(cmdLog) > 500 {
						cmdLog = cmdLog[:497] + "..."
					}
					// Format command logs with proper indentation
					lines := strings.Split(cmdLog, "\n")
					for _, line := range lines {
						if strings.TrimSpace(line) != "" {
							message += fmt.Sprintf("     `%s`\n", line)
						}
					}
				}
			}
		}
		message += "\n"
	}

	message += fmt.Sprintf("Completed at: %s", time.Now().Format("2006-01-02 15:04:05 UTC"))

	// Send final message
	_, err := ss.sendThreadMessage(ctx, thread.Channel, thread.Timestamp, message)
	if err != nil {
		return err
	}

	// Always update the initial message with final status
	var updatedMessage string

	// Count successful and failed backups
	totalBackups := len(results)
	successfulBackups := 0
	failedBackups := 0
	var totalSize int64
	var totalDuration time.Duration
	var strategies []string

	for _, result := range results {
		strategies = append(strategies, result.Strategy)
		if result.Success {
			successfulBackups++
			totalSize += result.Size
		} else {
			failedBackups++
		}
		totalDuration += result.Duration
	}

	if overallSuccess {
		if totalBackups == 1 {
			// Single backup
			result := results[0]
			updatedMessage = fmt.Sprintf("✅ **Database Backup Completed Successfully**\n\n"+
				"**Strategy:** %s\n"+
				"**Size:** %s\n"+
				"**Duration:** %v\n"+
				"**Completed at:** %s\n\n"+
				"_See thread for detailed logs_",
				result.Strategy,
				formatBytes(result.Size),
				result.Duration.Round(time.Second),
				time.Now().Format("2006-01-02 15:04:05 UTC"))
		} else {
			// Multiple backups
			updatedMessage = fmt.Sprintf("✅ **Database Backups Completed Successfully**\n\n"+
				"**Total Backups:** %d/%d successful\n"+
				"**Strategies:** %s\n"+
				"**Total Size:** %s\n"+
				"**Total Duration:** %v\n"+
				"**Completed at:** %s\n\n"+
				"_See thread for detailed logs_",
				successfulBackups, totalBackups,
				strings.Join(strategies, ", "),
				formatBytes(totalSize),
				totalDuration.Round(time.Second),
				time.Now().Format("2006-01-02 15:04:05 UTC"))
		}
	} else {
		if totalBackups == 1 {
			// Single backup failed
			result := results[0]
			updatedMessage = fmt.Sprintf("❌ **Database Backup Failed**\n\n"+
				"**Strategy:** %s\n"+
				"**Error:** %s\n"+
				"**Duration:** %v\n"+
				"**Failed at:** %s\n\n"+
				"_See thread for detailed error information_",
				result.Strategy,
				func() string {
					if result.Error != nil {
						errorMsg := result.Error.Error()
						if len(errorMsg) > 100 {
							return errorMsg[:97] + "..."
						}
						return errorMsg
					}
					return "Unknown error"
				}(),
				result.Duration.Round(time.Second),
				time.Now().Format("2006-01-02 15:04:05 UTC"))
		} else {
			// Multiple backups with failures
			updatedMessage = fmt.Sprintf("❌ **Database Backups Failed**\n\n"+
				"**Results:** %d successful, %d failed (%d total)\n"+
				"**Strategies:** %s\n"+
				"**Total Duration:** %v\n"+
				"**Completed at:** %s\n\n"+
				"_See thread for detailed error information_",
				successfulBackups, failedBackups, totalBackups,
				strings.Join(strategies, ", "),
				totalDuration.Round(time.Second),
				time.Now().Format("2006-01-02 15:04:05 UTC"))
		}
	}

	err = ss.updateMessage(ctx, thread.Channel, thread.Timestamp, updatedMessage)
	if err != nil {
		ss.logger.WithError(err).Warn("Failed to update original message with final status")
	}

	return nil
}

// SendDetailedError sends detailed error information for debugging
func (ss *SlackService) SendDetailedError(ctx context.Context, thread *ThreadInfo, strategy string, result *backup.BackupResult) error {
	if ss.client == nil || thread == nil || result == nil {
		return nil
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("🔍 **Detailed Error Information for %s**\n\n", strategy))

	if result.Error != nil {
		message.WriteString(fmt.Sprintf("**Error Message:**\n```%s```\n\n", result.Error.Error()))
	}

	if !result.StartTime.IsZero() {
		message.WriteString(fmt.Sprintf("**Start Time:** %s\n", result.StartTime.Format("2006-01-02 15:04:05 UTC")))
	}

	if !result.EndTime.IsZero() {
		message.WriteString(fmt.Sprintf("**End Time:** %s\n", result.EndTime.Format("2006-01-02 15:04:05 UTC")))
	}

	if result.Duration > 0 {
		message.WriteString(fmt.Sprintf("**Duration:** %v\n", result.Duration.Round(time.Second)))
	}

	if result.BackupPath != "" {
		message.WriteString(fmt.Sprintf("**Backup Path:** %s\n", result.BackupPath))
	}

	if len(result.CommandLogs) > 0 {
		message.WriteString("\n**Command Execution Logs:**\n")
		for i, cmdLog := range result.CommandLogs {
			// Split long logs into multiple messages if needed
			if len(cmdLog) > 2000 {
				// For very long logs, truncate and provide a summary
				message.WriteString(fmt.Sprintf("```Log %d (truncated):\n%s...\n```\n", i+1, cmdLog[:2000]))
			} else {
				message.WriteString(fmt.Sprintf("```Log %d:\n%s\n```\n", i+1, cmdLog))
			}
		}
	}

	_, err := ss.sendThreadMessage(ctx, thread.Channel, thread.Timestamp, message.String())
	return err
}

// SendDatabaseOutput sends database command output to Slack (only errors and warnings)
func (ss *SlackService) SendDatabaseOutput(ctx context.Context, thread *ThreadInfo, strategy string, output string) error {
	if ss.client == nil || thread == nil || strings.TrimSpace(output) == "" {
		return nil
	}

	// Clean up the output and check if it contains errors or warnings
	cleanOutput := strings.TrimSpace(output)
	outputLower := strings.ToLower(cleanOutput)

	// Only send error/warning messages to Slack
	if !strings.Contains(outputLower, "error") &&
		!strings.Contains(outputLower, "failed") &&
		!strings.Contains(outputLower, "warning") &&
		!strings.Contains(outputLower, "warn") &&
		!strings.Contains(outputLower, "fatal") &&
		!strings.Contains(outputLower, "critical") {
		// Skip sending non-error messages
		return nil
	}

	var icon string
	var messageType string

	// Determine message type for errors/warnings
	if strings.Contains(outputLower, "error") || strings.Contains(outputLower, "failed") || strings.Contains(outputLower, "fatal") {
		icon = "❌"
		messageType = "Database Error"
	} else if strings.Contains(outputLower, "warning") || strings.Contains(outputLower, "warn") {
		icon = "⚠️"
		messageType = "Database Warning"
	} else {
		icon = "�"
		messageType = "Database Issue"
	}

	// Truncate very long output
	if len(cleanOutput) > 1500 {
		cleanOutput = cleanOutput[:1497] + "..."
	}

	// Format the message
	var message strings.Builder
	message.WriteString(fmt.Sprintf("%s **%s** - %s:\n", icon, strategy, messageType))
	message.WriteString("```\n")
	message.WriteString(cleanOutput)
	message.WriteString("\n```")

	_, err := ss.sendThreadMessage(ctx, thread.Channel, thread.Timestamp, message.String())
	return err
}

// TestConnection tests the Slack connection
func (ss *SlackService) TestConnection(ctx context.Context) error {
	if ss.client == nil {
		return fmt.Errorf("Slack client not configured")
	}

	// Check if we have a valid bot token
	if !isValidBotToken(ss.config.Global.Slack.BotToken) {
		return fmt.Errorf("invalid or missing Slack bot token")
	}

	// Test bot authentication
	authResp, err := ss.client.AuthTestContext(ctx)
	if err != nil {
		return fmt.Errorf("Slack bot authentication failed: %w", err)
	}

	ss.logger.WithFields(logrus.Fields{
		"bot_id":  authResp.BotID,
		"user_id": authResp.UserID,
		"team":    authResp.Team,
		"team_id": authResp.TeamID,
	}).Info("Slack bot authentication successful")

	// Validate channel access if channel ID is configured
	if ss.config.Global.Slack.ChannelID != "" {
		// Try to get conversation info, but don't fail if we don't have the scope
		_, err := ss.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
			ChannelID: ss.config.Global.Slack.ChannelID,
		})
		if err != nil {
			// Check if it's a scope issue - if so, just log a warning instead of failing
			if strings.Contains(err.Error(), "missing_scope") {
				ss.logger.WithFields(logrus.Fields{
					"channel_id": ss.config.Global.Slack.ChannelID,
					"error":      err.Error(),
				}).Warn("Cannot verify channel access due to missing OAuth scope - this is normal for basic bot tokens")
			} else {
				return fmt.Errorf("failed to access Slack channel %s: %w", ss.config.Global.Slack.ChannelID, err)
			}
		} else {
			ss.logger.WithField("channel_id", ss.config.Global.Slack.ChannelID).Info("Slack channel access verified")
		}
	}

	return nil
}

// sendMessage sends a message to a Slack channel
func (ss *SlackService) sendMessage(ctx context.Context, channel, message string) (string, error) {
	_, timestamp, err := ss.client.PostMessageContext(ctx, channel,
		slack.MsgOptionText(message, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		return "", fmt.Errorf("failed to send Slack message: %w", err)
	}

	ss.logger.WithFields(logrus.Fields{
		"channel":   channel,
		"timestamp": timestamp,
	}).Debug("Sent Slack message")

	return timestamp, nil
}

// sendThreadMessage sends a message as a reply in a thread
func (ss *SlackService) sendThreadMessage(ctx context.Context, channel, threadTimestamp, message string) (string, error) {
	_, timestamp, err := ss.client.PostMessageContext(ctx, channel,
		slack.MsgOptionText(message, false),
		slack.MsgOptionTS(threadTimestamp),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		return "", fmt.Errorf("failed to send Slack thread message: %w", err)
	}

	ss.logger.WithFields(logrus.Fields{
		"channel":         channel,
		"thread":          threadTimestamp,
		"reply_timestamp": timestamp,
	}).Debug("Sent Slack thread message")

	return timestamp, nil
}

// updateMessage updates an existing Slack message
func (ss *SlackService) updateMessage(ctx context.Context, channel, timestamp, message string) error {
	_, _, _, err := ss.client.UpdateMessageContext(ctx, channel, timestamp,
		slack.MsgOptionText(message, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		return fmt.Errorf("failed to update Slack message: %w", err)
	}

	ss.logger.WithFields(logrus.Fields{
		"channel":   channel,
		"timestamp": timestamp,
	}).Debug("Updated Slack message")

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
