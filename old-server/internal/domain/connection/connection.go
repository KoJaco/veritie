package connection

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/auth"
)

// ConnectionID represents a unique identifier for a WebSocket connection
type ConnectionID string

// WSSessionID represents the WebSocket session identifier (from JWT)
type WSSessionID string

// ConnectionStatus represents the current state of a connection
type ConnectionStatus string

const (
	ConnectionStatusConnecting ConnectionStatus = "connecting"
	ConnectionStatusActive     ConnectionStatus = "active"
	ConnectionStatusIdle       ConnectionStatus = "idle"
	ConnectionStatusClosing    ConnectionStatus = "closing"
	ConnectionStatusClosed     ConnectionStatus = "closed"
)

// LLMMode represents the LLM processing mode for this connection
type LLMMode string

// TODO: update this when we add new LLM modes
const (
	LLMModeFunctions  LLMMode = "functions"
	LLMModeStructured LLMMode = "structured"
	LLMModeNone       LLMMode = "none"
)

// EventType represents connection lifecycle events
type EventType string

const (
	EventTypeConnect    EventType = "connect"
	EventTypeDisconnect EventType = "disconnect"
	EventTypeError      EventType = "error"
	EventTypeTimeout    EventType = "timeout"
	EventTypeInfo       EventType = "info"
)

// Connection represents a WebSocket connection with its lifecycle state
type Connection struct {
	ID           ConnectionID
	WSSessionID  WSSessionID
	Principal    auth.Principal
	Status       ConnectionStatus
	LLMMode      LLMMode
	
	// Connection metadata
	RemoteAddr   string
	UserAgent    string
	Subprotocols []string
	
	// Timing
	CreatedAt    time.Time
	LastActivity time.Time
	ClosedAt     *time.Time
	
	// Active session tracking
	ActiveSessionID *pgtype.UUID
	
	// Configuration state
	STTProvider              string
	FunctionDefinitionsCount int
	StructuredSchemaPresent  bool
	
	// Performance metrics
	PingLatencyMS           int
	MessagesSent            int
	MessagesReceived        int
	AudioChunksProcessed    int
	
	// Error tracking
	LastError               string
	ErrorCode               string
	ErrorCount              int
}

// ConnectionEvent represents a connection lifecycle event
type ConnectionEvent struct {
	ConnectionID ConnectionID
	WSSessionID  WSSessionID
	AppID        pgtype.UUID
	AccountID    pgtype.UUID
	EventType    EventType
	EventData    map[string]interface{}
	Timestamp    time.Time
	
	// Optional error information
	ErrorMessage *string
	ErrorCode    *string
	
	// Optional completion data
	DurationMS   *int
	FinalMetrics *ConnectionMetrics
}

// ConnectionMetrics represents final connection statistics
type ConnectionMetrics struct {
	MessagesSent         int
	MessagesReceived     int
	AudioChunksProcessed int
	TotalDurationMS      int
}

// ConnectionState represents a snapshot of connection state for persistence
type ConnectionState struct {
	ConnectionID            ConnectionID
	WSSessionID            WSSessionID
	AppID                  pgtype.UUID
	AccountID              pgtype.UUID
	LLMMode                LLMMode
	ActiveSessionID        *pgtype.UUID
	ConnectionStatus       ConnectionStatus
	STTProvider            string
	FunctionDefinitionsCount int
	StructuredSchemaPresent  bool
	LastActivity           time.Time
	PingLatencyMS          int
	LastError              string
	ErrorCount             int
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// ConnectionConfig represents configuration for a new connection
type ConnectionConfig struct {
	WSSessionID  WSSessionID
	Principal    auth.Principal
	RemoteAddr   string
	UserAgent    string
	Subprotocols []string
	LLMMode      LLMMode
}

// NewConnection creates a new connection instance
func NewConnection(config ConnectionConfig) *Connection {
	now := time.Now()
	return &Connection{
		ID:           generateConnectionID(),
		WSSessionID:  config.WSSessionID,
		Principal:    config.Principal,
		Status:       ConnectionStatusConnecting,
		LLMMode:      config.LLMMode,
		RemoteAddr:   config.RemoteAddr,
		UserAgent:    config.UserAgent,
		Subprotocols: config.Subprotocols,
		CreatedAt:    now,
		LastActivity: now,
	}
}

// generateConnectionID creates a unique connection identifier
func generateConnectionID() ConnectionID {
	return ConnectionID(fmt.Sprintf("conn_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8]))
}

// Domain methods for connection state management

// Activate marks the connection as active and updates activity timestamp
func (c *Connection) Activate() {
	c.Status = ConnectionStatusActive
	c.updateActivity()
}

// SetIdle marks the connection as idle
func (c *Connection) SetIdle() {
	c.Status = ConnectionStatusIdle
	c.updateActivity()
}

// SetClosing marks the connection as closing
func (c *Connection) SetClosing() {
	c.Status = ConnectionStatusClosing
	c.updateActivity()
}

// Close marks the connection as closed and sets closed timestamp
func (c *Connection) Close() {
	c.Status = ConnectionStatusClosed
	now := time.Now()
	c.ClosedAt = &now
	c.updateActivity()
}

// updateActivity updates the last activity timestamp
func (c *Connection) updateActivity() {
	c.LastActivity = time.Now()
}

// SetActiveSession sets the active session ID
func (c *Connection) SetActiveSession(sessionID pgtype.UUID) {
	c.ActiveSessionID = &sessionID
	c.updateActivity()
}

// ClearActiveSession clears the active session ID
func (c *Connection) ClearActiveSession() {
	c.ActiveSessionID = nil
	c.updateActivity()
}

// UpdateLLMMode updates the LLM mode
func (c *Connection) UpdateLLMMode(mode LLMMode) {
	c.LLMMode = mode
	c.updateActivity()
}

// RecordMessageSent increments the sent message counter
func (c *Connection) RecordMessageSent() {
	c.MessagesSent++
	c.updateActivity()
}

// RecordMessageReceived increments the received message counter
func (c *Connection) RecordMessageReceived() {
	c.MessagesReceived++
	c.updateActivity()
}

// RecordAudioChunk increments the audio chunk counter
func (c *Connection) RecordAudioChunk() {
	c.AudioChunksProcessed++
	c.updateActivity()
}

// RecordError records an error and increments the error counter
func (c *Connection) RecordError(err error, code string) {
	c.LastError = err.Error()
	c.ErrorCode = code
	c.ErrorCount++
	c.updateActivity()
}

// UpdatePingLatency updates the ping latency measurement
func (c *Connection) UpdatePingLatency(latencyMS int) {
	c.PingLatencyMS = latencyMS
	c.updateActivity()
}

// GetDuration returns the total connection duration
func (c *Connection) GetDuration() time.Duration {
	if c.ClosedAt != nil {
		return c.ClosedAt.Sub(c.CreatedAt)
	}
	return time.Since(c.CreatedAt)
}

// IsActive returns true if the connection is currently active
func (c *Connection) IsActive() bool {
	return c.Status == ConnectionStatusActive || c.Status == ConnectionStatusIdle
}

// IsClosed returns true if the connection is closed
func (c *Connection) IsClosed() bool {
	return c.Status == ConnectionStatusClosed
}

// ToConnectionState converts the connection to a state snapshot
func (c *Connection) ToConnectionState() ConnectionState {
	return ConnectionState{
		ConnectionID:            c.ID,
		WSSessionID:            c.WSSessionID,
		AppID:                  c.Principal.AppID,
		AccountID:              c.Principal.AccountID,
		LLMMode:                c.LLMMode,
		ActiveSessionID:        c.ActiveSessionID,
		ConnectionStatus:       c.Status,
		STTProvider:            c.STTProvider,
		FunctionDefinitionsCount: c.FunctionDefinitionsCount,
		StructuredSchemaPresent:  c.StructuredSchemaPresent,
		LastActivity:           c.LastActivity,
		PingLatencyMS:          c.PingLatencyMS,
		LastError:              c.LastError,
		ErrorCount:             c.ErrorCount,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              time.Now(),
	}
}

// ToConnectionEvent converts the connection to an event
func (c *Connection) ToConnectionEvent(eventType EventType, data map[string]interface{}) ConnectionEvent {
	return ConnectionEvent{
		ConnectionID: c.ID,
		WSSessionID:  c.WSSessionID,
		AppID:        c.Principal.AppID,
		AccountID:    c.Principal.AccountID,
		EventType:    eventType,
		EventData:    data,
		Timestamp:    time.Now(),
	}
}
