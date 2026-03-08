# Session Domain Documentation

## Overview

The session domain manages the lifecycle and state of voice processing sessions in the Schma system. It provides a clean interface for tracking session state, managing configuration changes, and handling session events.

## Core Concepts

### Session Hierarchy

-   **WebSocket Session (WSSessionID)**: Represents a client connection
-   **Database Session (DBSessionID)**: Represents individual pipeline sessions within a connection
-   **Session State**: Tracks the current state and configuration of a session

### Session Lifecycle

Sessions progress through different states:

1. **Idle**: Session created but not yet processing
2. **Recording**: Actively processing audio
3. **Processing**: LLM processing in progress
4. **Closed**: Session completed or terminated

### Configuration Tracking

Sessions track function configurations over time, allowing for:

-   **Dynamic updates**: Configuration changes during active sessions
-   **History tracking**: Complete audit trail of configuration changes
-   **Schema linking**: Association with database schemas via checksums

## Data Structures

### Session Identifiers

```go
type WSSessionID string      // WebSocket connection identifier
type DBSessionID pgtype.UUID // Database session identifier
```

### Session Status

```go
type SessionStatus string

const (
    SessionIdle       SessionStatus = "idle"
    SessionRecording  SessionStatus = "recording"
    SessionProcessing SessionStatus = "processing"
    SessionClosed     SessionStatus = "closed"
)
```

### Session State

```go
type SessionState struct {
    ID           DBSessionID
    WSSessionID  WSSessionID
    Status       SessionStatus
    Principal    auth.Principal
    CreatedAt    time.Time
    LastActivity time.Time

    // Session configuration history
    FunctionConfigs []TrackedFunctionConfig // Multiple configs over time
    CurrentConfig   *TrackedFunctionConfig  // Current active config

    // Schema tracking
    SchemaChecksums map[string]pgtype.UUID // checksum -> schema_id
}
```

**Fields:**

-   `ID`: Unique database session identifier
-   `WSSessionID`: Associated WebSocket connection
-   `Status`: Current session state
-   `Principal`: Authentication principal
-   `CreatedAt/LastActivity`: Timestamps for lifecycle tracking
-   `FunctionConfigs`: History of configuration changes
-   `CurrentConfig`: Currently active configuration
-   `SchemaChecksums`: Mapping of schema checksums to database IDs

### Tracked Function Configuration

```go
type TrackedFunctionConfig struct {
    FunctionDeclarations []speech.FunctionDefinition
    ParsingGuide         string
    Timestamp            time.Time // When this config was applied
}
```

**Purpose**: Tracks function configuration changes over time with timestamps.

### Session Events

```go
type Event struct {
    Type string
    Data any
}
```

**Purpose**: Represents events that can progress a session through its lifecycle.

### Session Snapshot

```go
type SessionSnapshot struct {
    ID           DBSessionID
    WSSessionID  WSSessionID
    Status       SessionStatus
    Principal    auth.Principal
    CreatedAt    time.Time
    LastActivity time.Time
}
```

**Purpose**: Immutable snapshot of session state for auditing and debugging.

### Database Record

```go
type Record struct {
    ID          DBSessionID
    WSSessionID WSSessionID
    AccountID   pgtype.UUID
    AppID       pgtype.UUID
    CreatedAt   pgtype.Timestamp
    ClosedAt    *pgtype.Timestamp
    Status      SessionStatus
}
```

**Purpose**: Database representation of a session record.

### Partial Updates

```go
type PartialUpdate struct {
    Status          *SessionStatus
    LastActivity    *time.Time
    FunctionConfig  *TrackedFunctionConfig
    SchemaChecksums *map[string]pgtype.UUID
}
```

**Purpose**: Allows selective updates to session state without full replacement.

## Interfaces

### Manager

```go
type Manager interface {
    // Session lifecycle within connection
    StartSession(ctx context.Context, wsSessionID WSSessionID, principal auth.Principal) (*SessionState, error)
    GetSession(sessionID DBSessionID) (*SessionState, bool)

    // Session State Management
    UpdateSessionStatus(ctx context.Context, sessionID DBSessionID, status SessionStatus) error
    CloseSession(ctx context.Context, sessionID DBSessionID) error

    // Session events
    UpdateSessionMetadata(ctx context.Context, sessionID DBSessionID, metadata PartialUpdate) error
    Snapshot(ctx context.Context, id DBSessionID) (SessionSnapshot, error)
}
```

**Purpose**: Manages session lifecycle and state transitions.

**Methods:**

#### Session Lifecycle

-   `StartSession`: Creates a new session within a WebSocket connection

    -   `wsSessionID`: WebSocket connection identifier
    -   `principal`: Authentication principal
    -   Returns: New session state or error

-   `GetSession`: Retrieves session state by ID
    -   `sessionID`: Database session identifier
    -   Returns: Session state and existence flag

#### State Management

-   `UpdateSessionStatus`: Transitions session to new status

    -   `sessionID`: Database session identifier
    -   `status`: New session status
    -   Returns: Error if transition fails

-   `CloseSession`: Marks session as closed
    -   `sessionID`: Database session identifier
    -   Returns: Error if closure fails

#### Metadata Updates

-   `UpdateSessionMetadata`: Updates session metadata

    -   `sessionID`: Database session identifier
    -   `metadata`: Partial update data
    -   Returns: Error if update fails

-   `Snapshot`: Creates immutable session snapshot
    -   `id`: Database session identifier
    -   Returns: Session snapshot or error

## Session Lifecycle Flow

### 1. Session Creation

```go
// Start a new session
session, err := manager.StartSession(ctx, wsSessionID, principal)
if err != nil {
    return err
}
```

### 2. Status Transitions

```go
// Transition to recording
err = manager.UpdateSessionStatus(ctx, session.ID, SessionRecording)

// Transition to processing
err = manager.UpdateSessionStatus(ctx, session.ID, SessionProcessing)

// Transition to closed
err = manager.CloseSession(ctx, session.ID)
```

### 3. Configuration Updates

```go
// Update session configuration
update := PartialUpdate{
    FunctionConfig: &TrackedFunctionConfig{
        FunctionDeclarations: newDeclarations,
        ParsingGuide:         newGuide,
        Timestamp:            time.Now(),
    },
}
err = manager.UpdateSessionMetadata(ctx, session.ID, update)
```

### 4. Session Monitoring

```go
// Get current session state
session, exists := manager.GetSession(sessionID)
if !exists {
    return errors.New("session not found")
}

// Create snapshot for auditing
snapshot, err := manager.Snapshot(ctx, sessionID)
```

## Configuration Management

### Dynamic Configuration Updates

Sessions support dynamic configuration changes during processing:

1. **Configuration History**: All configuration changes are tracked with timestamps
2. **Current Configuration**: Active configuration is always available
3. **Schema Linking**: Configurations are linked to database schemas via checksums
4. **Rollback Support**: Previous configurations can be restored

### Configuration Tracking Flow

```go
// Track configuration change
config := TrackedFunctionConfig{
    FunctionDeclarations: declarations,
    ParsingGuide:         guide,
    Timestamp:            time.Now(),
}

// Update session with new configuration
update := PartialUpdate{
    FunctionConfig: &config,
}
err = manager.UpdateSessionMetadata(ctx, sessionID, update)
```

## Error Handling

### Common Error Scenarios

-   **Session Not Found**: Attempting to access non-existent session
-   **Invalid Status Transition**: Attempting invalid state changes
-   **Configuration Conflicts**: Invalid configuration updates
-   **Database Errors**: Persistence failures

### Error Recovery

-   **Graceful Degradation**: System continues operating with reduced functionality
-   **State Recovery**: Attempt to restore session state from database
-   **Logging**: Comprehensive error logging for debugging

## Testing

### Unit Testing

```go
func TestSessionLifecycle(t *testing.T) {
    manager := NewMockManager()

    // Test session creation
    session, err := manager.StartSession(ctx, wsID, principal)
    assert.NoError(t, err)
    assert.Equal(t, SessionIdle, session.Status)

    // Test status transitions
    err = manager.UpdateSessionStatus(ctx, session.ID, SessionRecording)
    assert.NoError(t, err)

    // Test session closure
    err = manager.CloseSession(ctx, session.ID)
    assert.NoError(t, err)
}
```

### Integration Testing

-   **Database Integration**: Test with real database connections
-   **Concurrency Testing**: Test multiple sessions and updates
-   **Configuration Testing**: Test dynamic configuration changes

## Implementation Notes

### Thread Safety

-   **Concurrent Access**: Session manager must be thread-safe
-   **State Consistency**: Ensure atomic state transitions
-   **Memory Management**: Proper cleanup of closed sessions

### Performance Considerations

-   **In-Memory State**: Keep active sessions in memory for fast access
-   **Database Persistence**: Persist state changes to database
-   **Cleanup**: Regular cleanup of expired sessions

### Monitoring and Observability

-   **Metrics**: Track session counts, durations, and status transitions
-   **Logging**: Comprehensive logging for debugging and auditing
-   **Tracing**: Distributed tracing for session flows

## Future Enhancements

### Planned Features

-   **Session Recovery**: Automatic recovery from failures
-   **Configuration Validation**: Validate configuration changes
-   **Session Limits**: Enforce session limits per account
-   **Advanced Analytics**: Detailed session analytics and reporting

### Extension Points

-   **Custom Status Types**: Support for custom session statuses
-   **Event Sourcing**: Event-sourced session state management
-   **Distributed Sessions**: Support for distributed session management
