package batch

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/batch"
	"schma.ai/internal/pkg/logger"
)

// QueueManager manages batch job processing without constant database polling
type QueueManager struct {
	jobRepo   batch.JobRepo
	processor *BatchProcessor

	// Job queue channel
	jobQueue   chan string      // Job IDs to process
	workerPool chan chan string // Worker pool
	workers    []*QueueWorker

	// Control
	quit      chan struct{}
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.Mutex
}

// QueueWorker represents a single worker goroutine
type QueueWorker struct {
	id         int
	jobQueue   chan string
	workerPool chan chan string
	processor  *BatchProcessor
	quit       chan struct{}
}

func NewQueueManager(jobRepo batch.JobRepo, processor *BatchProcessor, numWorkers int) *QueueManager {
	qm := &QueueManager{
		jobRepo:    jobRepo,
		processor:  processor,
		jobQueue:   make(chan string, 100), // Buffer up to 100 job IDs
		workerPool: make(chan chan string, numWorkers),
		workers:    make([]*QueueWorker, numWorkers),
		quit:       make(chan struct{}),
	}

	// Create workers
	for i := 0; i < numWorkers; i++ {
		worker := &QueueWorker{
			id:         i + 1,
			jobQueue:   make(chan string),
			workerPool: qm.workerPool,
			processor:  processor,
			quit:       make(chan struct{}),
		}
		qm.workers[i] = worker
	}

	return qm
}

func (qm *QueueManager) Start(ctx context.Context) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.isRunning {
		return nil
	}

	qm.isRunning = true
	logger.Infof("🚀 [BATCH] Starting batch queue manager with %d workers", len(qm.workers))

	// Start all workers
	for _, worker := range qm.workers {
		qm.wg.Add(1)
		go worker.start(ctx, &qm.wg)
	}

	// Start dispatcher
	qm.wg.Add(1)
	go qm.dispatch(ctx)

	return nil
}

func (qm *QueueManager) Stop() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if !qm.isRunning {
		return
	}

	logger.Infof("ℹ️ [BATCH] Stopping batch queue manager...")

	close(qm.quit)

	// Stop all workers
	for _, worker := range qm.workers {
		close(worker.quit)
	}

	qm.wg.Wait()
	qm.isRunning = false

	logger.Infof("🛑 [BATCH] Batch queue manager stopped")
}

// EnqueueJob adds a job to the processing queue (called when a new job is submitted)
func (qm *QueueManager) EnqueueJob(jobID string) {
	select {
	case qm.jobQueue <- jobID:
		logger.Infof("ℹ️ [BATCH] Job %s enqueued for processing", jobID)
	default:
		logger.Warnf("⚠️ [BATCH] Job queue full, job %s may be delayed", jobID)
		// Still try to enqueue with blocking behavior
		qm.jobQueue <- jobID
	}
}

// dispatch distributes jobs to available workers
func (qm *QueueManager) dispatch(ctx context.Context) {
	defer qm.wg.Done()

	logger.Infof("🚀 [BATCH] Batch job dispatcher started")

	for {
		select {
		case <-ctx.Done():
			logger.Infof("🛑 [BATCH] Batch job dispatcher stopping (context cancelled)")
			return
		case <-qm.quit:
			logger.Infof("🛑 [BATCH] Batch job dispatcher stopping (quit signal)")
			return
		case jobID := <-qm.jobQueue:
			// Wait for an available worker
			select {
			case workerJobQueue := <-qm.workerPool:
				// Send job to worker
				workerJobQueue <- jobID
			case <-ctx.Done():
				return
			case <-qm.quit:
				return
			}
		}
	}
}

// QueueWorker methods
func (w *QueueWorker) start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	logger.Infof("🚀 [BATCH] Batch worker %d started (idle)", w.id)

	for {
		// Register this worker in the worker pool
		select {
		case w.workerPool <- w.jobQueue:
			// Wait for job assignment
			select {
			case jobID := <-w.jobQueue:
				w.processJob(ctx, jobID)
			case <-ctx.Done():
				logger.Infof("🛑 [BATCH] Batch worker %d stopping (context cancelled)", w.id)
				return
			case <-w.quit:
				logger.Infof("🛑 [BATCH] Batch worker %d stopping (quit signal)", w.id)
				return
			}
		case <-ctx.Done():
			logger.Infof("🛑 [BATCH] Batch worker %d stopping (context cancelled)", w.id)
			return
		case <-w.quit:
			logger.Infof("🛑 [BATCH] Batch worker %d stopping (quit signal)", w.id)
			return
		}
	}
}

func (w *QueueWorker) processJob(ctx context.Context, jobID string) {
	logger.Infof("🚀 [BATCH] Worker %d: Processing job %s", w.id, jobID)

	// Get job from database
	var jobUUID pgtype.UUID
	if err := jobUUID.Scan(jobID); err != nil {
		logger.Errorf("❌ [BATCH] Worker %d: Invalid job ID %s: %v", w.id, jobID, err)
		return
	}

	job, err := w.processor.jobRepo.Get(ctx, jobUUID)
	if err != nil {
		logger.Errorf("❌ [BATCH] Worker %d: Failed to get job %s: %v", w.id, jobID, err)
		return
	}

	// Process the job
	startTime := time.Now()
	if err := w.processor.ProcessJob(ctx, job); err != nil {
		logger.Errorf("❌ [BATCH] Worker %d: Job %s failed after %v: %v", w.id, jobID, time.Since(startTime), err)
		return
	}

	logger.Infof("✅ [BATCH] Worker %d: Job %s completed in %v", w.id, jobID, time.Since(startTime))
}
