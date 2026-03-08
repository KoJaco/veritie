package llmworker

import (
	"context"
	"testing"
	"time"

	"schma.ai/internal/domain/speech"
)

// MockLLM implements speech.LLM for testing
type MockLLM struct {
	shouldFail bool
	delay      time.Duration
}

func (m *MockLLM) Enrich(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	
	if m.shouldFail {
		return nil, nil, &MockError{message: "500 Internal Server Error"}
	}
	
	return []speech.FunctionCall{
		{Name: "test_function", Args: map[string]interface{}{"test": "value"}},
	}, &speech.LLMUsage{Prompt: 10, Completion: 5}, nil
}

// MockError is a simple error type for testing
type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}

func TestWorkerPool_BasicFunctionality(t *testing.T) {
	// Create a mock LLM
	mockLLM := &MockLLM{shouldFail: false, delay: 10 * time.Millisecond}
	
	// Create worker pool
	pool := NewWorkerPool(mockLLM, 2)
	
	// Start the pool
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()
	
	// Submit a job
	jobID := "test_job_1"
	prompt := speech.Prompt("test prompt")
	transcript := speech.Transcript{Text: "test transcript", IsFinal: true}
	cfg := &speech.FunctionConfig{
		ParsingConfig: speech.ParsingConfig{ParsingStrategy: "realtime"},
		UpdateMs: 1000,
		Declarations: []speech.FunctionDefinition{
			{Name: "test_function", Description: "test function"},
		},
		ParsingGuide: "test guide",
	}
	
	result := pool.SubmitFunctionJob(ctx, jobID, prompt, transcript, cfg)
	
	// Verify result
	if !result.Success {
		t.Errorf("Expected successful result, got error: %v", result.Error)
	}
	
	if len(result.FunctionCalls) != 1 {
		t.Errorf("Expected 1 function call, got %d", len(result.FunctionCalls))
	}
	
	if result.FunctionCalls[0].Name != "test_function" {
		t.Errorf("Expected function name 'test_function', got '%s'", result.FunctionCalls[0].Name)
	}
	
	if result.Duration < 10*time.Millisecond {
		t.Errorf("Expected duration >= 10ms, got %v", result.Duration)
	}
}

func TestWorkerPool_RetryOnFailure(t *testing.T) {
	// Create a mock LLM that fails initially but succeeds on retry
	mockLLM := &MockLLM{shouldFail: true, delay: 5 * time.Millisecond}
	
	// Create worker pool with shorter retry delay for testing
	pool := &WorkerPool{
		numWorkers: 1,
		llmTimeout: 30 * time.Second,
		maxRetries: 2,
		retryDelay: 10 * time.Millisecond, // Short delay for testing
		llm:        mockLLM,
		jobQueue:   make(chan *LLMJob, 10),
		workerPool: make(chan chan *LLMJob, 1),
		quit:       make(chan struct{}),
	}
	
	// Start the pool
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()
	
	// Submit a job
	jobID := "test_job_2"
	prompt := speech.Prompt("test prompt")
	transcript := speech.Transcript{Text: "test transcript", IsFinal: true}
	cfg := &speech.FunctionConfig{
		ParsingConfig: speech.ParsingConfig{ParsingStrategy: "realtime"},
		UpdateMs: 1000,
		Declarations: []speech.FunctionDefinition{
			{Name: "test_function", Description: "test function"},
		},
		ParsingGuide: "test guide",
	}
	
	result := pool.SubmitFunctionJob(ctx, jobID, prompt, transcript, cfg)
	
	// Verify result shows failure after retries
	if result.Success {
		t.Error("Expected failure result after retries")
	}
	
	if result.Retries != 2 {
		t.Errorf("Expected 2 retries, got %d", result.Retries)
	}
	
	if result.Error == nil {
		t.Error("Expected error after retries")
	}
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	// Create a mock LLM with long delay
	mockLLM := &MockLLM{shouldFail: false, delay: 100 * time.Millisecond}
	
	// Create worker pool
	pool := NewWorkerPool(mockLLM, 1)
	
	// Start the pool
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()
	
	// Create a context with short timeout
	shortCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	// Submit a job with short timeout
	jobID := "test_job_3"
	prompt := speech.Prompt("test prompt")
	transcript := speech.Transcript{Text: "test transcript", IsFinal: true}
	cfg := &speech.FunctionConfig{
		ParsingConfig: speech.ParsingConfig{ParsingStrategy: "realtime"},
		UpdateMs: 1000,
		Declarations: []speech.FunctionDefinition{
			{Name: "test_function", Description: "test function"},
		},
		ParsingGuide: "test guide",
	}
	
	result := pool.SubmitFunctionJob(shortCtx, jobID, prompt, transcript, cfg)
	
	// Verify result shows context cancellation
	if result.Success {
		t.Error("Expected failure due to context cancellation")
	}
	
	if result.Error != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got: %v", result.Error)
	}
}
