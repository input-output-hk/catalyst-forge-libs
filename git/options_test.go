package git

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	gitfs "github.com/input-output-hk/catalyst-forge-libs/fs"
)

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name     string
		options  Options
		expected error
	}{
		{
			name:     "valid options",
			options:  Options{FS: &mockFilesystem{}},
			expected: nil,
		},
		{
			name: "nil filesystem",
			options: Options{
				FS: nil,
			},
			expected: ErrInvalidRef,
		},
		{
			name: "negative cache size",
			options: Options{
				FS:              &mockFilesystem{},
				StorerCacheSize: -1,
			},
			expected: ErrInvalidRef,
		},
		{
			name: "negative shallow depth",
			options: Options{
				FS:           &mockFilesystem{},
				ShallowDepth: -1,
			},
			expected: ErrInvalidRef,
		},
		{
			name: "zero values are valid",
			options: Options{
				FS:              &mockFilesystem{},
				StorerCacheSize: 0,
				ShallowDepth:    0,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.expected == nil {
				if err != nil {
					t.Errorf("Validate() = %v; want nil", err)
				}
				return
			}

			if err == nil {
				t.Errorf("Validate() = nil; want %v", tt.expected)
				return
			}

			if !IsSentinelError(err, tt.expected) {
				t.Errorf("Validate() = %v; want error wrapping %v", err, tt.expected)
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

			// Compare fields individually since we can't compare function pointers
			if tt.input.Workdir != tt.expected.Workdir {
				t.Errorf("Workdir = %q; want %q", tt.input.Workdir, tt.expected.Workdir)
			}

			if tt.input.StorerCacheSize != tt.expected.StorerCacheSize {
				t.Errorf("StorerCacheSize = %d; want %d", tt.input.StorerCacheSize, tt.expected.StorerCacheSize)
			}

			if tt.input.HTTPClient.Timeout != tt.expected.HTTPClient.Timeout {
				t.Errorf("HTTPClient.Timeout = %v; want %v",
					tt.input.HTTPClient.Timeout, tt.expected.HTTPClient.Timeout)
			}
		})
	}
}

func TestRefKind_String(t *testing.T) {
	tests := []struct {
		kind     RefKind
		expected string
	}{
		{RefBranch, "branch"},
		{RefRemoteBranch, "remote-branch"},
		{RefTag, "tag"},
		{RefRemote, "remote"},
		{RefCommit, "commit"},
		{RefOther, "other"},
		{RefKind(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.kind.String()
			if result != tt.expected {
				t.Errorf("String() = %q; want %q", result, tt.expected)
			}
		})
	}
}

// mockFilesystem implements fs.Filesystem for testing.
// Note: ireturn warnings are expected for mock implementations.
type mockFilesystem struct{}

//nolint:ireturn // mock implementations return interfaces
func (m *mockFilesystem) Create(name string) (gitfs.File, error) { return nil, nil }

func (m *mockFilesystem) Exists(path string) (bool, error)             { return true, nil }
func (m *mockFilesystem) MkdirAll(path string, perm os.FileMode) error { return nil }

//nolint:ireturn // mock implementations return interfaces
func (m *mockFilesystem) Open(name string) (gitfs.File, error) { return nil, nil }

//nolint:ireturn // mock implementations return interfaces
func (m *mockFilesystem) OpenFile(name string, flag int, perm os.FileMode) (gitfs.File, error) {
	return nil, nil
}
func (m *mockFilesystem) ReadDir(dirname string) ([]os.FileInfo, error)                  { return nil, nil }
func (m *mockFilesystem) ReadFile(path string) ([]byte, error)                           { return nil, nil }
func (m *mockFilesystem) Remove(name string) error                                       { return nil }
func (m *mockFilesystem) Rename(oldpath, newpath string) error                           { return nil }
func (m *mockFilesystem) Stat(name string) (os.FileInfo, error)                          { return nil, nil }
func (m *mockFilesystem) TempDir(dir, prefix string) (string, error)                     { return "", nil }
func (m *mockFilesystem) Walk(root string, walkFn filepath.WalkFunc) error               { return nil }
func (m *mockFilesystem) WriteFile(filename string, data []byte, perm os.FileMode) error { return nil }
func (m *mockFilesystem) Symlink(oldname, newname string) error                          { return nil }

// IsSentinelError checks if an error wraps a specific sentinel error.
func IsSentinelError(err, sentinel error) bool {
	return errors.Is(err, sentinel)
}
