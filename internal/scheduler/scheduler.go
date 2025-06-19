package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"easy-backup/internal/backup"
	"easy-backup/internal/config"
	"easy-backup/internal/logger"
	"easy-backup/internal/monitoring"
	"easy-backup/internal/notification"
	"easy-backup/internal/storage"
)

// SchedulerService handles backup scheduling
type SchedulerService struct {
	config            *config.Config
	logger            *logrus.Logger
	cron              *cron.Cron
	backupService     *backup.BackupService
	s3Service         *storage.S3Service
	slackService      *notification.SlackService
	monitoringService *monitoring.MonitoringService
	semaphore         chan struct{}
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(
	cfg *config.Config,
	backupService *backup.BackupService,
	s3Service *storage.S3Service,
	slackService *notification.SlackService,
	monitoringService *monitoring.MonitoringService,
) *SchedulerService {
	// Parse timezone
	location, err := time.LoadLocation(cfg.Global.Timezone)
	if err != nil {
		logger.GetLogger().WithError(err).Warn("Failed to load timezone, using UTC")
		location = time.UTC
	}

	// Create cron with timezone
	cronScheduler := cron.New(cron.WithLocation(location))

	ctx, cancel := context.WithCancel(context.Background())

	return &SchedulerService{
		config:            cfg,
		logger:            logger.GetLogger(),
		cron:              cronScheduler,
		backupService:     backupService,
		s3Service:         s3Service,
		slackService:      slackService,
		monitoringService: monitoringService,
		semaphore:         make(chan struct{}, cfg.Global.MaxParallel),
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start starts the scheduler
func (ss *SchedulerService) Start() error {
	ss.logger.Info("Starting backup scheduler")

	// Schedule each strategy
	for _, strategy := range ss.config.Strategies {
		cronExpr, err := ss.convertToCronExpression(strategy.Schedule)
		if err != nil {
			return fmt.Errorf("invalid schedule for strategy %s: %w", strategy.Name, err)
		}

		// Capture strategy in closure
		strategyConfig := strategy
		_, err = ss.cron.AddFunc(cronExpr, func() {
			ss.executeBackupJob(strategyConfig)
		})
		if err != nil {
			return fmt.Errorf("failed to schedule strategy %s: %w", strategy.Name, err)
		}

		ss.logger.WithFields(logrus.Fields{
			"strategy": strategy.Name,
			"schedule": strategy.Schedule,
			"cron":     cronExpr,
		}).Info("Scheduled backup strategy")
	}

	// Start the cron scheduler
	ss.cron.Start()
	ss.logger.Info("Backup scheduler started")

	return nil
}

// Stop stops the scheduler
func (ss *SchedulerService) Stop() {
	ss.logger.Info("Stopping backup scheduler")
	ss.cancel()
	ctx := ss.cron.Stop()
	<-ctx.Done()
	ss.logger.Info("Backup scheduler stopped")
}

// executeBackupJob executes a backup job for a specific strategy
func (ss *SchedulerService) executeBackupJob(strategy config.StrategyConfig) {
	// Acquire semaphore to limit parallel executions
	select {
	case ss.semaphore <- struct{}{}:
		defer func() { <-ss.semaphore }()
	case <-ss.ctx.Done():
		return
	}

	ss.logger.WithField("strategy", strategy.Name).Info("Starting scheduled backup")

	// Update strategy status
	ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
		Status:  "running",
		LastRun: time.Now().UTC().Format(time.RFC3339),
	})

	// Send Slack notification
	thread, err := ss.slackService.SendBackupStarted(ss.ctx, []string{strategy.Name}, strategy.Slack)
	if err != nil {
		ss.logger.WithError(err).Warn("Failed to send backup started notification")
	}

	// Execute backup with retry
	var result *backup.BackupResult
	var lastErr error

	for attempt := 1; attempt <= ss.config.Global.Retry.MaxAttempts; attempt++ {
		if attempt > 1 {
			ss.logger.WithFields(logrus.Fields{
				"strategy": strategy.Name,
				"attempt":  attempt,
			}).Info("Retrying backup")
			if thread != nil {
				retryMsg := fmt.Sprintf("Retrying backup (attempt %d/%d)", attempt, ss.config.Global.Retry.MaxAttempts)
				if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, retryMsg); err != nil {
					ss.logger.WithError(err).Warn("Failed to send backup progress notification")
				}
			}
		}

		result, lastErr = ss.backupService.ExecuteBackupWithProgress(ss.ctx, strategy, func(strategyName, message string) {
			// Send database output to Slack
			if thread != nil {
				if err := ss.slackService.SendDatabaseOutput(ss.ctx, thread, strategyName, message); err != nil {
					ss.logger.WithError(err).Warn("Failed to send database output to Slack")
				}
			}
		})
		if lastErr == nil {
			break
		}

		ss.logger.WithError(lastErr).WithFields(logrus.Fields{
			"strategy": strategy.Name,
			"attempt":  attempt,
		}).Warn("Backup attempt failed")

		// Send progress update about the failed attempt
		if thread != nil && attempt < ss.config.Global.Retry.MaxAttempts {
			failureMsg := fmt.Sprintf("Attempt %d/%d failed: %s", attempt, ss.config.Global.Retry.MaxAttempts, lastErr.Error())
			if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, failureMsg); err != nil {
				ss.logger.WithError(err).Warn("Failed to send backup progress notification")
			}
		}
	}

	if lastErr != nil {
		// All attempts failed
		ss.handleBackupFailure(strategy, lastErr, result, thread)
		return
	}

	// Backup successful, upload to S3
	if thread != nil {
		if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, "Uploading to S3..."); err != nil {
			ss.logger.WithError(err).Warn("Failed to send backup progress notification")
		}
	}

	s3Location, err := ss.s3Service.UploadBackup(ss.ctx, strategy.Name, result.BackupPath)
	if err != nil {
		ss.logger.WithError(err).WithField("strategy", strategy.Name).Error("Failed to upload backup to S3")
		ss.handleBackupFailure(strategy, err, nil, thread)
		return
	}

	// Clean up local file
	if err := ss.backupService.CleanupTempFiles(result.BackupPath); err != nil {
		ss.logger.WithError(err).Warn("Failed to cleanup temporary files")
	}

	// Clean up old backups
	if thread != nil {
		if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, "Cleaning up old backups..."); err != nil {
			ss.logger.WithError(err).Warn("Failed to send backup progress notification")
		}
	}

	err = ss.s3Service.CleanupOldBackups(ss.ctx, strategy.Name, strategy.Retention)
	if err != nil {
		ss.logger.WithError(err).WithField("strategy", strategy.Name).Warn("Failed to cleanup old backups")
	}

	// Update metrics and status
	ss.monitoringService.RecordBackupMetrics(strategy.Name, result.Duration, result.Size, true)

	nextRun := ss.getNextRunTime(strategy.Schedule)
	ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
		Status:  "success",
		LastRun: time.Now().UTC().Format(time.RFC3339),
		NextRun: nextRun,
	})

	// Send success notification
	if thread != nil {
		if err := ss.slackService.SendBackupResult(ss.ctx, thread, []*backup.BackupResult{result}, true); err != nil {
			ss.logger.WithError(err).Warn("Failed to send backup result notification")
		}
	}

	ss.logger.WithFields(logrus.Fields{
		"strategy":    strategy.Name,
		"duration":    result.Duration,
		"size":        result.Size,
		"s3_location": s3Location,
	}).Info("Backup completed successfully")
}

// handleBackupFailure handles backup failures
func (ss *SchedulerService) handleBackupFailure(strategy config.StrategyConfig, err error, result *backup.BackupResult, thread *notification.ThreadInfo) {
	ss.logger.WithError(err).WithField("strategy", strategy.Name).Error("Backup failed after all retry attempts")

	// Update metrics and status
	ss.monitoringService.RecordBackupMetrics(strategy.Name, 0, 0, false)

	nextRun := ss.getNextRunTime(strategy.Schedule)
	ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
		Status:  "failed",
		LastRun: time.Now().UTC().Format(time.RFC3339),
		NextRun: nextRun,
		Error:   err.Error(),
	})

	// Send failure notification
	if thread != nil {
		var failedResult *backup.BackupResult
		if result != nil {
			// Use the actual backup result with command logs
			failedResult = result
			failedResult.Success = false
			failedResult.Error = err
		} else {
			// Fallback to creating a minimal result
			failedResult = &backup.BackupResult{
				Strategy:  strategy.Name,
				Success:   false,
				Error:     err,
				StartTime: time.Now(),
				EndTime:   time.Now(),
			}
		}

		// Send the main backup result notification
		if err := ss.slackService.SendBackupResult(ss.ctx, thread, []*backup.BackupResult{failedResult}, false); err != nil {
			ss.logger.WithError(err).Warn("Failed to send backup failure notification")
		}

		// Send detailed error information for debugging if we have a result with command logs
		if result != nil && len(result.CommandLogs) > 0 {
			if err := ss.slackService.SendDetailedError(ss.ctx, thread, strategy.Name, failedResult); err != nil {
				ss.logger.WithError(err).Warn("Failed to send detailed error information")
			}
		}
	}
}

// convertToCronExpression validates cron expression
func (ss *SchedulerService) convertToCronExpression(schedule string) (string, error) {
	// Check if it's a valid cron expression
	if ss.isValidCronExpression(schedule) {
		return schedule, nil
	}

	return "", fmt.Errorf("invalid cron expression: %s. Expected format: 'minute hour day month dayOfWeek' (e.g., '0 2 * * *' for daily at 2 AM)", schedule)
}

// isValidCronExpression checks if a string is a valid cron expression
func (ss *SchedulerService) isValidCronExpression(expr string) bool {
	// Try to parse as cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	return err == nil
}

// getNextRunTime calculates the next run time for a schedule
func (ss *SchedulerService) getNextRunTime(schedule string) string {
	cronExpr, err := ss.convertToCronExpression(schedule)
	if err != nil {
		ss.logger.WithError(err).WithField("schedule", schedule).Warn("Failed to convert schedule to cron expression")
		return ""
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(cronExpr)
	if err != nil {
		ss.logger.WithError(err).WithField("cron_expr", cronExpr).Warn("Failed to parse cron expression")
		return ""
	}

	nextTime := sched.Next(time.Now())
	return nextTime.UTC().Format(time.RFC3339)
}

// ExecuteAllStrategiesManually executes all backup strategies manually
func (ss *SchedulerService) ExecuteAllStrategiesManually() {
	ss.logger.Info("Starting manual execution of all backup strategies")

	if len(ss.config.Strategies) == 0 {
		ss.logger.Warn("No backup strategies configured")
		return
	}

	// Send initial Slack notification for all strategies
	strategyNames := make([]string, len(ss.config.Strategies))
	for i, strategy := range ss.config.Strategies {
		strategyNames[i] = strategy.Name
	}

	// Use the first strategy's Slack config or global config
	var slackConfig config.SlackConfig
	if len(ss.config.Strategies) > 0 {
		slackConfig = ss.config.Strategies[0].Slack
		if slackConfig.BotToken == "" {
			slackConfig = ss.config.Global.Slack
		}
	} else {
		slackConfig = ss.config.Global.Slack
	}

	thread, err := ss.slackService.SendBackupStarted(ss.ctx, strategyNames, slackConfig)
	if err != nil {
		ss.logger.WithError(err).Warn("Failed to send manual backup started notification")
	}

	// Execute each strategy
	successCount := 0
	failureCount := 0
	results := make(map[string]*backup.BackupResult)

	for _, strategy := range ss.config.Strategies {
		ss.logger.WithField("strategy", strategy.Name).Info("Starting manual backup execution")

		// Update strategy status
		ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
			Status:  "running",
			LastRun: time.Now().UTC().Format(time.RFC3339),
		})

		// Send progress update
		if thread != nil {
			progressMsg := fmt.Sprintf("Starting backup for strategy: %s", strategy.Name)
			if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, progressMsg); err != nil {
				ss.logger.WithError(err).Warn("Failed to send backup progress notification")
			}
		}

		// Execute backup with retry
		var result *backup.BackupResult
		var lastErr error

		for attempt := 1; attempt <= ss.config.Global.Retry.MaxAttempts; attempt++ {
			if attempt > 1 {
				ss.logger.WithFields(logrus.Fields{
					"strategy": strategy.Name,
					"attempt":  attempt,
				}).Info("Retrying manual backup")
				if thread != nil {
					retryMsg := fmt.Sprintf("Retrying backup for %s (attempt %d/%d)", strategy.Name, attempt, ss.config.Global.Retry.MaxAttempts)
					if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, retryMsg); err != nil {
						ss.logger.WithError(err).Warn("Failed to send backup progress notification")
					}
				}
			}

			result, lastErr = ss.backupService.ExecuteBackupWithProgress(ss.ctx, strategy, func(strategyName, message string) {
				// Send database output to Slack
				if thread != nil {
					if err := ss.slackService.SendDatabaseOutput(ss.ctx, thread, strategyName, message); err != nil {
						ss.logger.WithError(err).Warn("Failed to send database output to Slack")
					}
				}
			})
			if lastErr == nil {
				break
			}

			ss.logger.WithError(lastErr).WithFields(logrus.Fields{
				"strategy": strategy.Name,
				"attempt":  attempt,
			}).Warn("Manual backup attempt failed")

			// Send progress update about the failed attempt
			if thread != nil && attempt < ss.config.Global.Retry.MaxAttempts {
				failureMsg := fmt.Sprintf("Attempt %d/%d failed for %s: %s", attempt, ss.config.Global.Retry.MaxAttempts, strategy.Name, lastErr.Error())
				if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, failureMsg); err != nil {
					ss.logger.WithError(err).Warn("Failed to send backup progress notification")
				}
			}
		}

		if lastErr != nil {
			// All attempts failed
			failureCount++
			results[strategy.Name] = result
			ss.logger.WithError(lastErr).WithField("strategy", strategy.Name).Error("Manual backup failed after all attempts")

			// Update strategy status
			ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
				Status:  "failed",
				LastRun: time.Now().UTC().Format(time.RFC3339),
				NextRun: ss.getNextRunTime(strategy.Schedule),
				Error:   lastErr.Error(),
			})

			// Send failure notification
			if thread != nil {
				if err := ss.slackService.SendDetailedError(ss.ctx, thread, strategy.Name, result); err != nil {
					ss.logger.WithError(err).Warn("Failed to send backup failed notification")
				}
			}
			continue
		}

		// Backup successful, upload to S3
		if thread != nil {
			uploadMsg := fmt.Sprintf("Uploading %s backup to S3...", strategy.Name)
			if err := ss.slackService.SendBackupProgress(ss.ctx, thread, strategy.Name, uploadMsg); err != nil {
				ss.logger.WithError(err).Warn("Failed to send backup progress notification")
			}
		}

		s3Location, err := ss.s3Service.UploadBackup(ss.ctx, strategy.Name, result.BackupPath)
		if err != nil {
			ss.logger.WithError(err).WithField("strategy", strategy.Name).Error("Failed to upload manual backup to S3")
			failureCount++
			results[strategy.Name] = result

			// Update strategy status
			ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
				Status:  "failed",
				LastRun: time.Now().UTC().Format(time.RFC3339),
				NextRun: ss.getNextRunTime(strategy.Schedule),
				Error:   err.Error(),
			})

			// Send failure notification
			if thread != nil {
				if err := ss.slackService.SendDetailedError(ss.ctx, thread, strategy.Name, result); err != nil {
					ss.logger.WithError(err).Warn("Failed to send backup failed notification")
				}
			}
			continue
		}

		// Clean up local file
		if err := ss.backupService.CleanupTempFiles(result.BackupPath); err != nil {
			ss.logger.WithError(err).Warn("Failed to cleanup temporary files")
		}

		// Success
		successCount++
		results[strategy.Name] = result
		ss.logger.WithFields(logrus.Fields{
			"strategy":    strategy.Name,
			"size":        result.Size,
			"duration":    result.Duration,
			"s3_location": s3Location,
		}).Info("Manual backup completed successfully")

		// Update strategy status
		ss.monitoringService.UpdateStrategyStatus(strategy.Name, monitoring.StrategyStatus{
			Status:  "success",
			LastRun: time.Now().UTC().Format(time.RFC3339),
			NextRun: ss.getNextRunTime(strategy.Schedule),
		})
	}

	// Send final summary notification
	if thread != nil {
		var summaryResults []*backup.BackupResult
		for _, result := range results {
			if result != nil {
				summaryResults = append(summaryResults, result)
			}
		}

		if failureCount == 0 {
			// All successful
			if err := ss.slackService.SendBackupResult(ss.ctx, thread, summaryResults, true); err != nil {
				ss.logger.WithError(err).Warn("Failed to send backup success notification")
			}
		} else if successCount == 0 {
			// All failed
			if err := ss.slackService.SendBackupResult(ss.ctx, thread, summaryResults, false); err != nil {
				ss.logger.WithError(err).Warn("Failed to send backup failed notification")
			}
		} else {
			// Mixed results
			mixedMsg := fmt.Sprintf("Manual backup completed: %d successful, %d failed", successCount, failureCount)
			if err := ss.slackService.SendBackupProgress(ss.ctx, thread, "Summary", mixedMsg); err != nil {
				ss.logger.WithError(err).Warn("Failed to send backup summary notification")
			}
		}
	}

	ss.logger.WithFields(logrus.Fields{
		"total":      len(ss.config.Strategies),
		"successful": successCount,
		"failed":     failureCount,
	}).Info("Manual execution of all backup strategies completed")
}

// ExecuteStrategyManually executes a specific backup strategy manually
func (ss *SchedulerService) ExecuteStrategyManually(strategyName string) error {
	ss.logger.WithField("strategy", strategyName).Info("Starting manual execution of backup strategy")

	// Find the strategy
	var targetStrategy *config.StrategyConfig
	for _, strategy := range ss.config.Strategies {
		if strategy.Name == strategyName {
			targetStrategy = &strategy
			break
		}
	}

	if targetStrategy == nil {
		return fmt.Errorf("strategy '%s' not found", strategyName)
	}

	// Execute the backup job
	ss.executeBackupJob(*targetStrategy)
	return nil
}
