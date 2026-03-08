# Dynamic Schema Watcher

## Overview

The Dynamic Schema Watcher enables real-time hot-swapping of function definitions during active WebSocket sessions without requiring client reconnection. This system ensures that all connected sessions remain synchronized with the latest function schemas, providing a seamless experience when function definitions are updated.

## Business Rationale

### Why Dynamic Schema Watching is Essential

#### 1. **Real-World Use Case: Functions Change During Sessions**

In production environments, function definitions frequently change while users are actively connected:

-   **Admin Updates**: Administrators modify function definitions to add features or fix issues
-   **User-Triggered Changes**: User actions trigger new capabilities (e.g., "add calendar integration")
-   **A/B Testing**: Different function sets deployed to different user cohorts
-   **Progressive Rollouts**: New functions gradually enabled across user base

**Without dynamic watching**: Sessions use outdated schemas, leading to:

-   ❌ Missed opportunities for new features
-   ❌ Incorrect parsing or function call failures
-   ❌ Inconsistent user experience across sessions
-   ❌ Manual reconnection required for updates

**With dynamic watching**: Sessions automatically receive updates:

-   ✅ Instant access to new functions
-   ✅ Consistent parsing and LLM behavior
-   ✅ Zero-downtime user experience
-   ✅ Seamless feature rollouts

#### 2. **LLM/Parser Tooling Synchronization**

The LLM (Gemini) and fast parser both depend on current function schemas for:

```
User Speech → STT → Parser → LLM → Function Calls
                ↑        ↑
          Function    Function
          Schemas     Schemas
```

**Schema Drift Problems**:

-   **LLM Hallucination**: Suggests functions that no longer exist
-   **Missing Functions**: Fails to suggest newly available functions
-   **Parser Errors**: Cannot match speech to outdated function definitions
-   **Inconsistent Results**: Different sessions produce different outputs

**Synchronized Tooling Benefits**:

-   **Accurate Parsing**: All sessions use identical, current schemas
-   **Consistent LLM Behavior**: Reliable function suggestions across sessions
-   **Error Prevention**: No outdated function references
-   **Quality Assurance**: Predictable system behavior

#### 3. **Zero-Downtime Real-Time Experience**

Modern users expect instant feature availability:

```
Traditional Approach:
Admin Updates → Users Reconnect → New Features Available
    ↓              ↓                    ↓
  Instant       Manual Step         Delayed Access

Dynamic Approach:
Admin Updates → Auto Hot-Swap → Instant Feature Access
    ↓              ↓                ↓
  Instant      Automatic          Immediate
```

**User Experience Benefits**:

-   **No Interruption**: Continuous conversation flow
-   **Instant Gratification**: New features available immediately
-   **Professional Feel**: Enterprise-grade real-time updates
-   **Reduced Support**: No "refresh to see changes" instructions

#### 4. **Multi-Client and Multi-Tenant Environments**

In collaborative environments, schema changes affect multiple concurrent sessions:

```
Multi-User Scenario:
Admin (Session A) → Updates Schema
User 1 (Session B) → Needs new schema instantly
User 2 (Session C) → Needs new schema instantly
User 3 (Session D) → Needs new schema instantly
```

**Consistency Requirements**:

-   **Global State**: All sessions must use identical schemas
-   **Atomic Updates**: No partial or mixed schema states
-   **Race Condition Prevention**: Coordinated schema propagation
-   **Fair Distribution**: All sessions receive updates equally

#### 5. **Operational Excellence**

**Observability Benefits**:

-   **Schema Version Tracking**: Know which sessions use which schemas
-   **Update Audit Trail**: Track when and how schemas changed
-   **Performance Monitoring**: Measure hot-swap latency and success rates
-   **Debugging Support**: Correlate issues with schema changes

**Safety Benefits**:

-   **Rollback Capability**: Quickly revert problematic schema changes
-   **Graceful Degradation**: Handle schema update failures safely
-   **Validation**: Ensure new schemas are valid before propagation
-   **Error Isolation**: Prevent schema issues from affecting all sessions

## Technical Architecture

### System Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Database      │    │  Schema Watcher  │    │  WebSocket      │
│                 │    │                  │    │  Sessions       │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │ ┌─────────────┐ │
│ │ function_   │ │    │ │   Polling    │ │    │ │ Session A   │ │
│ │ schemas     │ │◄───┤ │   Service    │ ├───►│ │ (v1.2.3)    │ │
│ │             │ │    │ │              │ │    │ │             │ │
│ │ - checksum  │ │    │ └──────────────┘ │    │ └─────────────┘ │
│ │ - version   │ │    │                  │    │                 │
│ │ - created   │ │    │ ┌──────────────┐ │    │ ┌─────────────┐ │
│ └─────────────┘ │    │ │  Change      │ │    │ │ Session B   │ │
└─────────────────┘    │ │  Detector    │ │    │ │ (v1.2.2)    │ │
                       │ │              │ │    │ │ ← needs     │ │
┌─────────────────┐    │ └──────────────┘ │    │ │   update    │ │
│   Admin API     │    │                  │    │ └─────────────┘ │
│                 │    │ ┌──────────────┐ │    └─────────────────┘
│ ┌─────────────┐ │    │ │  Session     │ │
│ │ Schema      │ │    │ │  Notifier    │ │
│ │ Management  │ ├───►│ │              │ │
│ │ API         │ │    │ └──────────────┘ │
│ └─────────────┘ │    └──────────────────┘
└─────────────────┘
```

### Core Interfaces

```go
// Domain layer interfaces
type SchemaWatcher interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Subscribe(sessionID string) <-chan SchemaUpdate
    Unsubscribe(sessionID string)
    GetCurrentVersion() string
}

type SchemaUpdate struct {
    Version     string
    Schemas     []FunctionSchema
    Checksum    string
    UpdatedAt   time.Time
    ChangeType  ChangeType // Added, Modified, Removed
}

type ChangeType string
const (
    ChangeTypeAdded    ChangeType = "added"
    ChangeTypeModified ChangeType = "modified"
    ChangeTypeRemoved  ChangeType = "removed"
)
```

### Polling Strategy

#### Database Schema Versioning

```sql
-- Function schemas table with versioning
CREATE TABLE function_schemas (
    id UUID PRIMARY KEY,
    app_id UUID NOT NULL,
    name TEXT NOT NULL,
    parameters JSONB NOT NULL,
    checksum TEXT NOT NULL,
    version TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Schema versions table for atomic updates
CREATE TABLE schema_versions (
    app_id UUID PRIMARY KEY,
    version TEXT NOT NULL,
    checksum TEXT NOT NULL,
    schema_count INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Efficient Change Detection

```go
// Polling query optimized for change detection
func (w *Watcher) checkForUpdates(ctx context.Context, appID string) (*SchemaUpdate, error) {
    // 1. Quick version check (lightweight query)
    var latestVersion, latestChecksum string
    err := w.db.QueryRow(ctx, `
        SELECT version, checksum
        FROM schema_versions
        WHERE app_id = $1
    `, appID).Scan(&latestVersion, &latestChecksum)

    // 2. Only fetch full schemas if version changed
    if latestVersion != w.currentVersion[appID] {
        return w.fetchFullSchemas(ctx, appID, latestVersion)
    }

    return nil, nil // No changes
}
```

### Hot-Swap Implementation Strategy

#### 1. **LLM Session Management**

```go
// Gemini session hot-swap
func (s *SessionManager) updateLLMTools(update SchemaUpdate) error {
    // 1. Wait for current LLM call to complete
    s.llmMutex.Lock()
    defer s.llmMutex.Unlock()

    // 2. Create new Gemini session with updated tools
    newSession, err := llmgemini.NewSession(s.apiKey, s.model)
    if err != nil {
        return fmt.Errorf("failed to create new LLM session: %w", err)
    }

    // 3. Configure new tools
    if err := newSession.ConfigureOnce(update.Schemas, s.parsingGuide); err != nil {
        return fmt.Errorf("failed to configure LLM tools: %w", err)
    }

    // 4. Atomic swap
    oldSession := s.llmSession
    s.llmSession = newSession

    // 5. Cleanup old session
    if oldSession != nil {
        oldSession.Close()
    }

    log.Printf("🔄 LLM tools updated to version %s (%d functions)",
        update.Version, len(update.Schemas))

    return nil
}
```

#### 2. **Fast Parser Index Rebuild**

```go
// Draft function index hot-swap
func (s *SessionManager) updateDraftIndex(update SchemaUpdate) error {
    // 1. Build new index in background
    newIndex, err := draft.New(
        update.Schemas,
        s.fastParser,
        s.modelDir,
    )
    if err != nil {
        return fmt.Errorf("failed to build new draft index: %w", err)
    }

    // 2. Atomic swap
    s.indexMutex.Lock()
    oldIndex := s.draftIndex
    s.draftIndex = newIndex
    s.indexMutex.Unlock()

    // 3. Cleanup old index
    if oldIndex != nil {
        oldIndex.Close()
    }

    log.Printf("🔄 Draft index updated to version %s", update.Version)
    return nil
}
```

#### 3. **Pipeline Coordination**

```go
// Coordinated pipeline update
func (p *Pipeline) updateSchemas(update SchemaUpdate) error {
    // 1. Pause new processing
    p.pauseProcessing()
    defer p.resumeProcessing()

    // 2. Wait for in-flight operations
    p.waitForCompletion()

    // 3. Update all components atomically
    if err := p.updateLLMTools(update); err != nil {
        return err
    }

    if err := p.updateDraftIndex(update); err != nil {
        return err
    }

    if err := p.updatePromptTemplate(update); err != nil {
        return err
    }

    // 4. Update version tracking
    p.currentSchemaVersion = update.Version

    return nil
}
```

## Client Protocol Integration

### Schema Version Headers

```go
// WebSocket config message with schema version
type ConfigMessage struct {
    Type          string            `json:"type"`
    SessionID     string            `json:"session_id"`
    SchemaVersion string            `json:"schema_version,omitempty"`
    Functions     FunctionConfig    `json:"function_config"`
    // ... other fields
}
```

### Schema Update Notifications

```json
// Server notifies client of schema update
{
    "type": "schema_update",
    "old_version": "v1.2.2",
    "new_version": "v1.2.3",
    "changes": [
        {
            "type": "added",
            "function": "create_calendar_event",
            "description": "Create calendar events"
        },
        {
            "type": "modified",
            "function": "send_email",
            "description": "Updated email validation rules"
        }
    ],
    "timestamp": "2025-01-15T10:30:00Z"
}
```

### Client Acknowledgment

```json
// Client acknowledges schema update
{
    "type": "schema_ack",
    "version": "v1.2.3",
    "status": "applied"
}
```

## Performance Considerations

### Polling Efficiency

```go
type WatcherConfig struct {
    // Polling frequency (default: 5 seconds)
    PollInterval time.Duration

    // Batch size for schema fetching
    BatchSize int

    // Timeout for database operations
    DBTimeout time.Duration

    // Maximum concurrent session updates
    MaxConcurrentUpdates int
}

func DefaultWatcherConfig() WatcherConfig {
    return WatcherConfig{
        PollInterval:         5 * time.Second,
        BatchSize:           100,
        DBTimeout:           10 * time.Second,
        MaxConcurrentUpdates: 50,
    }
}
```

### Update Batching

```go
// Batch updates to reduce database load
func (w *Watcher) batchUpdates() {
    ticker := time.NewTicker(w.config.PollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Check all apps in single query
            updates := w.checkAllAppsForUpdates()

            // Batch notify sessions by app
            for appID, update := range updates {
                w.notifyAppSessions(appID, update)
            }

        case <-w.ctx.Done():
            return
        }
    }
}
```

### Memory Management

```go
// Efficient session tracking
type SessionTracker struct {
    mu       sync.RWMutex
    sessions map[string]*SessionInfo
}

type SessionInfo struct {
    AppID           string
    CurrentVersion  string
    UpdateChannel   chan SchemaUpdate
    LastUpdateTime  time.Time
}

// Cleanup stale sessions
func (st *SessionTracker) cleanup() {
    st.mu.Lock()
    defer st.mu.Unlock()

    cutoff := time.Now().Add(-1 * time.Hour)
    for sessionID, info := range st.sessions {
        if info.LastUpdateTime.Before(cutoff) {
            close(info.UpdateChannel)
            delete(st.sessions, sessionID)
        }
    }
}
```

## Error Handling & Edge Cases

### Schema Validation

```go
func (w *Watcher) validateSchemaUpdate(update SchemaUpdate) error {
    // 1. JSON schema validation
    for _, schema := range update.Schemas {
        if err := validateFunctionSchema(schema); err != nil {
            return fmt.Errorf("invalid schema %s: %w", schema.Name, err)
        }
    }

    // 2. Checksum verification
    computedChecksum := computeSchemaChecksum(update.Schemas)
    if computedChecksum != update.Checksum {
        return fmt.Errorf("checksum mismatch: expected %s, got %s",
            update.Checksum, computedChecksum)
    }

    // 3. Version ordering
    if !isValidVersionProgression(w.currentVersion, update.Version) {
        return fmt.Errorf("invalid version progression: %s -> %s",
            w.currentVersion, update.Version)
    }

    return nil
}
```

### Rollback Capability

```go
func (w *Watcher) rollbackSchema(sessionID string, targetVersion string) error {
    // 1. Fetch historical schema version
    schemas, err := w.fetchSchemaVersion(targetVersion)
    if err != nil {
        return fmt.Errorf("failed to fetch rollback version: %w", err)
    }

    // 2. Create rollback update
    rollbackUpdate := SchemaUpdate{
        Version:    targetVersion,
        Schemas:    schemas,
        UpdatedAt:  time.Now(),
        ChangeType: ChangeTypeModified,
    }

    // 3. Apply rollback
    return w.applyUpdate(sessionID, rollbackUpdate)
}
```

### Graceful Degradation

```go
func (w *Watcher) handleUpdateFailure(sessionID string, err error) {
    log.Printf("⚠️ Schema update failed for session %s: %v", sessionID, err)

    // 1. Mark session as degraded
    w.markSessionDegraded(sessionID)

    // 2. Continue with old schema
    w.notifySessionDegraded(sessionID, err)

    // 3. Schedule retry
    w.scheduleRetry(sessionID, backoffDuration(err))
}
```

## Testing Strategy

### Unit Tests

```go
func TestSchemaWatcher_DetectsChanges(t *testing.T) {
    // Test change detection logic
    watcher := NewMockWatcher()

    // Initial state
    assert.Equal(t, "v1.0.0", watcher.GetCurrentVersion())

    // Simulate schema change
    watcher.SimulateSchemaChange("v1.1.0")

    // Verify detection
    update := <-watcher.Updates()
    assert.Equal(t, "v1.1.0", update.Version)
}

func TestHotSwap_AtomicUpdate(t *testing.T) {
    // Test atomic LLM tool updates
    session := NewTestSession()

    // Verify no partial updates occur
    update := createTestUpdate()
    err := session.ApplySchemaUpdate(update)

    assert.NoError(t, err)
    assert.Equal(t, update.Version, session.GetSchemaVersion())
}
```

### Integration Tests

```go
func TestEndToEnd_SchemaUpdate(t *testing.T) {
    // Test full schema update flow
    server := startTestServer()
    client := connectTestClient()

    // 1. Initial connection
    client.SendConfig(initialSchema)

    // 2. Admin updates schema
    server.UpdateSchema(newSchema)

    // 3. Verify client receives update
    update := client.WaitForSchemaUpdate(5 * time.Second)
    assert.Equal(t, newSchema.Version, update.Version)

    // 4. Verify function calls use new schema
    response := client.SendSpeech("create calendar event")
    assert.Contains(t, response.Functions, "create_calendar_event")
}
```

### Performance Tests

```go
func BenchmarkSchemaWatcher_PollingPerformance(b *testing.B) {
    watcher := NewWatcher(testDB)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        watcher.CheckForUpdates("test-app-id")
    }
}

func TestConcurrentUpdates_RaceConditions(t *testing.T) {
    // Test multiple sessions updating simultaneously
    sessions := make([]*TestSession, 100)

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            sessions[idx].ApplySchemaUpdate(testUpdate)
        }(i)
    }

    wg.Wait()

    // Verify all sessions have consistent state
    for _, session := range sessions {
        assert.Equal(t, testUpdate.Version, session.GetSchemaVersion())
    }
}
```

## Monitoring & Observability

### Key Metrics

```go
type WatcherMetrics struct {
    // Performance metrics
    PollLatency        *prometheus.HistogramVec // Database poll latency
    UpdateLatency      *prometheus.HistogramVec // Schema update application time

    // Update metrics
    UpdatesDetected    *prometheus.CounterVec   // Schema changes detected
    UpdatesApplied     *prometheus.CounterVec   // Successful updates
    UpdatesFailed      *prometheus.CounterVec   // Failed updates

    // Session metrics
    ActiveSessions     *prometheus.GaugeVec     // Sessions per app
    SessionsUpdated    *prometheus.CounterVec   // Sessions receiving updates

    // Error metrics
    ValidationErrors   *prometheus.CounterVec   // Schema validation failures
    RollbacksTriggered *prometheus.CounterVec   // Emergency rollbacks
}
```

### Logging Examples

```
🔄 Schema watcher started (poll_interval=5s, apps=12)
🔍 Checking schema updates for app: demo-app (current_version=v1.2.2)
📝 Schema change detected: demo-app v1.2.2 → v1.2.3 (2 functions added)
🚀 Updating 5 active sessions to schema v1.2.3
✅ Schema update completed for session abc123 (latency=45ms)
⚠️ Schema update failed for session xyz789: LLM tool configuration error
🔄 LLM tools updated to version v1.2.3 (12 functions)
🔄 Draft index updated to version v1.2.3
📊 Schema watcher stats: 156 polls, 12 updates, 0 failures (uptime: 2h15m)
```

## Deployment Considerations

### Feature Flags

```go
type FeatureFlags struct {
    EnableSchemaWatcher    bool
    EnableHotSwap         bool
    EnableClientUpdates   bool
    EnableRollback        bool
}

// Gradual rollout support
func (w *Watcher) shouldUpdateSession(sessionID string) bool {
    if !w.flags.EnableHotSwap {
        return false
    }

    // Percentage rollout
    return w.rolloutPercent.ShouldInclude(sessionID)
}
```

### Database Migration

```sql
-- Migration for schema versioning support
ALTER TABLE function_schemas ADD COLUMN version TEXT DEFAULT 'v1.0.0';
ALTER TABLE function_schemas ADD COLUMN checksum TEXT;

-- Indexes for efficient polling
CREATE INDEX idx_function_schemas_app_version ON function_schemas(app_id, version);
CREATE INDEX idx_schema_versions_updated_at ON schema_versions(updated_at);

-- Trigger for automatic version updates
CREATE OR REPLACE FUNCTION update_schema_version()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE schema_versions
    SET version = generate_version(),
        checksum = compute_checksum(NEW.app_id),
        updated_at = NOW()
    WHERE app_id = NEW.app_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

The Dynamic Schema Watcher provides essential infrastructure for maintaining real-time consistency between function definitions and active sessions, enabling seamless feature rollouts and ensuring optimal user experience in production environments.
