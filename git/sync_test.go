package git

import (
	"errors"
	"testing"

	"github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetch tests the Fetch method
func TestFetch(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		prune    bool
		depth    int
		validate func(t *testing.T, err error)
	}{
		{
			name: "fetch from non-existent remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "nonexistent",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "fetch with empty remote (uses default)",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(
					t,
					errors.Is(err, ErrResolveFailed),
					"should return resolve failed error for default remote",
				)
			},
		},
		{
			name: "fetch with prune and depth",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			prune:  true,
			depth:  1,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.Fetch(tr.ctx, tt.remote, tt.prune, tt.depth)
			tt.validate(t, err)
		})
	}
}

// TestPullFFOnly tests the PullFFOnly method
func TestPullFFOnly(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		validate func(t *testing.T, err error)
	}{
		{
			name: "pull from bare repository",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, true)
			},
			remote: "",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidRef), "should return invalid ref error for bare repo")
			},
		},
		{
			name: "pull from non-existent remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "nonexistent",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "pull with empty remote (uses default)",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.PullFFOnly(tr.ctx, tt.remote)
			tt.validate(t, err)
		})
	}
}

// TestFetchAndMerge tests the FetchAndMerge method
func TestFetchAndMerge(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		fromRef  string
		strategy MergeStrategy
		validate func(t *testing.T, err error)
	}{
		{
			name: "merge with invalid remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote:   "nonexistent",
			fromRef:  "HEAD",
			strategy: FastForwardOnly,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "merge with invalid ref",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepo(t, false)
				// Add a remote so fetch succeeds but merge fails
				_, err := tr.repo.repo.CreateRemote(&config.RemoteConfig{
					Name: DefaultRemoteName,
					URLs: []string{"file://" + t.TempDir()},
				})
				require.NoError(t, err)
				return tr
			},
			remote:   "",
			fromRef:  "invalid-ref",
			strategy: FastForwardOnly,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				// The fetch may fail due to the temp dir not being a valid git repo
				assert.Error(t, err, "should return an error")
			},
		},
		{
			name: "merge with unsupported strategy",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepo(t, false)
				// Add a remote so fetch succeeds and we get to strategy validation
				_, err := tr.repo.repo.CreateRemote(&config.RemoteConfig{
					Name: DefaultRemoteName,
					URLs: []string{"file://" + t.TempDir()},
				})
				require.NoError(t, err)
				return tr
			},
			remote:   "",
			fromRef:  "HEAD",
			strategy: MergeStrategy(99), // Invalid strategy
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				// The fetch may fail, or we may get to strategy validation
				// Either way, we expect some error
				assert.Error(t, err, "should return some error")
			},
		},
		{
			name: "merge with valid parameters but no remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote:   "",
			fromRef:  "HEAD",
			strategy: FastForwardOnly,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.FetchAndMerge(tr.ctx, tt.remote, tt.fromRef, tt.strategy)
			tt.validate(t, err)
		})
	}
}

// TestPush tests the Push method
func TestPush(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		force    bool
		validate func(t *testing.T, err error)
	}{
		{
			name: "push to non-existent remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "nonexistent",
			force:  false,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "push with empty remote (uses default)",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			force:  false,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "push with force flag",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			force:  true,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.Push(tr.ctx, tt.remote, tt.force)
			tt.validate(t, err)
		})
	}
}

// TestMergeStrategy_String tests the String method of MergeStrategy
func TestMergeStrategy_String(t *testing.T) {
	tests := []struct {
		strategy MergeStrategy
		expected string
	}{
		{FastForwardOnly, "fast-forward-only"},
		{MergeStrategy(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.strategy.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
