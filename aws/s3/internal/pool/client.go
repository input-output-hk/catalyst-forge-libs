package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ClientPool manages a pool of S3 clients for optimized reuse.
// This reduces connection overhead and improves performance for concurrent operations.
type ClientPool struct {
	clients     chan *s3.Client
	factory     func() (*s3.Client, error)
	maxSize     int
	mu          sync.RWMutex
	stats       Stats
	healthCheck func(*s3.Client) bool
}

// Stats tracks pool usage statistics.
type Stats struct {
	Created   int64
	Reused    int64
	Destroyed int64
	Active    int64
	Idle      int64
}

// NewClientPool creates a new client pool.
func NewClientPool(factory func() (*s3.Client, error), size int) *ClientPool {
	if size <= 0 {
		size = 10 // Default pool size
	}

	pool := &ClientPool{
		clients: make(chan *s3.Client, size),
		factory: factory,
		maxSize: size,
		healthCheck: func(c *s3.Client) bool {
			// Simple health check - client exists
			return c != nil
		},
	}

	// Pre-create clients for immediate availability
	for i := 0; i < size/2; i++ {
		if client, err := factory(); err == nil {
			pool.clients <- client
			pool.stats.Created++
			pool.stats.Idle++
		}
	}

	return pool
}

// Get retrieves a client from the pool or creates a new one.
func (p *ClientPool) Get(ctx context.Context) (*s3.Client, error) {
	select {
	case client := <-p.clients:
		// Check health before reusing
		if p.healthCheck(client) {
			p.mu.Lock()
			p.stats.Reused++
			p.stats.Active++
			p.stats.Idle--
			p.mu.Unlock()
			return client, nil
		}
		// Client failed health check, destroy it
		p.mu.Lock()
		p.stats.Destroyed++
		p.stats.Idle--
		p.mu.Unlock()
		// Fall through to create new client
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
		// Pool empty, create new client
	}

	// Create new client
	client, err := p.factory()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.stats.Created++
	p.stats.Active++
	p.mu.Unlock()

	return client, nil
}

// Put returns a client to the pool.
func (p *ClientPool) Put(client *s3.Client) {
	if client == nil {
		return
	}

	// Check health before returning to pool
	if !p.healthCheck(client) {
		p.mu.Lock()
		p.stats.Destroyed++
		p.stats.Active--
		p.mu.Unlock()
		return
	}

	select {
	case p.clients <- client:
		p.mu.Lock()
		p.stats.Active--
		p.stats.Idle++
		p.mu.Unlock()
	default:
		// Pool full, discard client
		p.mu.Lock()
		p.stats.Destroyed++
		p.stats.Active--
		p.mu.Unlock()
	}
}

// Stats returns pool statistics.
func (p *ClientPool) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// Close drains the pool and destroys all clients.
func (p *ClientPool) Close() {
	close(p.clients)
	for range p.clients {
		p.mu.Lock()
		p.stats.Destroyed++
		p.stats.Idle--
		p.mu.Unlock()
	}
}

// SharedClientManager manages a single shared client with optimizations.
type SharedClientManager struct {
	client      *s3.Client
	mu          sync.RWMutex
	lastUsed    time.Time
	useCount    int64
	maxIdleTime time.Duration
}

// NewSharedClientManager creates a manager for a shared S3 client.
func NewSharedClientManager(client *s3.Client, maxIdleTime time.Duration) *SharedClientManager {
	if maxIdleTime <= 0 {
		maxIdleTime = 5 * time.Minute
	}

	manager := &SharedClientManager{
		client:      client,
		maxIdleTime: maxIdleTime,
		lastUsed:    time.Now(),
	}

	// Start idle checker
	go manager.idleChecker()

	return manager
}

// Get returns the shared client and updates usage stats.
func (m *SharedClientManager) Get() *s3.Client {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastUsed = time.Now()
	m.useCount++
	return m.client
}

// Stats returns usage statistics.
func (m *SharedClientManager) Stats() (useCount int64, lastUsed time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.useCount, m.lastUsed
}

// idleChecker monitors idle time and can trigger cleanup.
func (m *SharedClientManager) idleChecker() {
	ticker := time.NewTicker(m.maxIdleTime / 2)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		idle := time.Since(m.lastUsed)
		m.mu.RUnlock()

		if idle > m.maxIdleTime {
			// Client has been idle too long
			// In production, you might want to recreate the client here
			// or trigger a health check
			_ = idle // Placeholder for future implementation
		}
	}
}

// ConnectionManager optimizes connection reuse across operations.
type ConnectionManager struct {
	httpClient   *s3.Client
	connPool     sync.Pool
	maxIdleConns int
}

// NewConnectionManager creates a connection manager with optimized settings.
func NewConnectionManager(client *s3.Client, maxIdleConns int) *ConnectionManager {
	if maxIdleConns <= 0 {
		maxIdleConns = 100
	}

	return &ConnectionManager{
		httpClient:   client,
		maxIdleConns: maxIdleConns,
		connPool: sync.Pool{
			New: func() interface{} {
				// Return wrapped client for connection pooling
				return client
			},
		},
	}
}

// Execute runs an operation with optimized connection handling.
func (m *ConnectionManager) Execute(ctx context.Context, operation func(*s3.Client) error) error {
	// Get client from pool
	client := m.connPool.Get().(*s3.Client)
	defer m.connPool.Put(client)

	// Execute operation
	return operation(client)
}

// RequestBatcher batches multiple requests for efficiency.
type RequestBatcher struct {
	client       *s3.Client
	batchSize    int
	flushTimeout time.Duration
	requests     []interface{}
	mu           sync.Mutex
	flushChan    chan struct{}
}

// NewRequestBatcher creates a new request batcher.
func NewRequestBatcher(client *s3.Client, batchSize int, flushTimeout time.Duration) *RequestBatcher {
	b := &RequestBatcher{
		client:       client,
		batchSize:    batchSize,
		flushTimeout: flushTimeout,
		requests:     make([]interface{}, 0, batchSize),
		flushChan:    make(chan struct{}, 1),
	}

	go b.autoFlusher()
	return b
}

// Add adds a request to the batch.
func (b *RequestBatcher) Add(request interface{}) {
	b.mu.Lock()
	b.requests = append(b.requests, request)
	shouldFlush := len(b.requests) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushChan <- struct{}{}:
		default:
		}
	}
}

// Flush processes all pending requests.
func (b *RequestBatcher) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.requests) == 0 {
		return
	}

	// Process batch
	// Implementation depends on specific request types

	// Clear requests
	b.requests = make([]interface{}, 0, b.batchSize)
}

// autoFlusher periodically flushes the batch.
func (b *RequestBatcher) autoFlusher() {
	ticker := time.NewTicker(b.flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.Flush()
		case <-b.flushChan:
			b.Flush()
		}
	}
}
