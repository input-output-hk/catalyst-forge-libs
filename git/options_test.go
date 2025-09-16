package git

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr error
	}{
		{
			name:    "valid options",
			options: Options{FS: &mockFilesystem{}},
			wantErr: nil,
		},
		{
			name:    "nil filesystem",
			options: Options{FS: nil},
			wantErr: ErrInvalidRef,
		},
		{
			name: "negative cache size",
			options: Options{
				FS:              &mockFilesystem{},
				StorerCacheSize: -1,
			},
			wantErr: ErrInvalidRef,
		},
		{
			name: "negative shallow depth",
			options: Options{
				FS:           &mockFilesystem{},
				ShallowDepth: -1,
			},
			wantErr: ErrInvalidRef,
		},
		{
			name: "zero values are valid",
			options: Options{
				FS:              &mockFilesystem{},
				StorerCacheSize: 0,
				ShallowDepth:    0,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestOptions_applyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    Options
		expected Options
	}{
		{
			name: "empty options gets defaults",
			input: Options{
				FS: &mockFilesystem{},
			},
			expected: Options{
				FS:              &mockFilesystem{},
				Workdir:         DefaultWorkdir,
				StorerCacheSize: DefaultStorerCacheSize,
				HTTPClient:      &http.Client{Timeout: 30 * time.Second},
			},
		},
		{
			name: "custom workdir preserved",
			input: Options{
				FS:      &mockFilesystem{},
				Workdir: "/custom",
			},
			expected: Options{
				FS:              &mockFilesystem{},
				Workdir:         "/custom",
				StorerCacheSize: DefaultStorerCacheSize,
				HTTPClient:      &http.Client{Timeout: 30 * time.Second},
			},
		},
		{
			name: "custom cache size preserved",
			input: Options{
				FS:              &mockFilesystem{},
				StorerCacheSize: 500,
			},
			expected: Options{
				FS:              &mockFilesystem{},
				Workdir:         DefaultWorkdir,
				StorerCacheSize: 500,
				HTTPClient:      &http.Client{Timeout: 30 * time.Second},
			},
		},
		{
			name: "custom http client preserved",
			input: Options{
				FS:         &mockFilesystem{},
				HTTPClient: &http.Client{Timeout: 60 * time.Second},
			},
			expected: Options{
				FS:              &mockFilesystem{},
				Workdir:         DefaultWorkdir,
				StorerCacheSize: DefaultStorerCacheSize,
				HTTPClient:      &http.Client{Timeout: 60 * time.Second},
			},
		},
		{
			name: "all custom values preserved",
			input: Options{
				FS:              &mockFilesystem{},
				Workdir:         "/repo",
				Bare:            true,
				StorerCacheSize: 2000,
				HTTPClient:      &http.Client{Timeout: 120 * time.Second},
				ShallowDepth:    5,
			},
			expected: Options{
				FS:              &mockFilesystem{},
				Workdir:         "/repo",
				Bare:            true,
				StorerCacheSize: 2000,
				HTTPClient:      &http.Client{Timeout: 120 * time.Second},
				ShallowDepth:    5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.applyDefaults()

			assert.Equal(t, tt.expected.Workdir, tt.input.Workdir, "Workdir should match")
			assert.Equal(t, tt.expected.StorerCacheSize, tt.input.StorerCacheSize, "StorerCacheSize should match")
			assert.Equal(t, tt.expected.Bare, tt.input.Bare, "Bare should match")
			assert.Equal(t, tt.expected.ShallowDepth, tt.input.ShallowDepth, "ShallowDepth should match")

			if tt.expected.HTTPClient != nil {
				require.NotNil(t, tt.input.HTTPClient, "HTTPClient should not be nil")
				assert.Equal(t, tt.expected.HTTPClient.Timeout, tt.input.HTTPClient.Timeout,
					"HTTPClient.Timeout should match")
			}
		})
	}
}

func TestRefKind_String(t *testing.T) {
	tests := []struct {
		name     string
		kind     RefKind
		expected string
	}{
		{"branch", RefBranch, "branch"},
		{"remote-branch", RefRemoteBranch, "remote-branch"},
		{"tag", RefTag, "tag"},
		{"remote", RefRemote, "remote"},
		{"commit", RefCommit, "commit"},
		{"other", RefOther, "other"},
		{"unknown", RefKind(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kind.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
