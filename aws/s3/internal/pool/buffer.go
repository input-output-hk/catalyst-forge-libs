// Package pool provides memory management optimizations.
// This includes buffer pooling and resource reuse to reduce allocations.
//
// The pool package helps optimize performance for high-throughput operations
// by reusing expensive resources like buffers and connections.
package pool

import (
	"sync"
)

const (
	// SmallBufferSize defines the size for small buffers (4KB)
	SmallBufferSize = 4 * 1024
	// MediumBufferSize defines the size for medium buffers (64KB)
	MediumBufferSize = 64 * 1024
	// LargeBufferSize defines the size for large buffers (1MB)
	LargeBufferSize = 1024 * 1024
)

// BufferPool manages reusable buffers of different sizes to reduce allocations.
type BufferPool struct {
	small  *sync.Pool
	medium *sync.Pool
	large  *sync.Pool
}

// NewBufferPool creates a new buffer pool with default sizes.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		small: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, SmallBufferSize)
				return &buf
			},
		},
		medium: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, MediumBufferSize)
				return &buf
			},
		},
		large: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, LargeBufferSize)
				return &buf
			},
		},
	}
}

// GetSmall returns a small buffer from the pool.
// The caller is responsible for calling PutSmall to return the buffer to the pool.
func (bp *BufferPool) GetSmall() []byte {
	bufPtr := bp.small.Get().(*[]byte)
	// Reset length to 0 but keep capacity
	*bufPtr = (*bufPtr)[:0]
	return *bufPtr
}

// PutSmall returns a small buffer to the pool.
// The buffer should not be used after calling PutSmall.
func (bp *BufferPool) PutSmall(buf []byte) {
	// Reset buffer length but keep capacity
	buf = buf[:0]
	bp.small.Put(&buf)
}

// GetMedium returns a medium buffer from the pool.
// The caller is responsible for calling PutMedium to return the buffer to the pool.
func (bp *BufferPool) GetMedium() []byte {
	bufPtr := bp.medium.Get().(*[]byte)
	// Reset length to 0 but keep capacity
	*bufPtr = (*bufPtr)[:0]
	return *bufPtr
}

// PutMedium returns a medium buffer to the pool.
// The buffer should not be used after calling PutMedium.
func (bp *BufferPool) PutMedium(buf []byte) {
	// Reset buffer length but keep capacity
	buf = buf[:0]
	bp.medium.Put(&buf)
}

// GetLarge returns a large buffer from the pool.
// The caller is responsible for calling PutLarge to return the buffer to the pool.
func (bp *BufferPool) GetLarge() []byte {
	bufPtr := bp.large.Get().(*[]byte)
	// Reset length to 0 but keep capacity
	*bufPtr = (*bufPtr)[:0]
	return *bufPtr
}

// PutLarge returns a large buffer to the pool.
// The buffer should not be used after calling PutLarge.
func (bp *BufferPool) PutLarge(buf []byte) {
	// Reset buffer length but keep capacity
	buf = buf[:0]
	bp.large.Put(&buf)
}

// GetBuffer returns a buffer of the specified minimum size.
// If the requested size is larger than LargeBufferSize, a new buffer is allocated.
// The caller is responsible for calling PutBuffer to return the buffer to the pool.
func (bp *BufferPool) GetBuffer(size int) []byte {
	switch {
	case size <= SmallBufferSize:
		bufPtr := bp.small.Get().(*[]byte)
		*bufPtr = (*bufPtr)[:0]
		return *bufPtr
	case size <= MediumBufferSize:
		bufPtr := bp.medium.Get().(*[]byte)
		*bufPtr = (*bufPtr)[:0]
		return *bufPtr
	case size <= LargeBufferSize:
		bufPtr := bp.large.Get().(*[]byte)
		*bufPtr = (*bufPtr)[:0]
		return *bufPtr
	default:
		// For very large buffers, allocate new ones with zero length
		return make([]byte, 0, size)
	}
}

// PutBuffer returns a buffer to the appropriate pool based on its capacity.
// Buffers larger than LargeBufferSize are not returned to any pool.
func (bp *BufferPool) PutBuffer(buf []byte) {
	switch capacity := cap(buf); capacity {
	case SmallBufferSize:
		bp.PutSmall(buf)
	case MediumBufferSize:
		bp.PutMedium(buf)
	case LargeBufferSize:
		bp.PutLarge(buf)
		// Very large buffers are not pooled to avoid memory bloat
	}
}

// Global buffer pool instance for use throughout the package.
var globalBufferPool = NewBufferPool()

// GetSmallBuffer returns a small buffer from the global pool.
func GetSmallBuffer() []byte {
	return globalBufferPool.GetSmall()
}

// PutSmallBuffer returns a small buffer to the global pool.
func PutSmallBuffer(buf []byte) {
	globalBufferPool.PutSmall(buf)
}

// GetMediumBuffer returns a medium buffer from the global pool.
func GetMediumBuffer() []byte {
	return globalBufferPool.GetMedium()
}

// PutMediumBuffer returns a medium buffer to the global pool.
func PutMediumBuffer(buf []byte) {
	globalBufferPool.PutMedium(buf)
}

// GetLargeBuffer returns a large buffer from the global pool.
func GetLargeBuffer() []byte {
	return globalBufferPool.GetLarge()
}

// PutLargeBuffer returns a large buffer to the global pool.
func PutLargeBuffer(buf []byte) {
	globalBufferPool.PutLarge(buf)
}

// GetBuffer returns a buffer from the global pool for the specified size.
func GetBuffer(size int) []byte {
	return globalBufferPool.GetBuffer(size)
}

// PutBuffer returns a buffer to the global pool.
func PutBuffer(buf []byte) {
	globalBufferPool.PutBuffer(buf)
}
