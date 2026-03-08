# LLM Worker Pool Refactor

## Problem Statement

The original architecture had several issues that led to 504 timeouts and poor reliability:

1. **Context Inheritance**: LLM calls inherited the WebSocket context, which had a 60-second timeout and could be cancelled by ping/pong failures
2. **Blocking Pipeline**: LLM calls happened synchronously in the pipeline goroutine, blocking transcript processing
3. **No Isolation**: WebSocket connection issues could directly affect LLM processing
4. **Timeout Conflicts**: The 6-second LLM timeout conflicted with WebSocket ping/pong timing
5. **No Retry Logic**: Failed LLM calls didn't have proper retry mechanisms

## Solution: LLM Worker Pool Architecture

The refactor introduces a worker pool system that:

1. **Isolates LLM calls** from WebSocket context
2. **Provides clean, independent contexts** for LLM operations
3. **Implements proper retry logic** with exponential backoff
4. **Maintains real-time responsiveness** through non-blocking job submission
5. **Handles both function and structured output modes**

## Architecture Overview

```
WebSocket Handler → Pipeline → WorkerPoolAdapter → WorkerPool → LLM Workers → Base LLM
```

### Components

#### WorkerPool

-   Manages a pool of worker goroutines
-   Handles job queuing and distribution
-   Implements retry logic with exponential backoff
-   Provides clean contexts for LLM calls

#### WorkerPoolAdapter

-   Implements all LLM interfaces (`speech.LLM`, `speech.StructuredLLM`, etc.)
-   Delegates calls to the worker pool
-   Maintains compatibility with existing pipeline code
-   Provides session management pass-through

#### Worker

-   Individual goroutine that processes LLM jobs
-   Creates isolated contexts for LLM calls
-   Implements retry logic for transient failures
-   Handles both function and structured output jobs

## Key Benefits

### 1. Context Isolation

-   LLM calls use `context.Background()` instead of inheriting WebSocket context
-   No more timeout conflicts between WebSocket ping/pong and LLM processing
-   Clean separation of concerns

### 2. Retry Logic

-   Automatic retry on 5xx errors, timeouts, and connection issues
-   Exponential backoff to prevent overwhelming the LLM service
-   Configurable retry count and delay

### 3. Non-blocking Operation

-   Job submission is non-blocking
-   Pipeline continues processing transcripts while LLM calls are queued
-   Real-time responsiveness maintained

### 4. Graceful Degradation

-   Failed LLM calls don't block the entire pipeline
-   Proper error handling and logging
-   Metrics tracking for retry attempts

## Configuration

The worker pool can be configured with:

-   **Number of workers**: Default 4, configurable via environment variable
-   **LLM timeout**: 30 seconds (independent of WebSocket timeout)
-   **Max retries**: 3 attempts with exponential backoff
-   **Retry delay**: 1 second base delay

## Usage

The refactor is transparent to existing code. The main application now wraps the base LLM with the worker pool adapter:

```go
// Before
llm, err := llmprovider.ProvideLLM(apiKey, modelName)

// After
baseLLM, err := llmprovider.ProvideLLM(apiKey, modelName)
llm := llmworker.NewWorkerPoolAdapter(baseLLM, numWorkers)
llm.Start(context.Background())
defer llm.Stop()
```

All existing pipeline code continues to work without changes, but now benefits from:

-   Isolated LLM processing
-   Automatic retries
-   Better error handling
-   Improved reliability

## Monitoring

The worker pool provides comprehensive logging:

-   Job submission and completion
-   Retry attempts and failures
-   Performance metrics (duration, retry count)
-   Error categorization

## Testing

The package includes comprehensive tests:

-   Basic functionality verification
-   Retry logic testing
-   Context cancellation handling
-   Mock LLM for isolated testing

## Migration

This refactor is a drop-in replacement that maintains full backward compatibility while significantly improving reliability and performance.
