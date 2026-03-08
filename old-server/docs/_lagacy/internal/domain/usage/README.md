# Usage Domain Documentation

## Overview

The usage domain handles resource consumption tracking, cost calculation, and billing analytics for the Schma system. It provides comprehensive metering for audio processing, LLM usage, CPU utilization, and draft function detection.

## Core Concepts

### Resource Metering

-   **Audio Processing**: Tracks STT processing time in seconds
-   **LLM Usage**: Monitors prompt and completion token consumption
-   **CPU Utilization**: Separates active and idle CPU time
-   **Draft Detection**: Tracks function detection patterns and accuracy

### Cost Calculation

-   **Real-time Pricing**: Dynamic cost calculation based on current rates
-   **Multi-currency Support**: Support for USD, AUD, and other currencies
-   **Idle Discounting**: Reduced rates for idle CPU time
-   **Savings Tracking**: Tracks cost savings from caching and optimizations

### Analytics

-   **Session Totals**: Aggregated usage data per session
-   **Event Logging**: Detailed event-level usage tracking
-   **Draft Analytics**: Function detection accuracy and patterns
-   **Performance Metrics**: Latency and efficiency measurements

## Data Structures

### Meter

```go
type Meter struct {
    SessionID pgtype.UUID
    AccountID pgtype.UUID
    AppID     pgtype.UUID

    // accumulators
    StartedAt        time.Time
    AudioSeconds     float64 // STT billed seconds
    PromptTokens     int64   // LLM in
    CompletionTokens int64   // LLM out
    CPUActiveSeconds float64 // fly app
    CPUIdleSeconds   float64

    // pricing ref
    pr Pricing
}
```

**Purpose**: Tracks resource consumption during a session.

**Fields:**

-   `SessionID/AccountID/AppID`: Identifiers for the session
-   `StartedAt`: Session start timestamp
-   `AudioSeconds`: Total STT processing time
-   `PromptTokens/CompletionTokens`: LLM token usage
-   `CPUActiveSeconds/CPUIdleSeconds`: CPU utilization breakdown
-   `pr`: Pricing reference for cost calculations

### Pricing

```go
type Pricing struct {
    Currency               string
    CostAudioPerMin        float64 // $/ min for STT
    CostGemPromptPer1M     float64 // $/ token for LLM Input
    CostGemCompletionPer1M float64 // $/ token for LLM Output
    CostFlyPerSec          float64 // $/ second for server runtime
    IdleDiscount           float64 // (e.g. 0.10 = 90% disc)
}
```

**Purpose**: Defines cost rates for different resources.

**Default Pricing:**

```go
var DefaultPricing = Pricing{
    Currency:               "USD",
    CostAudioPerMin:        0.0077, // Deepgram streaming PAYG
    CostGemPromptPer1M:     0.15,   // Gemini 2.5 flash
    CostGemCompletionPer1M: 0.6,    // Gemini 2.5 flash
    CostFlyPerSec:          0.00000095, // Fly.io pricing
    IdleDiscount:           0.1,    // 10% discount for idle time
}
```

### Cost

```go
type Cost struct {
    AudioCost, LLMInCost, LLMOutCost, CPUCost float64
    TotalCost                                 float64
}
```

**Purpose**: Breakdown of costs by resource type.

### UsageEvent

```go
type UsageEvent struct {
    SessionID pgtype.UUID
    AppID     pgtype.UUID
    AccountID pgtype.UUID
    Type      string      // e.g., 'stt', 'llm', 'function_call'
    Metric    interface{} // Flexible data: tokens, duration, latency, cost, etc.
    LoggedAt  time.Time
}
```

**Purpose**: Individual usage events for detailed analytics.

## Draft Aggregation

### DraftAgg

```go
type DraftAgg struct {
    SessionID       pgtype.UUID
    AppID           pgtype.UUID
    AccountID       pgtype.UUID
    FunctionName    string
    TotalDetections int64       // Total number of times this function was detected
    HighestScore    float64     // Highest similarity score achieved
    AvgScore        float64     // Average similarity score
    FirstDetected   time.Time   // When first detected
    LastDetected    time.Time   // When last detected
    SampleArgs      interface{} // Sample arguments from highest scoring detection
    VersionCount    int64       // Number of different argument variations
    FinalCallCount  int64       // Number of times this became a final function call
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**Purpose**: Aggregated data for draft function detection patterns.

### DraftAggStats

```go
type DraftAggStats struct {
    SessionID           pgtype.UUID
    AppID               pgtype.UUID
    AccountID           pgtype.UUID
    TotalDraftFunctions int64   // Total draft functions detected
    TotalFinalFunctions int64   // Total final functions executed
    DraftToFinalRatio   float64 // Ratio of drafts to finals (higher = more explorative)
    UniqueFunction      int64   // Number of unique functions detected
    AvgDetectionLatency float64 // Average time from draft to final (seconds)
    TopFunction         string  // Most frequently detected function
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

**Purpose**: Session-level statistics for draft function analytics.

## Interfaces

### UsageMeterRepo

```go
type UsageMeterRepo interface {
    Save(ctx context.Context, meter Meter, c Cost, savedPromptTokens int64, savedPromptCost float64) (db.SessionUsageTotal, error)
}
```

**Purpose**: Persists session usage totals and cost calculations.

**Methods:**

-   `Save`: Saves usage data in an idempotent transaction
    -   `meter`: Current usage meter state
    -   `c`: Calculated costs
    -   `savedPromptTokens`: Tokens saved through caching
    -   `savedPromptCost`: Cost savings from caching
    -   Returns: Database record and error

### UsageEventRepo

```go
type UsageEventRepo interface {
    LogEvent(ctx context.Context, event UsageEvent) error
    ListEventsBySession(ctx context.Context, sessionID pgtype.UUID) ([]UsageEvent, error)
}
```

**Purpose**: Manages detailed usage event logging.

**Methods:**

-   `LogEvent`: Records individual usage events
-   `ListEventsBySession`: Retrieves all events for a session

### DraftAggRepo

```go
type DraftAggRepo interface {
    UpsertDraftAgg(ctx context.Context, agg DraftAgg) error
    GetDraftAggsBySession(ctx context.Context, sessionID pgtype.UUID) ([]DraftAgg, error)
    UpsertDraftAggStats(ctx context.Context, stats DraftAggStats) error
    GetDraftAggStats(ctx context.Context, sessionID pgtype.UUID) (DraftAggStats, error)
}
```

**Purpose**: Manages draft function aggregation data.

**Methods:**

-   `UpsertDraftAgg`: Updates or inserts draft aggregation data
-   `GetDraftAggsBySession`: Retrieves draft aggregations for a session
-   `UpsertDraftAggStats`: Updates session-level statistics
-   `GetDraftAggStats`: Retrieves session statistics

## Usage Examples

### Creating and Using a Meter

```go
// Create a new meter with default pricing
meter := usage.NewMeter(usage.DefaultPricing)

// Track audio processing
meter.AddSTT(10.5) // 10.5 seconds of audio

// Track LLM usage
meter.AddTokens(100, 50) // 100 prompt + 50 completion tokens

// Track CPU usage
meter.AddCPUActive(5 * time.Second)  // 5 seconds active
meter.AddCPUIdle(10 * time.Second)   // 10 seconds idle

// Calculate costs
cost := meter.CostUSD()
fmt.Printf("Total cost: $%.4f\n", cost)
```

### Cost Calculation

```go
// Calculate cost with default pricing
cost := meter.CostUSD()

// Calculate cost with custom pricing
customPricing := usage.Pricing{
    Currency:               "USD",
    CostAudioPerMin:        0.01,
    CostGemPromptPer1M:     0.20,
    CostGemCompletionPer1M: 0.80,
    CostFlyPerSec:          0.000001,
    IdleDiscount:           0.15,
}
cost := meter.Cost(customPricing)

// Access cost breakdown
fmt.Printf("Audio: $%.4f\n", cost.AudioCost)
fmt.Printf("LLM Input: $%.4f\n", cost.LLMInCost)
fmt.Printf("LLM Output: $%.4f\n", cost.LLMOutCost)
fmt.Printf("CPU: $%.4f\n", cost.CPUCost)
fmt.Printf("Total: $%.4f\n", cost.TotalCost)
```

### Usage Event Logging

```go
// Log STT usage event
event := usage.UsageEvent{
    SessionID: sessionID,
    AppID:     appID,
    AccountID: accountID,
    Type:      "stt",
    Metric: map[string]interface{}{
        "duration_seconds": 10.5,
        "provider":         "deepgram",
        "confidence":       0.95,
    },
    LoggedAt: time.Now(),
}
err := eventRepo.LogEvent(ctx, event)

// Log LLM usage event
llmEvent := usage.UsageEvent{
    SessionID: sessionID,
    AppID:     appID,
    AccountID: accountID,
    Type:      "llm",
    Metric: map[string]interface{}{
        "prompt_tokens":    100,
        "completion_tokens": 50,
        "provider":         "gemini",
        "model":            "gemini-2.0-flash",
        "cached":           false,
    },
    LoggedAt: time.Now(),
}
err = eventRepo.LogEvent(ctx, llmEvent)
```

### Draft Aggregation

```go
// Create draft aggregation
agg := usage.DraftAgg{
    SessionID:       sessionID,
    AppID:           appID,
    AccountID:       accountID,
    FunctionName:    "create_task",
    TotalDetections: 5,
    HighestScore:    0.95,
    AvgScore:        0.87,
    FirstDetected:   time.Now().Add(-10 * time.Minute),
    LastDetected:    time.Now(),
    SampleArgs:      map[string]interface{}{"title": "Sample task"},
    VersionCount:    3,
    FinalCallCount:  2,
}

// Save draft aggregation
err := draftRepo.UpsertDraftAgg(ctx, agg)

// Create session statistics
stats := usage.DraftAggStats{
    SessionID:           sessionID,
    AppID:               appID,
    AccountID:           accountID,
    TotalDraftFunctions: 15,
    TotalFinalFunctions: 8,
    DraftToFinalRatio:   1.875, // 15/8
    UniqueFunction:      5,
    AvgDetectionLatency: 2.5,
    TopFunction:         "create_task",
}

// Save session statistics
err = draftRepo.UpsertDraftAggStats(ctx, stats)
```

## Cost Calculation Details

### Audio Cost

```go
audioCost := (audioSeconds / 60) * costPerMinute
```

### LLM Cost

```go
promptCost := (promptTokens / 1_000_000) * costPerMillionPromptTokens
completionCost := (completionTokens / 1_000_000) * costPerMillionCompletionTokens
```

### CPU Cost

```go
activeCost := cpuActiveSeconds * costPerSecond
idleCost := cpuIdleSeconds * costPerSecond * idleDiscount
totalCPUCost := activeCost + idleCost
```

### Total Cost

```go
totalCost := audioCost + promptCost + completionCost + totalCPUCost
```

## Analytics and Insights

### Draft Function Analytics

-   **Detection Accuracy**: Track how often drafts become final calls
-   **Exploration Patterns**: Identify functions with high draft-to-final ratios
-   **Performance Optimization**: Find functions with high detection latency
-   **Usage Patterns**: Understand which functions are most commonly detected

### Cost Optimization

-   **Caching Impact**: Track cost savings from LLM caching
-   **Provider Comparison**: Compare costs across different STT/LLM providers
-   **Usage Trends**: Identify patterns in resource consumption
-   **Efficiency Metrics**: Monitor cost per function call or session

### Performance Monitoring

-   **Latency Tracking**: Monitor time from draft to final function call
-   **Error Rates**: Track failed function detections
-   **Resource Utilization**: Monitor CPU and memory usage patterns
-   **Session Analytics**: Analyze session duration and efficiency

## Implementation Notes

### Thread Safety

-   **Concurrent Access**: Meters must be thread-safe for concurrent updates
-   **Atomic Operations**: Use atomic operations for counter updates
-   **Memory Management**: Efficient memory usage for long-running sessions

### Performance Considerations

-   **Batch Processing**: Batch usage events for efficient database writes
-   **Caching**: Cache pricing data to avoid repeated calculations
-   **Compression**: Compress usage data for storage efficiency

### Monitoring and Alerting

-   **Cost Thresholds**: Alert when costs exceed thresholds
-   **Usage Anomalies**: Detect unusual usage patterns
-   **Performance Degradation**: Monitor for performance issues
-   **Resource Exhaustion**: Alert when approaching resource limits

## Future Enhancements

### Planned Features

-   **Real-time Billing**: Real-time cost updates during sessions
-   **Predictive Analytics**: Predict costs based on usage patterns
-   **Cost Optimization**: Automatic provider selection based on cost
-   **Advanced Reporting**: Detailed cost and usage reports

### Extension Points

-   **Custom Pricing**: Support for custom pricing models
-   **Multi-currency**: Enhanced multi-currency support
-   **Usage Quotas**: Implement usage quotas and limits
-   **Cost Allocation**: Advanced cost allocation features
