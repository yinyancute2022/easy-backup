package scheduler

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"easy-backup/internal/backup"
	"easy-backup/internal/config"
	"easy-backup/internal/monitoring"
	"easy-backup/internal/notification"
	"easy-backup/internal/storage"
)

func TestTimezoneConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		schedule string
		expected string // Expected timezone abbreviation or offset
		valid    bool
	}{
		{
			name:     "UTC timezone",
			timezone: "UTC",
			schedule: "0 2 * * *",
			expected: "UTC",
			valid:    true,
		},
		{
			name:     "New York timezone",
			timezone: "America/New_York",
			schedule: "0 9 * * *",
			expected: "EST", // or EDT depending on the date
			valid:    true,
		},
		{
			name:     "London timezone",
			timezone: "Europe/London",
			schedule: "0 14 * * *",
			expected: "GMT", // or BST depending on the date
			valid:    true,
		},
		{
			name:     "Tokyo timezone",
			timezone: "Asia/Tokyo",
			schedule: "0 3 * * *",
			expected: "JST",
			valid:    true,
		},
		{
			name:     "Invalid timezone falls back to UTC",
			timezone: "Invalid/Timezone",
			schedule: "0 2 * * *",
			expected: "UTC",
			valid:    false,
		},
		{
			name:     "Empty timezone defaults to UTC",
			timezone: "",
			schedule: "0 2 * * *",
			expected: "UTC",
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Global: config.GlobalConfig{
					Timezone:    tt.timezone,
					Schedule:    tt.schedule,
					MaxParallel: 1,
				},
			}

			// Create scheduler service
			scheduler := NewSchedulerService(
				cfg,
				&backup.BackupService{},
				&storage.S3Service{},
				&notification.SlackService{},
				&monitoring.MonitoringService{},
			)

			// Verify the cron scheduler was created with correct timezone
			require.NotNil(t, scheduler.cron)

			// Check if timezone was loaded correctly
			location := scheduler.cron.Location()
			require.NotNil(t, location)

			if tt.valid && tt.timezone != "" {
				// For valid timezones, check the location name
				if tt.timezone == "UTC" {
					assert.Equal(t, "UTC", location.String())
				} else {
					// For other timezones, verify it's not UTC (fallback)
					assert.NotEqual(t, "UTC", location.String())
					assert.Contains(t, location.String(), tt.timezone)
				}
			} else {
				// Invalid timezones should fall back to UTC
				assert.Equal(t, "UTC", location.String())
			}
		})
	}
}

func TestScheduleWithTimezone(t *testing.T) {
	tests := []struct {
		name            string
		timezone        string
		schedule        string
		expectedHour    int // Expected hour in the timezone
		expectValidCron bool
	}{
		{
			name:            "Daily 2 AM UTC",
			timezone:        "UTC",
			schedule:        "0 2 * * *",
			expectedHour:    2,
			expectValidCron: true,
		},
		{
			name:            "Daily 9 AM Eastern",
			timezone:        "America/New_York",
			schedule:        "0 9 * * *",
			expectedHour:    9,
			expectValidCron: true,
		},
		{
			name:            "Every 6 hours in Tokyo",
			timezone:        "Asia/Tokyo",
			schedule:        "0 */6 * * *",
			expectedHour:    0, // Will vary, but should be valid
			expectValidCron: true,
		},
		{
			name:            "Invalid cron expression",
			timezone:        "UTC",
			schedule:        "invalid cron",
			expectedHour:    0,
			expectValidCron: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load timezone
			location, err := time.LoadLocation(tt.timezone)
			if err != nil {
				location = time.UTC
			}

			// Create cron parser with the timezone
			cronScheduler := cron.New(cron.WithLocation(location))
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

			// Try to parse the schedule
			schedule, err := parser.Parse(tt.schedule)

			if tt.expectValidCron {
				require.NoError(t, err, "Expected valid cron expression")

				// Get next execution time
				now := time.Now().In(location)
				next := schedule.Next(now)

				// Verify the next execution is in the correct timezone
				assert.Equal(t, location, next.Location())

				// For specific hour tests, verify the hour matches
				if tt.schedule == "0 2 * * *" || tt.schedule == "0 9 * * *" {
					// For daily schedules, the hour should match
					expectedNext := schedule.Next(now)
					assert.Equal(t, tt.expectedHour, expectedNext.Hour())
				}
			} else {
				assert.Error(t, err, "Expected invalid cron expression to fail")
			}

			// Cleanup
			cronScheduler.Stop()
		})
	}
}

func TestDaylightSavingTimeHandling(t *testing.T) {
	// Test DST transitions for US Eastern Time
	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	// Create cron scheduler with Eastern timezone
	cronScheduler := cron.New(cron.WithLocation(location))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	// Parse a daily 2 AM schedule
	schedule, err := parser.Parse("0 2 * * *")
	require.NoError(t, err)

	// Test around DST transition dates (approximate - 2024 dates)
	testDates := []struct {
		name     string
		baseTime time.Time
		desc     string
	}{
		{
			name:     "Before DST (Standard Time)",
			baseTime: time.Date(2024, 3, 9, 12, 0, 0, 0, location), // Day before DST
			desc:     "Should schedule for 2 AM EST",
		},
		{
			name:     "During DST (Daylight Time)",
			baseTime: time.Date(2024, 7, 15, 12, 0, 0, 0, location), // During DST
			desc:     "Should schedule for 2 AM EDT",
		},
		{
			name:     "After DST (Standard Time)",
			baseTime: time.Date(2024, 12, 15, 12, 0, 0, 0, location), // After DST
			desc:     "Should schedule for 2 AM EST",
		},
	}

	for _, tt := range testDates {
		t.Run(tt.name, func(t *testing.T) {
			next := schedule.Next(tt.baseTime)

			// Verify the next execution is at 2 AM local time
			assert.Equal(t, 2, next.Hour(), "Should be scheduled for 2 AM local time")
			assert.Equal(t, location, next.Location(), "Should be in Eastern timezone")

			// The exact UTC offset will depend on whether it's DST or not
			_, offset := next.Zone()

			// EST is UTC-5 (-18000 seconds), EDT is UTC-4 (-14400 seconds)
			assert.True(t, offset == -18000 || offset == -14400,
				"Should be either EST (-5 hours) or EDT (-4 hours), got offset: %d seconds", offset)
		})
	}

	cronScheduler.Stop()
}

func TestTimezoneValidation(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timezone:    "Invalid/Timezone",
			MaxParallel: 1,
		},
	}

	// This should not panic and should fall back to UTC
	scheduler := NewSchedulerService(
		cfg,
		&backup.BackupService{},
		&storage.S3Service{},
		&notification.SlackService{},
		&monitoring.MonitoringService{},
	)

	// Verify fallback to UTC
	assert.Equal(t, "UTC", scheduler.cron.Location().String())
}

func TestNextRunTimeCalculation(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		schedule string
		baseTime time.Time
	}{
		{
			name:     "UTC daily backup",
			timezone: "UTC",
			schedule: "0 2 * * *",
			baseTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Eastern timezone daily backup",
			timezone: "America/New_York",
			schedule: "0 9 * * *",
			baseTime: time.Date(2024, 6, 15, 14, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Global: config.GlobalConfig{
					Timezone:    tt.timezone,
					MaxParallel: 1,
				},
			}

			scheduler := NewSchedulerService(
				cfg,
				&backup.BackupService{},
				&storage.S3Service{},
				&notification.SlackService{},
				&monitoring.MonitoringService{},
			)

			// Test the getNextRunTime method
			nextRunTime := scheduler.getNextRunTime(tt.schedule)

			// Should return a valid RFC3339 timestamp
			assert.NotEmpty(t, nextRunTime, "Next run time should not be empty")

			// Should be parseable as RFC3339
			_, err := time.Parse(time.RFC3339, nextRunTime)
			assert.NoError(t, err, "Next run time should be valid RFC3339 format")
		})
	}
}
