# Dynamic Config Watcher

## Overview

The Dynamic Config Watcher enables per-database-session, real-time updates to the function config (function declarations/schemas and parsing guide) during active audio sessions. This allows the client to adapt the set of functions the LLM is looking for, without disconnecting or restarting the session.

## Architecture

### Simplified Component Structure

1. **WebSocket Handler**: Orchestrates connection state and tracks all database sessions
2. **Pipeline**: Handles individual database session lifecycle (audio start → stop)
3. **Config Watcher**: Tracks config changes within each database session
4. **LLM Session**: Connection-level component (reusable across audio sessions)

### Component Lifecycle

```
WebSocket Connection
├── WebSocket Handler (connection-level)
│   ├── Manages connection state
│   ├── Tracks all database sessions
│   └── Orchestrates pipeline and config watcher
├── Database Sessions (session-level, per audio start → stop)
│   ├── Pipeline (handles session lifecycle)
│   └── Config Watcher (tracks configs for this session)
└── LLM Session (connection-level, reusable)
```

## Protocol

### WebSocket Message Types

#### Client → Server: Dynamic Config Update

```typescript
{
  type: "dynamic_config_update",
  functions: {
    definitions: FunctionDefinition[],
    parsing_guide: string
  }
}
```

#### Server → Client: Config Update Acknowledgment

```typescript
{
  type: "config_update_ack",
  success: boolean,
  message?: string
}
```

## Implementation Flow

### 1. Audio Session Start

```go
// WebSocket handler creates pipeline and config watcher for this database session
func (h *Handler) handleAudioStart(conn *ConnectionState) {
    // Create database session via pipeline
    session, err := h.pipeline.StartSession(ctx, conn.Principal)
    
    // Create config watcher for this database session
    configWatcher := configwatcher.New(h.queries)
    err = configWatcher.StartWatching(ctx, session.ID, initialConfig)
    
    // Store in connection state
    conn.Sessions[session.ID] = &SessionState{
        Pipeline:      pipeline,
        ConfigWatcher: configWatcher,
    }
}
```

### 2. Dynamic Config Update Processing

```go
func (h *Handler) handleDynamicConfigUpdate(conn *ConnectionState, msg DynamicConfigUpdateMessage) {
    sessionID := conn.ActiveSessionID
    
    // Get config watcher for this database session
    sessionState := conn.Sessions[sessionID]
    configWatcher := sessionState.ConfigWatcher
    
    // Update config watcher
    err := configWatcher.UpdateSessionConfig(ctx, sessionID, newConfig)
    
    // Coordinate with pipeline for LLM session rebuilding
    err = configWatcher.RebuildLLMSession(ctx, sessionID)
    
    // Send acknowledgment to client
    conn.WriteJSON(ConfigUpdateAckMessage{...})
}
```

### 3. Audio Session End

```go
func (h *Handler) handleAudioStop(conn *ConnectionState, sessionID string) {
    sessionState := conn.Sessions[sessionID]
    
    // Config watcher flushes configs to database
    err := sessionState.ConfigWatcher.FlushSessionConfigs(ctx, sessionID)
    
    // Pipeline closes database session
    err := sessionState.Pipeline.CloseSession(ctx, sessionID)
    
    // Remove from connection state
    delete(conn.Sessions, sessionID)
}
```

## Storage Strategy

### Config Tracking

- **Function Configs**: Tracked per database session with checksum-based deduplication
- **Session Linking**: Many-to-many relationship tracking all configs used per database session
- **Batch Flushing**: All configs stored together when database session ends

### Database Tables

- `function_schemas`: Schema definitions with checksum-based deduplication
- `session_function_schemas`: Links database sessions to schemas used
- `session_configs`: Links database sessions to configs used

## Example Usage Flow

### Scenario: Adaptive Restaurant Assistant

1. **Audio Session Start**: Client starts recording with initial config

    ```typescript
    functions: {
      definitions: [
        { name: "capture_intent", parameters: [...] }
      ],
      parsing_guide: "Detect user intents"
    }
    ```

2. **Dynamic Config Update**: User says "I want to book a table"

    ```typescript
    {
      type: "dynamic_config_update",
      functions: {
        definitions: [
          { name: "book_table", parameters: [...] },
          { name: "check_availability", parameters: [...] }
        ],
        parsing_guide: "Extract booking details: date, time, party size"
      }
    }
    ```

3. **Real-time Processing**: All subsequent transcripts use booking-specific functions

4. **Audio Session End**: Config watcher flushes all configs used in this database session

## Benefits

### For Developers

- **Adaptive UX**: Functions adapt to conversation context
- **Reduced Reconnections**: No need to restart sessions for config changes
- **Better Performance**: Only relevant functions are active at any time
- **Simplified Architecture**: WebSocket handler orchestrates everything

### For Users

- **Seamless Experience**: No interruptions during config transitions
- **Context-Aware**: System adapts to conversation flow
- **Improved Accuracy**: Focused function sets reduce false positives

## Error Handling

### Validation Errors

- Invalid function definitions → Return error, keep current config
- Malformed parsing guide → Return error, keep current config
- Config storage failure → Log warning, continue with in-memory config

### Fallback Strategy

- If dynamic update fails, session continues with previous config
- All successful config updates are tracked for session-end storage
- Partial failures are logged but don't interrupt the session

## Monitoring

### Metrics to Track

- Config updates per database session
- Config deduplication hit rate
- Update processing latency
- Storage success/failure rates

### Logging

```go
log.Printf("🔄 [CONFIG_WATCHER] Config updated: %d schemas", len(schemas))
log.Printf("✅ [CONFIG_WATCHER] Session %s used %d unique configs", sessionID, len(sessionConfigs))
```

## Future Enhancements

### Config Versioning

- Track config evolution within database sessions
- Enable rollback to previous configurations
- Version-aware conflict resolution

### Performance Optimizations

- Config caching across database sessions
- Predictive config loading
- Batch update processing

### Advanced Features

- Config inheritance and composition
- Conditional config activation
- Client-side config validation
