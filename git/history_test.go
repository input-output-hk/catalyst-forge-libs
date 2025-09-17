package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLog tests the Log method with various filters
func TestLog(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *testRepo
		filter      LogFilter
		expectError bool
		validate    func(t *testing.T, iter *CommitIter, err error)
	}{
		{
			name:        "basic log without filters",
			setup:       setupTestRepoWithCommit,
			filter:      LogFilter{},
			expectError: false,
			validate: func(t *testing.T, iter *CommitIter, err error) {
				require.NoError(t, err)
				require.NotNil(t, iter)

				// Should have at least one commit
				commit, err := iter.Next()
				require.NoError(t, err)
				require.NotNil(t, commit)
				assert.Equal(t, "Initial commit", commit.Message)

				// Should be end of iteration
				nextCommit, err := iter.Next()
				require.NoError(t, err)
				assert.Nil(t, nextCommit)

				iter.Close()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)

			ctx := context.Background()
			iter, err := tr.repo.Log(ctx, tt.filter)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			tt.validate(t, iter, err)
		})
	}
}
