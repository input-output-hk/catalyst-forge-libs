package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainsBinaryFiles(t *testing.T) {
	tests := []struct {
		name      string
		patchText string
		expected  bool
	}{
		{
			name: "text only patch",
			patchText: `diff --git a/file.txt b/file.txt
index 1234567..abcdefg 100644
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old
+new`,
			expected: false,
		},
		{
			name: "binary files differ",
			patchText: `diff --git a/image.png b/image.png
index 1234567..abcdefg 100644
Binary files differ`,
			expected: true,
		},
		{
			name: "git binary patch",
			patchText: `diff --git a/binary.bin b/binary.bin
GIT binary patch
literal 10
abcdefghij`,
			expected: true,
		},
		{
			name:      "empty patch",
			patchText: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsBinaryFiles(tt.patchText)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountChangedFiles(t *testing.T) {
	tests := []struct {
		name      string
		patchText string
		expected  int
	}{
		{
			name: "single file",
			patchText: `diff --git a/file.txt b/file.txt
index 1234567..abcdefg 100644
--- a/file.txt
+++ b/file.txt`,
			expected: 1,
		},
		{
			name: "multiple files",
			patchText: `diff --git a/file1.txt b/file1.txt
index 1234567..abcdefg 100644
--- a/file1.txt
+++ b/file1.txt
diff --git a/file2.go b/file2.go
index 2345678..bcdefgh 100644
--- a/file2.go
+++ b/file2.go`,
			expected: 2,
		},
		{
			name:      "empty patch",
			patchText: "",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountChangedFiles(tt.patchText)
			require.Equal(t, tt.expected, result)
		})
	}
}
