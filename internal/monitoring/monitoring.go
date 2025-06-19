package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"easy-backup/internal/config"
	"easy-backup/internal/logger"
	"easy-backup/internal/notification"
	"easy-backup/internal/storage"
)

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Status            string         `json:"status"`
	Timestamp         string         `json:"timestamp"`
	Version           string         `json:"version"`
	Strategies        StrategyHealth `json:"strategies"`
	S3Connectivity    string         `json:"s3_connectivity"`
	SlackConnectivity string         `json:"slack_connectivity"`
}

// StrategyHealth represents the health status of backup strategies
type StrategyHealth struct {
	Total            int                       `json:"total"`
	LastBackupStatus map[string]StrategyStatus `json:"last_backup_status"`
}

// StrategyStatus represents the status of a single strategy
type StrategyStatus struct {
	Status  string `json:"status"`
	LastRun string `json:"last_run,omitempty"`
	NextRun string `json:"next_run,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MonitoringService handles health checks and metrics
type MonitoringService struct {
	config         *config.Config
	logger         *logrus.Logger
	s3Service      *storage.S3Service
	slackService   *notification.SlackService
	strategyStatus map[string]StrategyStatus
	statusMutex    sync.RWMutex

	// Prometheus metrics
	backupDuration *prometheus.HistogramVec
	backupSize     *prometheus.GaugeVec
	backupSuccess  *prometheus.CounterVec
	backupFailures *prometheus.CounterVec
	lastBackupTime *prometheus.GaugeVec
}

// NewMonitoringService creates a new monitoring service
func NewMonitoringService(cfg *config.Config, s3Service *storage.S3Service, slackService *notification.SlackService) *MonitoringService {
	// Create Prometheus metrics
	backupDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "backup_duration_seconds",
			Help: "Duration of backup operations in seconds",
		},
		[]string{"strategy"},
	)

	backupSize := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "backup_size_bytes",
			Help: "Size of backup files in bytes",
		},
		[]string{"strategy"},
	)

	backupSuccess := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "backup_success_total",
			Help: "Total number of successful backups",
		},
		[]string{"strategy"},
	)

	backupFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "backup_failures_total",
			Help: "Total number of failed backups",
		},
		[]string{"strategy"},
	)

	lastBackupTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "backup_last_time_seconds",
			Help: "Timestamp of the last backup",
		},
		[]string{"strategy"},
	)

	// Register metrics
	prometheus.MustRegister(backupDuration, backupSize, backupSuccess, backupFailures, lastBackupTime)

	return &MonitoringService{
		config:         cfg,
		logger:         logger.GetLogger(),
		s3Service:      s3Service,
		slackService:   slackService,
		strategyStatus: make(map[string]StrategyStatus),
		backupDuration: backupDuration,
		backupSize:     backupSize,
		backupSuccess:  backupSuccess,
		backupFailures: backupFailures,
		lastBackupTime: lastBackupTime,
	}
}

// StartHTTPServer starts the HTTP server for health checks and metrics
func (ms *MonitoringService) StartHTTPServer() error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc(ms.config.Global.Monitoring.HealthCheck.Path, ms.healthCheckHandler)

	// Metrics endpoint (if enabled)
	if ms.config.Global.Monitoring.Metrics.Enabled {
		mux.Handle(ms.config.Global.Monitoring.Metrics.Path, promhttp.Handler())
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", ms.config.Global.Monitoring.HealthCheck.Port),
		Handler: mux,
	}

	ms.logger.WithField("port", ms.config.Global.Monitoring.HealthCheck.Port).Info("Starting monitoring HTTP server")

	return server.ListenAndServe()
}

// healthCheckHandler handles health check requests
func (ms *MonitoringService) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check external services
	s3Status := "ok"
	if err := ms.s3Service.TestConnection(ctx); err != nil {
		s3Status = "error"
		ms.logger.WithError(err).Warn("S3 health check failed")
	}

	slackStatus := "ok"
	if err := ms.slackService.TestConnection(ctx); err != nil {
		// Check if it's a scope issue vs a real connectivity issue
		if strings.Contains(err.Error(), "missing_scope") {
			slackStatus = "limited" // New status for scope limitations
			ms.logger.WithField("status", "limited").Debug("Slack health check shows limited permissions - basic functionality should work")
		} else {
			slackStatus = "error"
			ms.logger.WithError(err).Warn("Slack health check failed")
		}
	}

	// Determine overall health
	overallStatus := "healthy"
	if s3Status == "error" || slackStatus == "error" {
		overallStatus = "degraded"
	} else if slackStatus == "limited" {
		overallStatus = "healthy" // Limited Slack permissions don't degrade overall health
	}

	// Get strategy status
	ms.statusMutex.RLock()
	strategyStatusCopy := make(map[string]StrategyStatus)
	for k, v := range ms.strategyStatus {
		strategyStatusCopy[k] = v
	}
	ms.statusMutex.RUnlock()

	health := HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   "1.0.0", // You might want to make this configurable
		Strategies: StrategyHealth{
			Total:            len(ms.config.Strategies),
			LastBackupStatus: strategyStatusCopy,
		},
		S3Connectivity:    s3Status,
		SlackConnectivity: slackStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(health); err != nil {
		ms.logger.WithError(err).Error("Failed to encode health check response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// UpdateStrategyStatus updates the status of a backup strategy
func (ms *MonitoringService) UpdateStrategyStatus(strategy string, status StrategyStatus) {
	ms.statusMutex.Lock()
	defer ms.statusMutex.Unlock()
	ms.strategyStatus[strategy] = status
}

// RecordBackupMetrics records metrics for a backup operation
func (ms *MonitoringService) RecordBackupMetrics(strategy string, duration time.Duration, size int64, success bool) {
	if success {
		ms.backupSuccess.WithLabelValues(strategy).Inc()
		ms.backupDuration.WithLabelValues(strategy).Observe(duration.Seconds())
		ms.backupSize.WithLabelValues(strategy).Set(float64(size))
		ms.lastBackupTime.WithLabelValues(strategy).Set(float64(time.Now().Unix()))
	} else {
		ms.backupFailures.WithLabelValues(strategy).Inc()
	}
}
