// Package testutil provides test utilities for progress tracking.
package testutil

// MockProgressTracker is a mock implementation of ProgressTracker for testing.
type MockProgressTracker struct {
	UpdateCalled     bool
	CompleteCalled   bool
	ErrorCalled      bool
	BytesTransferred int64
	TotalBytes       int64
	LastError        error
	Updates          []ProgressUpdate // For detailed tracking
}

// ProgressUpdate represents a single progress update event.
type ProgressUpdate struct {
	Transferred int64
	Total       int64
}

// Update records a progress update.
func (m *MockProgressTracker) Update(bytesTransferred, totalBytes int64) {
	m.UpdateCalled = true
	m.BytesTransferred = bytesTransferred
	m.TotalBytes = totalBytes
	m.Updates = append(m.Updates, ProgressUpdate{
		Transferred: bytesTransferred,
		Total:       totalBytes,
	})
}

// Complete marks the operation as complete.
func (m *MockProgressTracker) Complete() {
	m.CompleteCalled = true
}

// Error records an error.
func (m *MockProgressTracker) Error(err error) {
	m.ErrorCalled = true
	m.LastError = err
}

// Reset clears the mock tracker state.
func (m *MockProgressTracker) Reset() {
	m.UpdateCalled = false
	m.CompleteCalled = false
	m.ErrorCalled = false
	m.BytesTransferred = 0
	m.TotalBytes = 0
	m.LastError = nil
	m.Updates = nil
}
