package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBufferPool(t *testing.T) {
	bp := NewBufferPool()
	require.NotNil(t, bp)
	assert.NotNil(t, bp.small)
	assert.NotNil(t, bp.medium)
	assert.NotNil(t, bp.large)
}

func TestBufferPool_GetSmall(t *testing.T) {
	bp := NewBufferPool()

	buf := bp.GetSmall()
	require.NotNil(t, buf)
	assert.Equal(t, SmallBufferSize, cap(buf))
	assert.Equal(t, 0, len(buf))

	// Use the buffer
	buf = append(buf, []byte("test data")...)
	assert.Equal(t, 9, len(buf))

	// Return to pool
	bp.PutSmall(buf)
}

func TestBufferPool_GetMedium(t *testing.T) {
	bp := NewBufferPool()

	buf := bp.GetMedium()
	require.NotNil(t, buf)
	assert.Equal(t, MediumBufferSize, cap(buf))
	assert.Equal(t, 0, len(buf))

	// Use the buffer
	buf = append(buf, []byte("test data")...)
	assert.Equal(t, 9, len(buf))

	// Return to pool
	bp.PutMedium(buf)
}

func TestBufferPool_GetLarge(t *testing.T) {
	bp := NewBufferPool()

	buf := bp.GetLarge()
	require.NotNil(t, buf)
	assert.Equal(t, LargeBufferSize, cap(buf))
	assert.Equal(t, 0, len(buf))

	// Use the buffer
	buf = append(buf, []byte("test data")...)
	assert.Equal(t, 9, len(buf))

	// Return to pool
	bp.PutLarge(buf)
}

func TestBufferPool_GetBuffer(t *testing.T) {
	bp := NewBufferPool()

	tests := []struct {
		name     string
		size     int
		expected int
	}{
		{"small size", 1000, SmallBufferSize},
		{"medium size", 10000, MediumBufferSize},
		{"large size", 100000, LargeBufferSize},
		{"very large size", LargeBufferSize * 2, LargeBufferSize * 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bp.GetBuffer(tt.size)
			require.NotNil(t, buf)
			assert.Equal(t, tt.expected, cap(buf))
			assert.Equal(t, 0, len(buf))

			bp.PutBuffer(buf)
		})
	}
}

func TestBufferPool_BufferReuse(t *testing.T) {
	bp := NewBufferPool()

	// Get and return a buffer
	buf1 := bp.GetSmall()
	buf1 = append(buf1, []byte("first use")...)
	bp.PutSmall(buf1)

	// Get another buffer - should reuse the same underlying memory
	buf2 := bp.GetSmall()
	assert.Equal(t, SmallBufferSize, cap(buf2))
	assert.Equal(t, 0, len(buf2)) // Should be reset

	bp.PutSmall(buf2)
}

func TestGlobalBufferPool(t *testing.T) {
	// Test global functions
	buf := GetSmallBuffer()
	require.NotNil(t, buf)
	assert.Equal(t, SmallBufferSize, cap(buf))

	PutSmallBuffer(buf)

	buf = GetMediumBuffer()
	require.NotNil(t, buf)
	assert.Equal(t, MediumBufferSize, cap(buf))

	PutMediumBuffer(buf)

	buf = GetLargeBuffer()
	require.NotNil(t, buf)
	assert.Equal(t, LargeBufferSize, cap(buf))

	PutLargeBuffer(buf)

	buf = GetBuffer(1000)
	require.NotNil(t, buf)
	assert.Equal(t, SmallBufferSize, cap(buf))

	PutBuffer(buf)
}

func BenchmarkBufferPool_GetPutSmall(b *testing.B) {
	bp := NewBufferPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bp.GetSmall()
			bp.PutSmall(buf)
		}
	})
}

func BenchmarkBufferPool_GetPutMedium(b *testing.B) {
	bp := NewBufferPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bp.GetMedium()
			bp.PutMedium(buf)
		}
	})
}

func BenchmarkBufferPool_GetPutLarge(b *testing.B) {
	bp := NewBufferPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bp.GetLarge()
			bp.PutLarge(buf)
		}
	})
}

func BenchmarkBufferAllocation_NewEachTime(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := make([]byte, SmallBufferSize)
			_ = buf
		}
	})
}
