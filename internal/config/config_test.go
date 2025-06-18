package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{"1 hour", "1h", "1h0m0s", false},
		{"1 day", "1d", "24h0m0s", false},
		{"1 week", "1w", "168h0m0s", false},
		{"30 minutes", "30m", "30m0s", false},
		{"invalid format", "1x", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := ParseDuration(tt.input)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, duration.String())
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	config := &Config{
		Global: GlobalConfig{},
		Strategies: []StrategyConfig{
			{Name: "test-db"},
		},
	}

	err := setDefaults(config)
	require.NoError(t, err)

	// Check global defaults
	assert.Equal(t, "info", config.Global.LogLevel)
	assert.Equal(t, "1d", config.Global.Schedule)
	assert.Equal(t, "30d", config.Global.Retention)
	assert.Equal(t, "UTC", config.Global.Timezone)
	assert.Equal(t, "/tmp/db-backup", config.Global.TempDir)
	assert.Equal(t, 2, config.Global.MaxParallel)
	assert.Equal(t, 3, config.Global.Retry.MaxAttempts)

	// Check strategy inherits defaults
	assert.Equal(t, "1d", config.Strategies[0].Schedule)
	assert.Equal(t, "30d", config.Strategies[0].Retention)
}
