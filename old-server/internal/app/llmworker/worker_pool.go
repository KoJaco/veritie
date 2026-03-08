package llmworker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// WorkerPool manages a pool of LLM workers for processing function and structured output requests
type WorkerPool struct {
	// Configuration
	numWorkers int
	llmTimeout time.Duration
	maxRetries int
	retryDelay time.Duration

	// Worker management
	workers    []*Worker
	jobQueue   chan *LLMJob
	workerPool chan chan *LLMJob
	quit       chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	isRunning  bool

	// LLM dependencies
	llm speech.LLM
}

// Worker represents a single LLM worker goroutine
type Worker struct {
	id         int
	jobQueue   chan *LLMJob
	workerPool chan chan *LLMJob
	llm        speech.LLM
	quit       chan struct{}
	timeout    time.Duration
	maxRetries int
	retryDelay time.Duration
}

// LLMJob represents a single LLM processing job
type LLMJob struct {
	ID        string
	Type      JobType
	Prompt    speech.Prompt
	Transcript speech.Transcript
	Config    interface{} // *speech.FunctionConfig or *speech.StructuredConfig
	Result    chan *LLMResult
	Created   time.Time
}

// JobType indicates the type of LLM job
type JobType string

const (
	JobTypeFunction  JobType = "function"
	JobTypeStructured JobType = "structured"
)

// LLMResult contains the result of an LLM job
type LLMResult struct {
	JobID       string
	Success     bool
	Error       error
	FunctionCalls []speech.FunctionCall
	StructuredOutput map[string]any
	Usage       *speech.LLMUsage
	Duration    time.Duration
	Retries     int
}

// NewWorkerPool creates a new LLM worker pool
func NewWorkerPool(llm speech.LLM, numWorkers int) *WorkerPool {
	return &WorkerPool{
		numWorkers: numWorkers,
		llmTimeout: 30 * time.Second, // Independent timeout for LLM calls
		maxRetries: 3,
		retryDelay: 1 * time.Second,
		llm:        llm,
		jobQueue:   make(chan *LLMJob, 100), // Buffer up to 100 jobs
		workerPool: make(chan chan *LLMJob, numWorkers),
		quit:       make(chan struct{}),
	}
}

// Start initializes and starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.isRunning {
		return nil
	}

	wp.isRunning = true
	logger.Infof("🚀 [LLM_WORKER] Starting LLM worker pool with %d workers", wp.numWorkers)

	// Create workers
	wp.workers = make([]*Worker, wp.numWorkers)
	for i := 0; i < wp.numWorkers; i++ {
		worker := &Worker{
			id:         i + 1,
			jobQueue:   make(chan *LLMJob),
			workerPool: wp.workerPool,
			llm:        wp.llm,
			quit:       make(chan struct{}),
			timeout:    wp.llmTimeout,
			maxRetries: wp.maxRetries,
			retryDelay: wp.retryDelay,
		}
		wp.workers[i] = worker
	}

	// Start all workers
	for _, worker := range wp.workers {
		wp.wg.Add(1)
		go worker.start(ctx, &wp.wg)
	}

	// Start dispatcher
	wp.wg.Add(1)
	go wp.dispatch(ctx)

	return nil
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.isRunning {
		return
	}

	logger.Infof("🛑 [LLM_WORKER] Stopping LLM worker pool...")

	close(wp.quit)

	// Stop all workers
	for _, worker := range wp.workers {
		close(worker.quit)
	}

	wp.wg.Wait()
	wp.isRunning = false

	logger.Infof("✅ [LLM_WORKER] LLM worker pool stopped successfully")
}

// SubmitFunctionJob submits a function extraction job to the worker pool
func (wp *WorkerPool) SubmitFunctionJob(ctx context.Context, jobID string, prompt speech.Prompt, transcript speech.Transcript, cfg *speech.FunctionConfig) *LLMResult {
	resultChan := make(chan *LLMResult, 1)
	
	job := &LLMJob{
		ID:        jobID,
		Type:      JobTypeFunction,
		Prompt:    prompt,
		Transcript: transcript,
		Config:    cfg,
		Result:    resultChan,
		Created:   time.Now(),
	}

	// Submit job (non-blocking)
	select {
	case wp.jobQueue <- job:
		logger.Debugf("📤 [LLM_WORKER] Function job %s submitted to queue", jobID)
	case <-ctx.Done():
		return &LLMResult{
			JobID:   jobID,
			Success: false,
			Error:   ctx.Err(),
		}
	}

	// Wait for result (with context timeout)
	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		return &LLMResult{
			JobID:   jobID,
			Success: false,
			Error:   ctx.Err(),
		}
	}
}

// SubmitStructuredJob submits a structured output job to the worker pool
func (wp *WorkerPool) SubmitStructuredJob(ctx context.Context, jobID string, prompt speech.Prompt, transcript speech.Transcript, cfg *speech.StructuredConfig) *LLMResult {
	resultChan := make(chan *LLMResult, 1)
	
	job := &LLMJob{
		ID:        jobID,
		Type:      JobTypeStructured,
		Prompt:    prompt,
		Transcript: transcript,
		Config:    cfg,
		Result:    resultChan,
		Created:   time.Now(),
	}

	// Submit job (non-blocking)
	select {
	case wp.jobQueue <- job:
		logger.Debugf("📤 [LLM_WORKER] Structured job %s submitted to queue", jobID)
	case <-ctx.Done():
		return &LLMResult{
			JobID:   jobID,
			Success: false,
			Error:   ctx.Err(),
		}
	}

	// Wait for result (with context timeout)
	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		return &LLMResult{
			JobID:   jobID,
			Success: false,
			Error:   ctx.Err(),
		}
	}
}

// dispatch distributes jobs to available workers
func (wp *WorkerPool) dispatch(ctx context.Context) {
	defer wp.wg.Done()

	logger.ServiceDebugf("LLM_WORKER", "🚀 Job dispatcher started")

	for {
		select {
		case <-ctx.Done():
			logger.ServiceDebugf("LLM_WORKER", "🛑 Job dispatcher stopping (context cancelled)")
			return
		case <-wp.quit:
			logger.ServiceDebugf("LLM_WORKER", "🛑 Job dispatcher stopping (quit signal)")
			return
		case job := <-wp.jobQueue:
			// Wait for an available worker
			select {
			case workerJobQueue := <-wp.workerPool:
				// Send job to worker
				workerJobQueue <- job
			case <-ctx.Done():
				return
			case <-wp.quit:
				return
			}
		}
	}
}

// Worker methods
func (w *Worker) start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	logger.ServiceDebugf("LLM_WORKER", "🚀 Worker %d started", w.id)

	for {
		// Register this worker in the worker pool
		select {
		case w.workerPool <- w.jobQueue:
			// Wait for job assignment
			select {
			case job := <-w.jobQueue:
				w.processJob(ctx, job)
			case <-ctx.Done():
				logger.ServiceDebugf("LLM_WORKER", "🛑 Worker %d stopping (context cancelled)", w.id)
				return
			case <-w.quit:
				logger.ServiceDebugf("LLM_WORKER", "🛑 Worker %d stopping (quit signal)", w.id)
				return
			}
		case <-ctx.Done():
			logger.ServiceDebugf("LLM_WORKER", "🛑 Worker %d stopping (context cancelled)", w.id)
			return
		case <-w.quit:
			logger.ServiceDebugf("LLM_WORKER", "🛑 Worker %d stopping (quit signal)", w.id)
			return
		}
	}
}

func (w *Worker) processJob(ctx context.Context, job *LLMJob) {
	startTime := time.Now()
	logger.Debugf("🔧 [LLM_WORKER] Worker %d: Processing job %s (type=%s)", w.id, job.ID, job.Type)

	// Create clean context for LLM call (independent of WebSocket context)
	llmCtx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	var result *LLMResult

	// Process based on job type
	switch job.Type {
	case JobTypeFunction:
		result = w.processFunctionJob(llmCtx, job)
	case JobTypeStructured:
		result = w.processStructuredJob(llmCtx, job)
	default:
		result = &LLMResult{
			JobID:   job.ID,
			Success: false,
			Error:   fmt.Errorf("unknown job type: %s", job.Type),
		}
	}

	result.Duration = time.Since(startTime)
	result.JobID = job.ID

	// Send result back to caller
	select {
	case job.Result <- result:
		logger.Debugf("✅ [LLM_WORKER] Worker %d: Job %s completed in %v (success=%v)", 
			w.id, job.ID, result.Duration, result.Success)
	case <-llmCtx.Done():
		logger.Warnf("⚠️ [LLM_WORKER] Worker %d: Job %s result channel closed (LLM context cancelled)", w.id, job.ID)
	}
}

func (w *Worker) processFunctionJob(llmCtx context.Context, job *LLMJob) *LLMResult {
	cfg, ok := job.Config.(*speech.FunctionConfig)
	if !ok {
		return &LLMResult{
			Success: false,
			Error:   fmt.Errorf("invalid config type for function job"),
		}
	}

	// Retry logic for function jobs
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		calls, usage, err := w.llm.Enrich(llmCtx, job.Prompt, job.Transcript, cfg)
		
		if err == nil {
			return &LLMResult{
				Success:       true,
				FunctionCalls: calls,
				Usage:         usage,
				Retries:       attempt,
			}
		}

		// Check if error is retryable
		if !w.isRetryableError(err) {
			return &LLMResult{
				Success: false,
				Error:   err,
				Retries: attempt,
			}
		}

		// Log retry attempt
		logger.Warnf("⚠️ [LLM_WORKER] Worker %d: Function job %s attempt %d failed: %v (retrying...)", 
			w.id, job.ID, attempt+1, err)

		// Wait before retry (exponential backoff)
		if attempt < w.maxRetries {
			backoffDelay := w.retryDelay * time.Duration(1<<attempt)
			select {
			case <-llmCtx.Done():
				return &LLMResult{
					Success: false,
					Error:   llmCtx.Err(),
					Retries: attempt,
				}
			case <-time.After(backoffDelay):
				continue
			}
		}
	}

	return &LLMResult{
		Success: false,
		Error:   fmt.Errorf("function job failed after %d retries", w.maxRetries+1),
		Retries: w.maxRetries,
	}
}

func (w *Worker) processStructuredJob(llmCtx context.Context, job *LLMJob) *LLMResult {
	cfg, ok := job.Config.(*speech.StructuredConfig)
	if !ok {
		return &LLMResult{
			Success: false,
			Error:   fmt.Errorf("invalid config type for structured job"),
		}
	}

	// Check if LLM supports structured output
	structuredLLM, ok := w.llm.(speech.StructuredLLM)
	if !ok {
		return &LLMResult{
			Success: false,
			Error:   fmt.Errorf("LLM does not support structured output"),
		}
	}

	// Retry logic for structured jobs
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		output, usage, err := structuredLLM.GenerateStructured(llmCtx, job.Prompt, job.Transcript, cfg)
		
		if err == nil {
			return &LLMResult{
				Success:         true,
				StructuredOutput: output,
				Usage:           usage,
				Retries:         attempt,
			}
		}

		// Check if error is retryable
		if !w.isRetryableError(err) {
			return &LLMResult{
				Success: false,
				Error:   err,
				Retries: attempt,
			}
		}

		// Log retry attempt
		logger.Warnf("⚠️ [LLM_WORKER] Worker %d: Structured job %s attempt %d failed: %v (retrying...)", 
			w.id, job.ID, attempt+1, err)

		// Wait before retry (exponential backoff)
		if attempt < w.maxRetries {
			backoffDelay := w.retryDelay * time.Duration(1<<attempt)
			select {
			case <-llmCtx.Done():
				return &LLMResult{
					Success: false,
					Error:   llmCtx.Err(),
					Retries: attempt,
				}
			case <-time.After(backoffDelay):
				continue
			}
		}
	}

	return &LLMResult{
		Success: false,
		Error:   fmt.Errorf("structured job failed after %d retries", w.maxRetries+1),
		Retries: w.maxRetries,
	}
}

// isRetryableError determines if an error should trigger a retry
func (w *Worker) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	
	// Retry on 5xx errors (server errors)
	if strings.Contains(errStr, "500") || 
	   strings.Contains(errStr, "502") || 
	   strings.Contains(errStr, "503") || 
	   strings.Contains(errStr, "504") {
		return true
	}

	// Retry on timeout errors
	if strings.Contains(errStr, "timeout") || 
	   strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	// Retry on connection errors
	if strings.Contains(errStr, "connection") || 
	   strings.Contains(errStr, "network") {
		return true
	}

	// Don't retry on 4xx errors (client errors) or other non-retryable errors
	return false
}
