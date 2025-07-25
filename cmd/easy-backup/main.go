package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"easy-backup/internal/backup"
	"easy-backup/internal/config"
	"easy-backup/internal/logger"
	"easy-backup/internal/monitoring"
	"easy-backup/internal/notification"
	"easy-backup/internal/scheduler"
	"easy-backup/internal/storage"
)

const (
	defaultConfigPath = "config.yaml"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	manualTrigger := flag.Bool("manual", false, "Execute all backup strategies manually and exit")
	manualStrategy := flag.String("strategy", "", "Execute a specific backup strategy manually and exit")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	if err := logger.InitLogger(cfg.Global.LogLevel); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	log := logger.GetLogger()
	log.Info("Starting Easy Backup service")

	// Initialize services
	backupService := backup.NewBackupService(cfg)

	s3Service, err := storage.NewS3Service(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize S3 service: %v", err)
	}

	slackService := notification.NewSlackService(cfg)

	monitoringService := monitoring.NewMonitoringService(cfg, s3Service, slackService)

	schedulerService := scheduler.NewSchedulerService(
		cfg,
		backupService,
		s3Service,
		slackService,
		monitoringService,
	)

	// Handle manual trigger modes
	if *manualTrigger {
		log.Info("Manual trigger mode: executing all backup strategies")
		schedulerService.ExecuteAllStrategiesManually()
		log.Info("Manual execution completed, exiting")
		return
	}

	if *manualStrategy != "" {
		log.WithField("strategy", *manualStrategy).Info("Manual trigger mode: executing specific backup strategy")
		if err := schedulerService.ExecuteStrategyManually(*manualStrategy); err != nil {
			log.Fatalf("Failed to execute strategy manually: %v", err)
		}
		log.Info("Manual execution completed, exiting")
		return
	}

	// Create context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start monitoring HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := monitoringService.StartHTTPServer(); err != nil {
			log.WithError(err).Error("Monitoring HTTP server failed")
		}
	}()

	// Start scheduler
	if err := schedulerService.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	log.Info("Easy Backup service started successfully")

	// Execute all strategies on startup if configured
	if cfg.Global.ExecuteOnStartup {
		log.Info("ExecuteOnStartup is enabled, triggering all backup strategies immediately")
		go func() {
			// Give the service a moment to fully initialize
			time.Sleep(2 * time.Second)
			schedulerService.ExecuteAllStrategiesManually()
		}()
	}

	// Wait for shutdown signal
	<-sigChan
	log.Info("Received shutdown signal, gracefully shutting down...")

	// Cancel context to signal shutdown
	cancel()

	// Stop scheduler
	schedulerService.Stop()

	// Wait for all goroutines to finish
	wg.Wait()

	log.Info("Easy Backup service stopped")
}
