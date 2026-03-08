# Batch Processing Architecture

## Overview

The batch processing system enables asynchronous processing of audio files through the same speech-to-function pipeline used for real-time sessions. It provides job queuing, worker management, progress tracking, and result storage with horizontal scalability and fault tolerance.

## Core Architecture

### System Flow

```
File Upload → Job Creation → Queue Management → Worker Processing → Result Storage
     ↓             ↓              ↓                 ↓                ↓
 REST API     Database         Memory Queue      Pipeline        Database
```

### Component Diagram

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP API      │    │  Job Queue      │    │   Worker        │
│  (File Upload)  │───▶│   Manager       │───▶│    Pool         │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Job Status    │    │   In-Memory     │    │ Batch Processor │
│   Tracking      │    │    Queue        │    │  (Pipeline)     │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Database      │    │   Goroutine     │    │   File System   │
│  Persistence    │    │   Dispatcher    │    │   Processing    │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Domain Layer

### Job Model (`internal/domain/batch/job.go`)

```go
type JobStatus string

const (
    StatusQueued     JobStatus = "queued"     // Waiting for processing
    StatusProcessing JobStatus = "processing" // Currently being processed
    StatusCompleted  JobStatus = "completed"  // Successfully finished
    StatusFailed     JobStatus = "failed"     // Processing failed
)

type Job struct {
    ID           pgtype.UUID    `json:"id"`
    AppID        pgtype.UUID    `json:"app_id"`
    AccountID    pgtype.UUID    `json:"account_id"`
    Status       JobStatus      `json:"status"`
    FilePath     string         `json:"file_path"`     // Local file system path
    FileSize     int64          `json:"file_size"`     // File size in bytes
    Config       map[string]any `json:"config"`        // Processing configuration
    Result       map[string]any `json:"result"`        // Processing output
    ErrorMessage string         `json:"error_message"` // Failure details
    StartedAt    *time.Time     `json:"started_at"`    // Processing start time
    CompletedAt  *time.Time     `json:"completed_at"`  // Processing end time
    CreatedAt    time.Time      `json:"created_at"`    // Job creation time
    UpdatedAt    time.Time      `json:"updated_at"`    // Last status update
}
```

### Repository Interface

```go
type JobRepo interface {
    Create(ctx context.Context, appID, accountID pgtype.UUID,
           filePath string, fileSize int64, config map[string]any) (Job, error)
    Get(ctx context.Context, id pgtype.UUID) (Job, error)
    ListQueued(ctx context.Context, limit int) ([]Job, error)
    UpdateStatus(ctx context.Context, id pgtype.UUID, status JobStatus,
                 result map[string]any, errorMsg string) error
    ListByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]Job, error)
}
```

## Application Layer

### Queue Manager (`internal/app/batch/queue_manager.go`)

The QueueManager orchestrates job processing without constant database polling:

```go
type QueueManager struct {
    jobRepo   batch.JobRepo
    processor *BatchProcessor

    // Job queue channel
    jobQueue   chan string      // Job IDs to process
    workerPool chan chan string // Worker pool for load balancing
    workers    []*QueueWorker   // Worker goroutines

    // Control channels
    quit      chan struct{}
    wg        sync.WaitGroup
    isRunning bool
    mu        sync.Mutex
}

func NewQueueManager(jobRepo batch.JobRepo, processor *BatchProcessor,
                     numWorkers int) *QueueManager {
    qm := &QueueManager{
        jobRepo:    jobRepo,
        processor:  processor,
        jobQueue:   make(chan string, 100), // Buffer up to 100 job IDs
        workerPool: make(chan chan string, numWorkers),
        workers:    make([]*QueueWorker, numWorkers),
        quit:       make(chan struct{}),
    }

    // Create worker pool
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
```

#### Queue Management Flow

```go
func (qm *QueueManager) Start(ctx context.Context) error {
    qm.mu.Lock()
    defer qm.mu.Unlock()

    if qm.isRunning {
        return nil
    }

    qm.isRunning = true
    log.Printf("Starting batch queue manager with %d workers", len(qm.workers))

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

func (qm *QueueManager) dispatch(ctx context.Context) {
    defer qm.wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        case <-qm.quit:
            return
        case jobID := <-qm.jobQueue:
            // Wait for available worker
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
```

#### Job Enqueuing

```go
func (qm *QueueManager) EnqueueJob(jobID string) {
    select {
    case qm.jobQueue <- jobID:
        log.Printf("Job %s enqueued for processing", jobID)
    default:
        log.Printf("Warning: Job queue full, job %s may be delayed", jobID)
        // Block until space available
        qm.jobQueue <- jobID
    }
}
```

### Worker Implementation

```go
type QueueWorker struct {
    id         int
    jobQueue   chan string
    workerPool chan chan string
    processor  *BatchProcessor
    quit       chan struct{}
}

func (w *QueueWorker) start(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()

    log.Printf("Batch worker %d started", w.id)

    for {
        // Register in worker pool
        select {
        case w.workerPool <- w.jobQueue:
            // Wait for job assignment
            select {
            case jobID := <-w.jobQueue:
                w.processJob(ctx, jobID)
            case <-ctx.Done():
                return
            case <-w.quit:
                return
            }
        case <-ctx.Done():
            return
        case <-w.quit:
            return
        }
    }
}

func (w *QueueWorker) processJob(ctx context.Context, jobID string) {
    log.Printf("Worker %d: Processing job %s", w.id, jobID)

    // Get job from database
    var jobUUID pgtype.UUID
    if err := jobUUID.Scan(jobID); err != nil {
        log.Printf("Worker %d: Invalid job ID %s: %v", w.id, jobID, err)
        return
    }

    job, err := w.processor.jobRepo.Get(ctx, jobUUID)
    if err != nil {
        log.Printf("Worker %d: Failed to get job %s: %v", w.id, jobID, err)
        return
    }

    // Process the job
    startTime := time.Now()
    if err := w.processor.ProcessJob(ctx, job); err != nil {
        log.Printf("Worker %d: Job %s failed after %v: %v",
                   w.id, jobID, time.Since(startTime), err)
        return
    }

    log.Printf("Worker %d: Job %s completed in %v",
               w.id, jobID, time.Since(startTime))
}
```

### Batch Processor (`internal/app/batch/runner.go`)

The processor handles the actual file processing logic:

```go
type BatchProcessor struct {
    jobRepo batch.JobRepo
    stt     speech.STTClient
    llm     Parser // Same LLM interface as real-time pipeline
}

type Parser interface {
    Enrich(ctx context.Context, prompt speech.Prompt, tr speech.Transcript,
           cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error)
}

func (p *BatchProcessor) ProcessJob(ctx context.Context, job batch.Job) error {
    // Update status to processing
    if err := p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusProcessing,
                                      nil, ""); err != nil {
        return fmt.Errorf("failed to update job status: %w", err)
    }

    result, err := p.processJobInternal(ctx, job)
    if err != nil {
        // Update status to failed
        p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusFailed, nil, err.Error())
        return err
    }

    // Update status to completed
    if err := p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusCompleted,
                                      result, ""); err != nil {
        return fmt.Errorf("failed to update job status: %w", err)
    }

    return nil
}
```

#### File Processing Pipeline

```go
func (p *BatchProcessor) processJobInternal(ctx context.Context,
                                            job batch.Job) (map[string]any, error) {
    // 1. Open and read audio file
    file, err := os.Open(job.FilePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open audio file: %w", err)
    }
    defer file.Close()

    audioData, err := io.ReadAll(file)
    if err != nil {
        return nil, fmt.Errorf("failed to read audio file: %w", err)
    }

    // 2. Create audio chunk channel
    audioIn := make(chan speech.AudioChunk, 1)
    audioIn <- speech.AudioChunk(audioData)
    close(audioIn)

    // 3. Process through STT
    transcriptOut, err := p.stt.Stream(ctx, audioIn)
    if err != nil {
        return nil, fmt.Errorf("failed to start STT stream: %w", err)
    }

    // 4. Collect transcripts
    var finalTranscript speech.Transcript
    for transcript := range transcriptOut {
        finalTranscript = transcript
    }

    // 5. Extract function configuration
    var functionConfig *speech.FunctionConfig
    if funcCfgData, ok := job.Config["function_config"]; ok {
        funcCfgBytes, _ := json.Marshal(funcCfgData)
        functionConfig = &speech.FunctionConfig{}
        json.Unmarshal(funcCfgBytes, functionConfig)
    }

    // 6. Process with LLM (if configured)
    var functionCalls []speech.FunctionCall
    var llmUsage *speech.LLMUsage

    if functionConfig != nil && p.llm != nil {
        prompt := speech.Prompt(fmt.Sprintf(
            "Extract function calls from: %s", finalTranscript.Text))
        functionCalls, llmUsage, err = p.llm.Enrich(ctx, prompt,
                                                     finalTranscript, functionConfig)
        if err != nil {
            log.Printf("LLM processing failed (non-fatal): %v", err)
        }
    }

    // 7. Prepare results
    result := map[string]any{
        "transcript": map[string]any{
            "text":       finalTranscript.Text,
            "confidence": finalTranscript.Confidence,
            "duration":   finalTranscript.ChunkDurSec,
        },
        "functions":    functionCalls,
        "processed_at": time.Now().UTC().Format(time.RFC3339),
        "file_size":    job.FileSize,
    }

    if llmUsage != nil {
        result["usage"] = map[string]any{
            "prompt_tokens":     llmUsage.Prompt,
            "completion_tokens": llmUsage.Completion,
        }
    }

    // 8. Save artifacts (optional)
    if err := p.saveArtifacts(job, finalTranscript, functionCalls); err != nil {
        log.Printf("Failed to save artifacts (non-fatal): %v", err)
    }

    return result, nil
}
```

#### Artifact Management

```go
func (p *BatchProcessor) saveArtifacts(job batch.Job, transcript speech.Transcript,
                                       functions []speech.FunctionCall) error {
    // Create output directory
    outputDir := filepath.Join(os.TempDir(), "schma-batch-results",
                               job.ID.String())
    if err := os.MkdirAll(outputDir, 0755); err != nil {
        return err
    }

    // Save transcript
    transcriptData, _ := json.MarshalIndent(transcript, "", "  ")
    if err := os.WriteFile(filepath.Join(outputDir, "transcript.json"),
                           transcriptData, 0644); err != nil {
        return err
    }

    // Save functions
    functionsData, _ := json.MarshalIndent(functions, "", "  ")
    if err := os.WriteFile(filepath.Join(outputDir, "functions.json"),
                           functionsData, 0644); err != nil {
        return err
    }

    log.Printf("Artifacts saved to: %s", outputDir)
    return nil
}
```

## Infrastructure Layer

### Database Repository (`internal/infra/db/repo/batch_repo.go`)

```go
type BatchRepo struct {
    q *db.Queries // Generated SQLC queries
}

func (r *BatchRepo) Create(ctx context.Context, appID, accountID pgtype.UUID,
                          filePath string, fileSize int64,
                          config map[string]any) (batch.Job, error) {
    // Convert config to JSONB
    configBytes, err := json.Marshal(config)
    if err != nil {
        return batch.Job{}, err
    }

    row, err := r.q.CreateBatchJob(ctx, db.CreateBatchJobParams{
        AppID:     appID,
        AccountID: accountID,
        FilePath:  filePath,
        FileSize:  fileSize,
        Config:    configBytes,
    })
    if err != nil {
        return batch.Job{}, err
    }

    return mapDBJobToDomain(row), nil
}

func (r *BatchRepo) UpdateStatus(ctx context.Context, id pgtype.UUID,
                                 status batch.JobStatus, result map[string]any,
                                 errorMsg string) error {
    var resultBytes []byte
    if result != nil {
        resultBytes, _ = json.Marshal(result)
    }

    return r.q.UpdateBatchJobStatus(ctx, db.UpdateBatchJobStatusParams{
        ID:           id,
        Status:       db.BatchJobStatus(status),
        Result:       resultBytes,
        ErrorMessage: pgtype.Text{String: errorMsg, Valid: errorMsg != ""},
    })
}

func (r *BatchRepo) ListQueued(ctx context.Context, limit int) ([]batch.Job, error) {
    rows, err := r.q.ListQueuedJobs(ctx, int32(limit))
    if err != nil {
        return nil, err
    }

    jobs := make([]batch.Job, len(rows))
    for i, row := range rows {
        jobs[i] = mapDBJobToDomain(row)
    }

    return jobs, nil
}
```

### Data Mapping

```go
func mapDBJobToDomain(row db.BatchJob) batch.Job {
    var config map[string]any
    json.Unmarshal(row.Config, &config)

    var result map[string]any
    if row.Result != nil {
        json.Unmarshal(row.Result, &result)
    }

    return batch.Job{
        ID:           row.ID,
        AppID:        row.AppID,
        AccountID:    row.AccountID,
        Status:       batch.JobStatus(row.Status),
        FilePath:     row.FilePath,
        FileSize:     row.FileSize,
        Config:       config,
        Result:       result,
        ErrorMessage: row.ErrorMessage.String,
        StartedAt:    timeFromPgTimestamp(row.StartedAt),
        CompletedAt:  timeFromPgTimestamp(row.CompletedAt),
        CreatedAt:    row.CreatedAt.Time,
        UpdatedAt:    row.UpdatedAt.Time,
    }
}

func timeFromPgTimestamp(t pgtype.Timestamp) *time.Time {
    if !t.Valid {
        return nil
    }
    return &t.Time
}
```

## HTTP API Integration

### Job Creation Endpoint

```go
func (h *BatchHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
    // Extract principal from auth middleware
    principal, ok := middleware.GetPrincipal(r.Context())
    if !ok {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Parse multipart form
    if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
        http.Error(w, "Failed to parse form", 400)
        return
    }

    file, header, err := r.FormFile("audio")
    if err != nil {
        http.Error(w, "Missing audio file", 400)
        return
    }
    defer file.Close()

    // Save uploaded file
    filePath, err := h.saveUploadedFile(file, header.Filename)
    if err != nil {
        http.Error(w, "Failed to save file", 500)
        return
    }

    // Parse configuration
    configStr := r.FormValue("config")
    var config map[string]any
    if configStr != "" {
        if err := json.Unmarshal([]byte(configStr), &config); err != nil {
            http.Error(w, "Invalid config JSON", 400)
            return
        }
    }

    // Create job
    job, err := h.batchService.CreateJob(r.Context(),
        principal.AppID, principal.AccountID,
        filePath, header.Size, config)
    if err != nil {
        http.Error(w, "Failed to create job", 500)
        return
    }

    // Enqueue for processing
    h.queueManager.EnqueueJob(job.ID.String())

    // Return job details
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "job_id":     job.ID.String(),
        "status":     job.Status,
        "created_at": job.CreatedAt,
    })
}
```

### Job Status Endpoint

```go
func (h *BatchHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
    jobID := r.PathValue("job_id")

    job, err := h.batchService.GetJob(r.Context(), jobID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            http.Error(w, "Job not found", 404)
            return
        }
        http.Error(w, "Failed to get job", 500)
        return
    }

    // Return job with results
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(job)
}
```

### File Upload Management

```go
func (h *BatchHandler) saveUploadedFile(file multipart.File,
                                        filename string) (string, error) {
    // Create upload directory
    uploadDir := filepath.Join(os.TempDir(), "schma-uploads")
    if err := os.MkdirAll(uploadDir, 0755); err != nil {
        return "", err
    }

    // Generate unique filename
    ext := filepath.Ext(filename)
    uniqueName := fmt.Sprintf("%d_%s%s", time.Now().Unix(),
                              generateRandomString(8), ext)
    filePath := filepath.Join(uploadDir, uniqueName)

    // Create destination file
    dst, err := os.Create(filePath)
    if err != nil {
        return "", err
    }
    defer dst.Close()

    // Copy uploaded data
    if _, err := io.Copy(dst, file); err != nil {
        os.Remove(filePath) // Cleanup on failure
        return "", err
    }

    return filePath, nil
}
```

## Configuration & Scaling

### Worker Pool Configuration

```go
type BatchConfig struct {
    NumWorkers       int           // Number of concurrent workers
    QueueBufferSize  int           // Job queue buffer capacity
    MaxFileSize      int64         // Maximum upload file size
    ProcessTimeout   time.Duration // Per-job processing timeout
    CleanupInterval  time.Duration // Artifact cleanup frequency
    UploadDir        string        // File upload directory
    ResultsDir       string        // Processing results directory
}

func DefaultBatchConfig() BatchConfig {
    return BatchConfig{
        NumWorkers:       5,
        QueueBufferSize:  100,
        MaxFileSize:      100 << 20, // 100MB
        ProcessTimeout:   10 * time.Minute,
        CleanupInterval:  24 * time.Hour,
        UploadDir:        "/tmp/schma-uploads",
        ResultsDir:       "/tmp/schma-results",
    }
}
```

### Horizontal Scaling

```go
// Multi-instance coordination using database
func (qm *QueueManager) startWithCoordination(ctx context.Context) error {
    // Register this instance
    instanceID := generateInstanceID()
    if err := qm.registerInstance(ctx, instanceID); err != nil {
        return err
    }

    // Start heartbeat
    go qm.heartbeat(ctx, instanceID)

    // Poll for jobs with instance-based distribution
    go qm.pollWithDistribution(ctx, instanceID)

    return nil
}

func (qm *QueueManager) pollWithDistribution(ctx context.Context,
                                             instanceID string) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Get jobs assigned to this instance
            jobs, err := qm.jobRepo.ListQueuedForInstance(ctx, instanceID, 10)
            if err != nil {
                log.Printf("Failed to get queued jobs: %v", err)
                continue
            }

            // Enqueue jobs locally
            for _, job := range jobs {
                qm.EnqueueJob(job.ID.String())
            }
        }
    }
}
```

## Error Handling & Recovery

### Job Failure Handling

```go
func (p *BatchProcessor) processWithRecovery(ctx context.Context,
                                             job batch.Job) error {
    // Set processing timeout
    ctx, cancel := context.WithTimeout(ctx, p.config.ProcessTimeout)
    defer cancel()

    // Recover from panics
    defer func() {
        if r := recover(); r != nil {
            err := fmt.Errorf("job panic: %v", r)
            p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusFailed,
                                   nil, err.Error())
            log.Printf("Job %s panicked: %v", job.ID.String(), r)
        }
    }()

    return p.ProcessJob(ctx, job)
}
```

### Retry Logic

```go
type RetryableError struct {
    Err     error
    Retries int
    MaxRetries int
}

func (p *BatchProcessor) processWithRetry(ctx context.Context,
                                          job batch.Job) error {
    maxRetries := 3

    for attempt := 1; attempt <= maxRetries; attempt++ {
        err := p.ProcessJob(ctx, job)
        if err == nil {
            return nil // Success
        }

        // Check if error is retryable
        if !isRetryableError(err) {
            return err // Permanent failure
        }

        if attempt < maxRetries {
            delay := time.Duration(attempt) * time.Second
            log.Printf("Job %s failed (attempt %d/%d), retrying in %v: %v",
                       job.ID.String(), attempt, maxRetries, delay, err)

            select {
            case <-time.After(delay):
                continue
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }

    return fmt.Errorf("job failed after %d attempts", maxRetries)
}

func isRetryableError(err error) bool {
    // Network timeouts, temporary STT failures, etc.
    return strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "connection") ||
           strings.Contains(err.Error(), "temporary")
}
```

### Dead Letter Queue

```go
func (p *BatchProcessor) handleFailedJob(ctx context.Context, job batch.Job,
                                         err error) {
    // Update job status
    p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusFailed, nil, err.Error())

    // Move to dead letter storage for investigation
    deadLetterData := map[string]any{
        "job_id":      job.ID.String(),
        "error":       err.Error(),
        "file_path":   job.FilePath,
        "config":      job.Config,
        "failed_at":   time.Now(),
        "worker_id":   os.Getenv("WORKER_ID"),
    }

    if err := p.saveDeadLetter(deadLetterData); err != nil {
        log.Printf("Failed to save dead letter: %v", err)
    }
}
```

## Monitoring & Observability

### Key Metrics

```go
// Job processing metrics
batch_jobs_total{status="queued|processing|completed|failed"}
batch_jobs_duration_seconds{status="completed|failed"}
batch_queue_size
batch_workers_active
batch_processing_errors_total{error_type="stt|llm|file|timeout"}

// Worker pool metrics
batch_worker_utilization_ratio
batch_jobs_per_second
batch_queue_wait_time_seconds

// File processing metrics
batch_file_size_bytes{percentile="50|95|99"}
batch_audio_duration_seconds{percentile="50|95|99"}
```

### Health Monitoring

```go
func (qm *QueueManager) HealthCheck(ctx context.Context) error {
    // Check if workers are responsive
    if !qm.isRunning {
        return errors.New("queue manager not running")
    }

    // Check queue capacity
    queueSize := len(qm.jobQueue)
    if queueSize >= cap(qm.jobQueue)*0.9 {
        return fmt.Errorf("queue nearly full: %d/%d", queueSize, cap(qm.jobQueue))
    }

    // Check worker health
    activeWorkers := qm.countActiveWorkers()
    if activeWorkers == 0 {
        return errors.New("no active workers")
    }

    log.Printf("Batch system healthy: %d active workers, queue=%d/%d",
               activeWorkers, queueSize, cap(qm.jobQueue))

    return nil
}
```

### Logging Examples

```
📁 Batch job created: job=abc123, app=demo-app, file=audio.wav, size=2.1MB
🚀 Job abc123 enqueued for processing (queue_size=5/100)
⚙️ Worker 3: Processing job abc123 (file=audio.wav, config={"functions": 2})
🎤 Job abc123: STT processing completed (duration=45.2s, confidence=0.94)
🤖 Job abc123: LLM processing completed (2 functions extracted, 156 tokens)
✅ Worker 3: Job abc123 completed in 12.3s (transcript=45.2s, functions=2)
📊 Batch stats: processed=156, failed=3, avg_duration=8.7s, queue=12/100
```

## Testing Strategy

### Unit Tests

```go
func TestBatchProcessor_ProcessJob(t *testing.T) {
    processor := NewBatchProcessor(
        &MockJobRepo{},
        &MockSTTClient{},
        &MockLLM{},
    )

    job := batch.Job{
        ID:       uuid.New(),
        FilePath: "test_audio.wav",
        Config:   map[string]any{"test": true},
    }

    // Setup mocks
    mockSTT.On("Stream", mock.Anything, mock.Anything).
        Return(createMockTranscriptChannel(), nil)

    // Test processing
    err := processor.ProcessJob(context.Background(), job)
    assert.NoError(t, err)

    // Verify interactions
    mockSTT.AssertExpectations(t)
}
```

### Integration Tests

```go
func TestBatchSystem_EndToEnd(t *testing.T) {
    // Setup test infrastructure
    db := setupTestDB(t)
    queueManager := NewQueueManager(NewBatchRepo(db), processor, 2)

    // Start queue manager
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    err := queueManager.Start(ctx)
    assert.NoError(t, err)

    // Create test job
    job := createTestJob(t, db)
    queueManager.EnqueueJob(job.ID.String())

    // Wait for completion
    eventually(t, func() bool {
        updated, _ := GetJob(ctx, job.ID)
        return updated.Status == batch.StatusCompleted
    }, 30*time.Second)

    // Verify results
    result, err := GetJob(ctx, job.ID)
    assert.NoError(t, err)
    assert.Equal(t, batch.StatusCompleted, result.Status)
    assert.NotEmpty(t, result.Result)
}
```

### Load Tests

```go
func TestBatchSystem_Concurrency(t *testing.T) {
    queueManager := setupQueueManager(t, 10) // 10 workers

    // Create 100 test jobs
    jobs := make([]batch.Job, 100)
    for i := 0; i < 100; i++ {
        jobs[i] = createTestJob(t)
        queueManager.EnqueueJob(jobs[i].ID.String())
    }

    // Wait for all jobs to complete
    completed := 0
    timeout := time.After(2 * time.Minute)
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for completed < 100 {
        select {
        case <-timeout:
            t.Fatalf("Timeout: only %d/100 jobs completed", completed)
        case <-ticker.C:
            completed = countCompletedJobs(t, jobs)
        }
    }

    // Verify all jobs processed successfully
    for _, job := range jobs {
        result := getJobResult(t, job.ID)
        assert.Equal(t, batch.StatusCompleted, result.Status)
    }
}
```

The batch processing system provides robust, scalable asynchronous processing capabilities with comprehensive error handling, monitoring, and horizontal scaling support.
