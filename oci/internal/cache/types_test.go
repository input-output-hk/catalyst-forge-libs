package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				MaxSizeBytes: 100 * 1024 * 1024, // 100MB
				DefaultTTL:   5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "zero max size",
			config: Config{
				MaxSizeBytes: 0,
				DefaultTTL:   5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "max size must be greater than 0",
		},
		{
			name: "negative max size",
			config: Config{
				MaxSizeBytes: -1,
				DefaultTTL:   5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "max size must be greater than 0",
		},
		{
			name: "zero default TTL",
			config: Config{
				MaxSizeBytes: 100 * 1024 * 1024,
				DefaultTTL:   0,
			},
			expectError: true,
			errorMsg:    "default TTL must be greater than 0",
		},
		{
			name: "negative default TTL",
			config: Config{
				MaxSizeBytes: 100 * 1024 * 1024,
				DefaultTTL:   -1 * time.Minute,
			},
			expectError: true,
			errorMsg:    "default TTL must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	config := Config{
		MaxSizeBytes: 100 * 1024 * 1024,
		DefaultTTL:   5 * time.Minute,
	}

	// Apply defaults
	config.SetDefaults()

	// No defaults to set currently since compression has been removed
	assert.Equal(t, int64(100*1024*1024), config.MaxSizeBytes)
	assert.Equal(t, 5*time.Minute, config.DefaultTTL)
}

func TestEntry_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		entry    Entry
		expected bool
	}{
		{
			name: "not expired",
			entry: Entry{
				CreatedAt: now.Add(-1 * time.Minute),
				TTL:       5 * time.Minute,
			},
			expected: false,
		},
		{
			name: "expired",
			entry: Entry{
				CreatedAt: now.Add(-10 * time.Minute),
				TTL:       5 * time.Minute,
			},
			expected: true,
		},
		{
			name: "zero TTL never expires",
			entry: Entry{
				CreatedAt: now.Add(-1 * time.Hour),
				TTL:       0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.IsExpired()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEntry_Size(t *testing.T) {
	entry := Entry{
		Key:         "test-key",
		Data:        []byte("test data content"),
		Metadata:    map[string]string{"type": "test"},
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         5 * time.Minute,
		AccessCount: 10,
	}

	size := entry.Size()

	// Size should be greater than just data length due to metadata overhead
	assert.Greater(t, size, int64(len(entry.Data)))
	assert.Greater(t, size, int64(0))
}

func TestMetrics_RecordHit(t *testing.T) {
	metrics := &Metrics{}

	metrics.RecordHit()

	assert.Equal(t, int64(1), metrics.Hits)
	assert.Equal(t, int64(0), metrics.Misses)
	assert.Equal(t, float64(1.0), metrics.HitRate())
}

func TestMetrics_RecordMiss(t *testing.T) {
	metrics := &Metrics{}

	metrics.RecordMiss()

	assert.Equal(t, int64(0), metrics.Hits)
	assert.Equal(t, int64(1), metrics.Misses)
	assert.Equal(t, float64(0.0), metrics.HitRate())
}

func TestMetrics_RecordEviction(t *testing.T) {
	metrics := &Metrics{}

	metrics.RecordEviction()

	assert.Equal(t, int64(1), metrics.Evictions)
}

func TestMetrics_RecordError(t *testing.T) {
	metrics := &Metrics{}

	metrics.RecordError()

	assert.Equal(t, int64(1), metrics.Errors)
}

func TestMetrics_HitRate(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{
			name:     "no requests",
			hits:     0,
			misses:   0,
			expected: 0.0,
		},
		{
			name:     "only hits",
			hits:     10,
			misses:   0,
			expected: 1.0,
		},
		{
			name:     "only misses",
			hits:     0,
			misses:   5,
			expected: 0.0,
		},
		{
			name:     "mixed hits and misses",
			hits:     7,
			misses:   3,
			expected: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &Metrics{
				Hits:   tt.hits,
				Misses: tt.misses,
			}

			rate := metrics.HitRate()
			assert.Equal(t, tt.expected, rate)
		})
	}
}

func TestMetrics_AddBytesStored(t *testing.T) {
	metrics := &Metrics{}

	metrics.AddBytesStored(1024)
	assert.Equal(t, int64(1024), metrics.BytesStored)

	metrics.AddBytesStored(512)
	assert.Equal(t, int64(1536), metrics.BytesStored)
}

func TestMetrics_RemoveBytesStored(t *testing.T) {
	metrics := &Metrics{
		BytesStored: 2048,
	}

	metrics.RemoveBytesStored(1024)
	assert.Equal(t, int64(1024), metrics.BytesStored)

	metrics.RemoveBytesStored(512)
	assert.Equal(t, int64(512), metrics.BytesStored)
}

func TestManager_New(t *testing.T) {
	config := Config{
		MaxSizeBytes: 100 * 1024 * 1024,
		DefaultTTL:   5 * time.Minute,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)
	require.NotNil(t, manager)

	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.metrics)
	assert.Greater(t, manager.createdAt.Unix(), int64(0))
}

func TestManager_New_InvalidConfig(t *testing.T) {
	config := Config{
		MaxSizeBytes: 0, // Invalid
		DefaultTTL:   5 * time.Minute,
	}

	manager, err := NewManager(config)
	require.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "max size must be greater than 0")
}
