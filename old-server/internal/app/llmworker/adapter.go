package llmworker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// WorkerPoolAdapter implements speech.LLM, speech.StructuredLLM, and speech.SessionSetter interfaces
// by delegating calls to a worker pool for isolated, retryable LLM processing
type WorkerPoolAdapter struct {
	workerPool *WorkerPool
	baseLLM    speech.LLM // Keep reference to base LLM for session management
	mu         sync.RWMutex
	isRunning  bool
}

// NewWorkerPoolAdapter creates a new adapter that wraps an LLM with worker pool processing
func NewWorkerPoolAdapter(llm speech.LLM, numWorkers int) *WorkerPoolAdapter {
	workerPool := NewWorkerPool(llm, numWorkers)
	
	return &WorkerPoolAdapter{
		workerPool: workerPool,
		baseLLM:    llm,
	}
}

// Start initializes the worker pool
func (a *WorkerPoolAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return nil
	}

	if err := a.workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	a.isRunning = true
	logger.Infof("🚀 [LLM_ADAPTER] Worker pool adapter started")
	return nil
}

// Stop gracefully shuts down the worker pool
func (a *WorkerPoolAdapter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return
	}

	a.workerPool.Stop()
	a.isRunning = false
	logger.Infof("🛑 [LLM_ADAPTER] Worker pool adapter stopped")
}

// Enrich implements speech.LLM.Enrich by delegating to the worker pool
func (a *WorkerPoolAdapter) Enrich(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
	a.mu.RLock()
	if !a.isRunning {
		a.mu.RUnlock()
		return nil, nil, fmt.Errorf("worker pool adapter is not running")
	}
	a.mu.RUnlock()

	// Generate unique job ID
	jobID := fmt.Sprintf("func_%d", time.Now().UnixNano())

	// Submit job to worker pool
	result := a.workerPool.SubmitFunctionJob(ctx, jobID, prompt, partial, cfg)

	if !result.Success {
		if result.Retries > 0 {
			logger.Warnf("⚠️ [LLM_ADAPTER] Function job %s failed after %d retries: %v", jobID, result.Retries, result.Error)
		}
		return nil, nil, result.Error
	}

	// Log successful completion
	if result.Retries > 0 {
		logger.Infof("✅ [LLM_ADAPTER] Function job %s completed successfully after %d retries in %v", 
			jobID, result.Retries, result.Duration)
	} else {
		logger.ServiceDebugf("LLM_ADAPTER", "✅ Function job %s completed successfully in %v", jobID, result.Duration)
	}

	return result.FunctionCalls, result.Usage, nil
}

// GenerateStructured implements speech.StructuredLLM.GenerateStructured by delegating to the worker pool
func (a *WorkerPoolAdapter) GenerateStructured(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error) {
	a.mu.RLock()
	if !a.isRunning {
		a.mu.RUnlock()
		return nil, nil, fmt.Errorf("worker pool adapter is not running")
	}
	a.mu.RUnlock()

	// Generate unique job ID
	jobID := fmt.Sprintf("struct_%d", time.Now().UnixNano())

	// Submit job to worker pool
	result := a.workerPool.SubmitStructuredJob(ctx, jobID, prompt, partial, cfg)

	if !result.Success {
		if result.Retries > 0 {
			logger.Warnf("⚠️ [LLM_ADAPTER] Structured job %s failed after %d retries: %v", jobID, result.Retries, result.Error)
		}
		return nil, nil, result.Error
	}

	// Log successful completion
	if result.Retries > 0 {
		logger.Infof("✅ [LLM_ADAPTER] Structured job %s completed successfully after %d retries in %v", 
			jobID, result.Retries, result.Duration)
	} else {
		logger.ServiceDebugf("LLM_ADAPTER", "✅ Structured job %s completed successfully in %v", jobID, result.Duration)
	}

	return result.StructuredOutput, result.Usage, nil
}

// SetSession implements speech.SessionSetter by delegating to the base LLM
func (a *WorkerPoolAdapter) SetSession(session any) {
	if setter, ok := a.baseLLM.(speech.SessionSetter); ok {
		setter.SetSession(session)
		logger.ServiceDebugf("LLM_ADAPTER", "🔧 Session set on base LLM")
	} else {
		logger.Warnf("⚠️ [LLM_ADAPTER] Base LLM does not implement SessionSetter")
	}
}

// PrepareCache implements speech.CachePreparer by delegating to the base LLM
func (a *WorkerPoolAdapter) PrepareCache(ctx context.Context, cfg *speech.FunctionConfig) (speech.CacheKey, error) {
	if preparer, ok := a.baseLLM.(speech.CachePreparer); ok {
		return preparer.PrepareCache(ctx, cfg)
	}
	return "", fmt.Errorf("base LLM does not implement CachePreparer")
}

// PrepareStructuredCache implements speech.StructuredCachePreparer by delegating to the base LLM
func (a *WorkerPoolAdapter) PrepareStructuredCache(ctx context.Context, cfg *speech.StructuredConfig) (speech.CacheKey, error) {
	if preparer, ok := a.baseLLM.(speech.StructuredCachePreparer); ok {
		return preparer.PrepareStructuredCache(ctx, cfg)
	}
	return "", fmt.Errorf("base LLM does not implement StructuredCachePreparer")
}

// EnrichWithCache implements speech.CachedLLM.EnrichWithCache by delegating to the base LLM
func (a *WorkerPoolAdapter) EnrichWithCache(ctx context.Context, cacheKey speech.CacheKey, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
	if cachedLLM, ok := a.baseLLM.(speech.CachedLLM); ok {
		return cachedLLM.EnrichWithCache(ctx, cacheKey, prompt, partial, cfg)
	}
	return nil, nil, fmt.Errorf("base LLM does not implement CachedLLM")
}

// GenerateStructuredWithCache implements speech.CachedStructuredLLM.GenerateStructuredWithCache by delegating to the base LLM
func (a *WorkerPoolAdapter) GenerateStructuredWithCache(ctx context.Context, cacheKey speech.CacheKey, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error) {
	if cachedStructuredLLM, ok := a.baseLLM.(speech.CachedStructuredLLM); ok {
		return cachedStructuredLLM.GenerateStructuredWithCache(ctx, cacheKey, prompt, partial, cfg)
	}
	return nil, nil, fmt.Errorf("base LLM does not implement CachedStructuredLLM")
}

// GenerateStructuredWithOptimalStrategy implements the optimal strategy interface by delegating to the base LLM
func (a *WorkerPoolAdapter) GenerateStructuredWithOptimalStrategy(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error) {
	// Try to use the optimal strategy if the base LLM supports it
	if optimalLLM, ok := a.baseLLM.(interface {
		GenerateStructuredWithOptimalStrategy(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error)
	}); ok {
		return optimalLLM.GenerateStructuredWithOptimalStrategy(ctx, prompt, partial, cfg)
	}
	
	// Fallback to regular structured generation
	if structuredLLM, ok := a.baseLLM.(speech.StructuredLLM); ok {
		return structuredLLM.GenerateStructured(ctx, prompt, partial, cfg)
	}
	
	return nil, nil, fmt.Errorf("base LLM does not support structured generation")
}

// Ensure interface conformance
var _ speech.LLM = (*WorkerPoolAdapter)(nil)
var _ speech.StructuredLLM = (*WorkerPoolAdapter)(nil)
var _ speech.SessionSetter = (*WorkerPoolAdapter)(nil)
var _ speech.CachePreparer = (*WorkerPoolAdapter)(nil)
var _ speech.StructuredCachePreparer = (*WorkerPoolAdapter)(nil)
var _ speech.CachedLLM = (*WorkerPoolAdapter)(nil)
var _ speech.CachedStructuredLLM = (*WorkerPoolAdapter)(nil)
