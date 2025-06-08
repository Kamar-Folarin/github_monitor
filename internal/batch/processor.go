package batch

import (
	"context"
	"fmt"
	"sync"
	"time"


	"github.com/Kamar-Folarin/github-monitor/internal/config"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// Processor handles batch processing of items
type Processor struct {
	config     *config.BatchConfig
	statusChan chan *models.BatchProgress
	mu         sync.RWMutex
}

// NewProcessor creates a new batch processor
func NewProcessor(cfg *config.BatchConfig) *Processor {
	return &Processor{
		config:     cfg,
		statusChan: make(chan *models.BatchProgress, 1),
	}
}

// ProcessItems processes items in batches
func (p *Processor) ProcessItems(ctx context.Context, items []interface{}, processFn func(ctx context.Context, batch []interface{}) error) error {
	totalItems := len(items)
	if totalItems == 0 {
		return nil
	}

	batchSize := p.config.Size
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}

	totalBatches := (totalItems + batchSize - 1) / batchSize
	progress := &models.BatchProgress{
		TotalBatches:     totalBatches,
		ProcessedBatches: 0,
		TotalCommits:     totalItems,
		ProcessedCommits: 0,
		StartTime:        time.Now(),
		LastUpdateTime:   time.Now(),
	}

	p.updateProgress(progress)

	// Create worker pool
	workerChan := make(chan int, p.config.Workers)
	var wg sync.WaitGroup
	var processErr error
	var mu sync.Mutex

	for i := 0; i < totalBatches; i++ {
		select {
		case <-ctx.Done():
			progress.Errors = append(progress.Errors, ctx.Err())
			p.updateProgress(progress)
			return ctx.Err()
		case workerChan <- i:
			wg.Add(1)
			go func(batchNum int) {
				defer wg.Done()
				defer func() { <-workerChan }()

				start := batchNum * batchSize
				end := start + batchSize
				if end > totalItems {
					end = totalItems
				}

				batch := items[start:end]
				err := p.processBatchWithRetry(ctx, batch, processFn)
				if err != nil {
					mu.Lock()
					if processErr == nil {
						processErr = err
					}
					progress.Errors = append(progress.Errors, err)
					mu.Unlock()
					return
				}

				mu.Lock()
				progress.ProcessedBatches++
				progress.ProcessedCommits += len(batch)
				progress.LastUpdateTime = time.Now()
				if len(batch) > 0 {
					if commit, ok := batch[len(batch)-1].(*models.Commit); ok {
						progress.LastProcessedSHA = commit.SHA
					}
				}
				p.updateProgress(progress)
				mu.Unlock()

				// Add delay between batches if configured
				if p.config.BatchDelay > 0 {
					time.Sleep(p.config.BatchDelay)
				}
			}(i)
		}
	}

	wg.Wait()

	if processErr != nil {
		progress.Errors = append(progress.Errors, processErr)
		p.updateProgress(progress)
		return processErr
	}

	p.updateProgress(progress)
	return nil
}

// GetProgress returns the current progress channel
func (p *Processor) GetProgress() <-chan *models.BatchProgress {
	return p.statusChan
}

// processBatchWithRetry processes a batch with retry logic
func (p *Processor) processBatchWithRetry(ctx context.Context, batch []interface{}, processFn func(ctx context.Context, batch []interface{}) error) error {
	var lastErr error
	for retry := 0; retry <= p.config.MaxRetries; retry++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := processFn(ctx, batch)
			if err == nil {
				return nil
			}

			lastErr = err
			if retry < p.config.MaxRetries {
				backoff := time.Duration(float64(p.config.BatchDelay) * float64(retry+1))
				time.Sleep(backoff)
			}
		}
	}

	return fmt.Errorf("failed to process batch after %d retries: %v", p.config.MaxRetries, lastErr)
}

// updateProgress updates and sends the current progress
func (p *Processor) updateProgress(progress *models.BatchProgress) {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case p.statusChan <- progress:
	default:
		// Channel is full, replace the value
		<-p.statusChan
		p.statusChan <- progress
	}
} 