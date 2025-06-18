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

	message := fmt.Sprintf("🔄 **Database Backup Started**\n\nStrategies: %v\nStarted at: %s\n\n_This message will be updated with the final status..._",
		strategies, time.Now().Format("2006-01-02 15:04:05 UTC"))

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

	progressMessage := fmt.Sprintf("📊 **%s**: %s", strategy, message)
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
		} else if result.Error != nil {
			message += fmt.Sprintf("   • Error: %s\n", result.Error.Error())
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
	if overallSuccess {
		updatedMessage = fmt.Sprintf("✅ **Database Backup Completed Successfully** - See thread for details\n\nCompleted at: %s",
			time.Now().Format("2006-01-02 15:04:05 UTC"))
	} else {
		updatedMessage = fmt.Sprintf("❌ **Database Backup Failed** - See thread for details\n\nCompleted at: %s",
			time.Now().Format("2006-01-02 15:04:05 UTC"))
	}

	err = ss.updateMessage(ctx, thread.Channel, thread.Timestamp, updatedMessage)
	if err != nil {
		ss.logger.WithError(err).Warn("Failed to update original message with final status")
	}

	return nil
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
		_, err := ss.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
			ChannelID: ss.config.Global.Slack.ChannelID,
		})
		if err != nil {
			return fmt.Errorf("failed to access Slack channel %s: %w", ss.config.Global.Slack.ChannelID, err)
		}

		ss.logger.WithField("channel_id", ss.config.Global.Slack.ChannelID).Info("Slack channel access verified")
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
