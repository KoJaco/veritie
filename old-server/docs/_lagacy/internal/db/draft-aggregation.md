# Draft Function Aggregation System

## Overview

The Draft Function Aggregation (DraftAgg) system provides real-time analytics and insights into function call behavior within Schma.ai sessions. It tracks both draft function detections (from the draft index) and final function calls (from LLM processing), aggregating them into meaningful statistics for performance analysis and user behavior insights.

## How It Works

### Real-Time Flow

```
1. User speaks → STT transcript (partial)
2. Draft Index detects function → AddDraftFunction(name, score, args)
3. STT transcript (final) → LLM processes → Final functions
4. Pipeline tracks → AddFinalFunctions(functions)
5. Every 10s → Aggregations flushed to database
6. Session end → Final aggregation statistics computed
```

### Components

#### 1. Draft Aggregator (`internal/app/usage/draft_aggregator.go`)

-   **Real-time processing**: Handles function events as they occur
-   **In-memory buffering**: Maintains session state for performance
-   **Periodic flushing**: Writes aggregations to database every 10 seconds
-   **Thread-safe**: Concurrent event processing with proper synchronization

#### 2. Usage Accumulator Integration (`internal/app/usage/accumulator.go`)

-   **Dual tracking**: Records both usage events AND aggregations
-   **Lifecycle management**: Coordinates start/stop with draft aggregator
-   **Event logging**: Maintains detailed function call events in `usage_logs`

#### 3. Database Schema (`internal/infra/db/schema.hcl`)

-   **`draft_function_aggs`**: Per-function aggregation data
-   **`draft_function_stats`**: Session-level statistics

## Database Tables

### draft_function_aggs

Stores aggregated data per function per session.

| Column             | Type      | Description                                |
| ------------------ | --------- | ------------------------------------------ |
| `session_id`       | UUID      | Session identifier                         |
| `app_id`           | UUID      | Application identifier                     |
| `account_id`       | UUID      | Account identifier                         |
| `function_name`    | TEXT      | Name of the function                       |
| `total_detections` | BIGINT    | Total times function was detected as draft |
| `highest_score`    | FLOAT8    | Highest similarity score achieved          |
| `avg_score`        | FLOAT8    | Average similarity score                   |
| `first_detected`   | TIMESTAMP | When first detected in session             |
| `last_detected`    | TIMESTAMP | When last detected in session              |
| `sample_args`      | JSONB     | Arguments from highest-scoring detection   |
| `version_count`    | BIGINT    | Number of argument variations              |
| `final_call_count` | BIGINT    | Times this became a final function call    |

**Key Features:**

-   **Unique constraint**: `(session_id, function_name)` - one row per function per session
-   **Smart upserts**: Automatically calculates rolling averages and tracks best examples
-   **Sample capture**: Stores arguments from the highest-scoring detection

### draft_function_stats

Stores session-level aggregation statistics.

| Column                  | Type   | Description                                |
| ----------------------- | ------ | ------------------------------------------ |
| `session_id`            | UUID   | Session identifier (primary key)           |
| `app_id`                | UUID   | Application identifier                     |
| `account_id`            | UUID   | Account identifier                         |
| `total_draft_functions` | BIGINT | Total draft functions detected             |
| `total_final_functions` | BIGINT | Total final functions executed             |
| `draft_to_final_ratio`  | FLOAT8 | Exploration vs execution ratio             |
| `unique_functions`      | BIGINT | Number of unique functions detected        |
| `avg_detection_latency` | FLOAT8 | Average time from draft to final (seconds) |
| `top_function`          | TEXT   | Most frequently detected function          |

## Data Insights Generated

### Per-Function Metrics

-   **Detection Performance**: Total detections, similarity scores
-   **Quality Metrics**: Highest/average similarity scores
-   **Behavioral Patterns**: Time from first draft to final call
-   **Argument Analysis**: Sample captures and variation tracking
-   **Success Rate**: Draft-to-final conversion ratio

### Session-Level Analytics

-   **Exploration Behavior**: Draft-to-final ratio (higher = more explorative)
-   **Function Popularity**: Most frequently detected functions
-   **Diversity Metrics**: Unique function count
-   **Performance Metrics**: Average detection latency
-   **User Patterns**: Function usage distribution

## SQL Queries

### Upsert Function Aggregation

```sql
INSERT INTO draft_function_aggs (
    session_id, app_id, account_id, function_name,
    total_detections, highest_score, avg_score,
    first_detected, last_detected, sample_args,
    version_count, final_call_count
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (session_id, function_name) DO UPDATE SET
    total_detections = draft_function_aggs.total_detections + EXCLUDED.total_detections,
    highest_score = GREATEST(draft_function_aggs.highest_score, EXCLUDED.highest_score),
    avg_score = (draft_function_aggs.avg_score * draft_function_aggs.total_detections +
                 EXCLUDED.avg_score * EXCLUDED.total_detections) /
                (draft_function_aggs.total_detections + EXCLUDED.total_detections),
    last_detected = EXCLUDED.last_detected,
    sample_args = CASE
        WHEN EXCLUDED.highest_score > draft_function_aggs.highest_score
        THEN EXCLUDED.sample_args
        ELSE draft_function_aggs.sample_args
    END,
    version_count = draft_function_aggs.version_count + EXCLUDED.version_count,
    final_call_count = draft_function_aggs.final_call_count + EXCLUDED.final_call_count,
    updated_at = now()
```

**Smart Logic:**

-   **Rolling averages**: Properly weighted calculation for `avg_score`
-   **Best example capture**: Updates `sample_args` only when score improves
-   **Cumulative counters**: Adds to existing detection counts

### Session Statistics

```sql
INSERT INTO draft_function_stats (
    session_id, app_id, account_id,
    total_draft_functions, total_final_functions,
    draft_to_final_ratio, unique_functions,
    avg_detection_latency, top_function
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (session_id) DO UPDATE SET
    total_draft_functions = EXCLUDED.total_draft_functions,
    total_final_functions = EXCLUDED.total_final_functions,
    draft_to_final_ratio = EXCLUDED.draft_to_final_ratio,
    unique_functions = EXCLUDED.unique_functions,
    avg_detection_latency = EXCLUDED.avg_detection_latency,
    top_function = EXCLUDED.top_function,
    updated_at = now()
```

## Business Value

### For Product Analytics

-   **User Behavior Patterns**: Understanding how users explore vs execute functions
-   **Feature Adoption**: Which functions are popular and successful
-   **Performance Optimization**: Detection accuracy and response time insights
-   **User Experience**: Friction points in function discovery and usage

### For Developers

-   **Function Design**: High draft-to-final ratios may indicate unclear function definitions
-   **Model Performance**: Similarity score distributions show detection accuracy
-   **User Intent**: Time from exploration to execution reveals user decision patterns
-   **Quality Assurance**: Detection success rates per function type

### For Operations

-   **Resource Optimization**: Focus development on high-usage functions
-   **Quality Metrics**: Monitor detection success rates and user satisfaction
-   **Debugging Support**: Detailed event trails for troubleshooting issues
-   **Capacity Planning**: Understand function usage patterns for scaling

## Integration Points

### WebSocket Handler (`internal/transport/ws/handler.go`)

```go
// Track draft function detection
case draft := <-outDr:
    usageAccumulator.AddDraftFunction(draft.Name, draft.SimilarityScore, draftArgs)

// Track final function calls
case calls := <-outFn:
    usageAccumulator.AddFinalFunctions(finalFunctions)
```

### Pipeline Integration (`internal/app/pipeline/pipeline.go`)

-   **Automatic initialization**: Draft aggregator created with each session
-   **Dependency injection**: Clean separation of concerns
-   **Lifecycle management**: Coordinated start/stop with usage accumulator

## Configuration

### Flush Intervals

-   **Draft Aggregator**: 10 seconds (configurable)
-   **Usage Accumulator**: 5 seconds for basic metrics
-   **Final Flush**: On session close for complete data

### Buffer Sizes

-   **Event Channel**: 100 events (prevents blocking)
-   **In-memory State**: Per-session function statistics
-   **Async Processing**: Non-blocking event recording

## Monitoring

### Key Metrics to Monitor

-   **Event processing latency**: Time from function call to database flush
-   **Buffer utilization**: Event channel usage patterns
-   **Aggregation accuracy**: Data consistency between events and aggregations
-   **Database performance**: Upsert query execution times

### Health Indicators

-   **No event loss**: Channel buffer should not overflow
-   **Consistent data**: Aggregation counts should match event logs
-   **Timely flushing**: Regular database updates within flush intervals
-   **Memory usage**: Bounded growth of in-memory statistics

## Troubleshooting

### Common Issues

1. **Missing aggregations**: Check draft aggregator start/stop lifecycle
2. **Inconsistent counts**: Verify event channel is not dropping events
3. **Performance issues**: Monitor flush interval and buffer sizes
4. **Data accuracy**: Compare aggregated data with raw usage events

### Debug Queries

```sql
-- Check session aggregation completeness
SELECT s.session_id,
       COUNT(dfa.function_name) as aggregated_functions,
       dfs.unique_functions as expected_functions
FROM draft_function_stats dfs
LEFT JOIN draft_function_aggs dfa ON dfs.session_id = dfa.session_id
WHERE dfs.session_id = $1
GROUP BY s.session_id;

-- Verify event consistency
SELECT function_name,
       COUNT(*) as event_count,
       MAX(dfa.total_detections) as agg_count
FROM usage_logs ul
LEFT JOIN draft_function_aggs dfa ON ul.session_id = dfa.session_id
WHERE ul.type = 'function_call' AND ul.session_id = $1
GROUP BY function_name;
```
