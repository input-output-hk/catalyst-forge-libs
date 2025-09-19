package list

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	s3types "github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// S3Interface defines the S3 operations we need.
type S3Interface interface {
	ListObjectsV2(
		ctx context.Context,
		input *s3.ListObjectsV2Input,
		opts ...func(*s3.Options),
	) (*s3.ListObjectsV2Output, error)
}

// Lister handles efficient listing of S3 objects.
type Lister struct {
	client S3Interface
}

// New creates a new Lister.
func New(client S3Interface) *Lister {
	return &Lister{
		client: client,
	}
}

// Config holds configuration for list operations.
type Config struct {
	Bucket      string
	Prefix      string
	Delimiter   string
	MaxKeys     int32
	StartAfter  string
	PageSize    int32 // Optimal page size for pagination
	Parallelism int   // Number of parallel pagination requests (for multi-prefix listing)
}

// Result represents the result of a list operation.
type Result struct {
	Objects           []s3types.Object
	CommonPrefixes    []string
	IsTruncated       bool
	ContinuationToken string
	KeyCount          int
}

// List performs an optimized single page listing.
func (l *Lister) List(ctx context.Context, config *Config) (*Result, error) {
	// Use optimal page size if not specified
	pageSize := config.MaxKeys
	if pageSize == 0 || pageSize > 1000 {
		pageSize = 1000 // Maximum allowed by S3
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(config.Bucket),
		Prefix:  aws.String(config.Prefix),
		MaxKeys: aws.Int32(pageSize),
	}

	if config.Delimiter != "" {
		input.Delimiter = aws.String(config.Delimiter)
	}
	if config.StartAfter != "" {
		input.StartAfter = aws.String(config.StartAfter)
	}

	output, err := l.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}

	return l.convertOutput(output), nil
}

// ListWithPaginator creates a paginator for efficient multi-page listing.
func (l *Lister) ListWithPaginator(ctx context.Context, config *Config) *Paginator {
	return &Paginator{
		client:   l.client,
		config:   config,
		pageSize: l.optimalPageSize(config),
	}
}

// ListAll performs efficient streaming of all objects.
func (l *Lister) ListAll(ctx context.Context, config *Config) <-chan ObjectResult {
	resultChan := make(chan ObjectResult, 100) // Buffered for performance

	go func() {
		defer close(resultChan)

		paginator := l.ListWithPaginator(ctx, config)

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				resultChan <- ObjectResult{Err: err}
				return
			}

			for _, obj := range page.Objects {
				select {
				case resultChan <- ObjectResult{Object: obj}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return resultChan
}

// ListPrefixes performs parallel listing for multiple prefixes.
func (l *Lister) ListPrefixes(
	ctx context.Context,
	bucket string,
	prefixes []string,
	parallelism int,
) <-chan ObjectResult {
	if parallelism <= 0 {
		parallelism = 5 // Default parallelism
	}

	resultChan := make(chan ObjectResult, 100)

	go func() {
		defer close(resultChan)

		sem := make(chan struct{}, parallelism)
		var wg sync.WaitGroup

		for _, prefix := range prefixes {
			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore

			go func(p string) {
				defer func() {
					<-sem // Release semaphore
					wg.Done()
				}()

				config := &Config{
					Bucket:   bucket,
					Prefix:   p,
					PageSize: 1000,
				}

				for obj := range l.ListAll(ctx, config) {
					select {
					case resultChan <- obj:
					case <-ctx.Done():
						return
					}
				}
			}(prefix)
		}

		wg.Wait()
	}()

	return resultChan
}

// ObjectResult wraps an object or error.
type ObjectResult struct {
	Object s3types.Object
	Err    error
}

// Paginator handles efficient pagination.
type Paginator struct {
	client            S3Interface
	config            *Config
	pageSize          int32
	continuationToken *string
	hasMorePages      bool
	firstPage         bool
}

// HasMorePages returns true if there are more pages to fetch.
func (p *Paginator) HasMorePages() bool {
	return p.firstPage || p.hasMorePages
}

// NextPage fetches the next page of results.
func (p *Paginator) NextPage(ctx context.Context) (*Result, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.config.Bucket),
		Prefix:  aws.String(p.config.Prefix),
		MaxKeys: aws.Int32(p.pageSize),
	}

	if p.config.Delimiter != "" {
		input.Delimiter = aws.String(p.config.Delimiter)
	}

	if !p.firstPage && p.continuationToken != nil {
		input.ContinuationToken = p.continuationToken
	} else if p.config.StartAfter != "" {
		input.StartAfter = aws.String(p.config.StartAfter)
	}

	output, err := p.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list objects page: %w", err)
	}

	p.firstPage = false
	p.hasMorePages = aws.ToBool(output.IsTruncated)
	p.continuationToken = output.NextContinuationToken

	lister := &Lister{client: p.client}
	return lister.convertOutput(output), nil
}

// convertOutput converts S3 output to our Result type.
func (l *Lister) convertOutput(output *s3.ListObjectsV2Output) *Result {
	result := &Result{
		Objects:        make([]s3types.Object, 0, len(output.Contents)),
		CommonPrefixes: make([]string, 0, len(output.CommonPrefixes)),
		IsTruncated:    aws.ToBool(output.IsTruncated),
		KeyCount:       int(aws.ToInt32(output.KeyCount)),
	}

	if output.NextContinuationToken != nil {
		result.ContinuationToken = *output.NextContinuationToken
	}

	for _, obj := range output.Contents {
		result.Objects = append(result.Objects, s3types.Object{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
			StorageClass: string(obj.StorageClass),
		})
	}

	for _, prefix := range output.CommonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, aws.ToString(prefix.Prefix))
	}

	return result
}

// optimalPageSize determines the optimal page size for pagination.
func (l *Lister) optimalPageSize(config *Config) int32 {
	if config.PageSize > 0 && config.PageSize <= 1000 {
		return config.PageSize
	}
	// Default to maximum for efficiency
	return 1000
}

// BatchedList performs efficient listing with result batching.
type BatchedList struct {
	lister    *Lister
	batchSize int
}

// NewBatchedList creates a new batched lister.
func NewBatchedList(lister *Lister, batchSize int) *BatchedList {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BatchedList{
		lister:    lister,
		batchSize: batchSize,
	}
}

// ListBatches returns results in batches for efficient processing.
func (b *BatchedList) ListBatches(ctx context.Context, config *Config) <-chan []s3types.Object {
	batchChan := make(chan []s3types.Object, 10)

	go func() {
		defer close(batchChan)

		batch := make([]s3types.Object, 0, b.batchSize)

		for result := range b.lister.ListAll(ctx, config) {
			if result.Err != nil {
				// Send partial batch if we have any
				if len(batch) > 0 {
					select {
					case batchChan <- batch:
					case <-ctx.Done():
						return
					}
				}
				return
			}

			batch = append(batch, result.Object)

			if len(batch) >= b.batchSize {
				select {
				case batchChan <- batch:
					batch = make([]s3types.Object, 0, b.batchSize)
				case <-ctx.Done():
					return
				}
			}
		}

		// Send remaining items
		if len(batch) > 0 {
			select {
			case batchChan <- batch:
			case <-ctx.Done():
			}
		}
	}()

	return batchChan
}
