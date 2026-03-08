package connection

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Repository defines the interface for connection data persistence
type Repository interface {
	// Connection lifecycle events
	CreateConnectionLog(ctx context.Context, event ConnectionEvent) error
	EndConnectionLog(ctx context.Context, connectionID ConnectionID, endedAt time.Time, durationMS int, errorMessage *string, errorCode *string, finalMetrics *ConnectionMetrics) error
	AppendConnectionEvent(ctx context.Context, event ConnectionEvent) error
	
	// Connection state management
	UpsertConnectionState(ctx context.Context, state ConnectionState) error
	UpdateConnectionStateOnClose(ctx context.Context, connectionID ConnectionID) error
	GetConnectionState(ctx context.Context, connectionID ConnectionID) (*ConnectionState, error)
	
	// Query operations
	ListConnectionLogsByApp(ctx context.Context, appID pgtype.UUID, since time.Time, limit, offset int) ([]ConnectionEvent, error)
	ListActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]ConnectionEvent, error)
	CountActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID) (int, error)
	ListActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]ConnectionState, error)
	CountActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID) (int, error)
	
	// Cleanup operations
	CleanupStaleConnections(ctx context.Context, olderThan time.Time) (int, error)
	ArchiveConnectionLogs(ctx context.Context, olderThan time.Time) (int, error)
}

// Manager defines the interface for connection lifecycle management
type Manager interface {
	// Connection lifecycle
	CreateConnection(ctx context.Context, config ConnectionConfig) (*Connection, error)
	GetConnection(ctx context.Context, connectionID ConnectionID) (*Connection, bool)
	CloseConnection(ctx context.Context, connectionID ConnectionID, reason string) error
	
	// Connection monitoring
	ListActiveConnections(ctx context.Context, appID pgtype.UUID) ([]*Connection, error)
	GetConnectionStats(ctx context.Context, appID pgtype.UUID) (*ConnectionStats, error)
	
	// Health checks
	IsConnectionHealthy(ctx context.Context, connectionID ConnectionID) (bool, error)
	PingConnection(ctx context.Context, connectionID ConnectionID) error
}

// ConnectionStats represents aggregated connection statistics
type ConnectionStats struct {
	AppID                    pgtype.UUID
	TotalConnections         int
	ActiveConnections        int
	IdleConnections          int
	PeakConnections          int
	TotalErrors              int
	AverageConnectionDuration time.Duration
	LastUpdated              time.Time
}

// Pool defines the interface for connection pool management
type Pool interface {
	// Pool management
	AddConnection(connection *Connection) error
	RemoveConnection(connectionID ConnectionID) error
	GetConnection(connectionID ConnectionID) (*Connection, bool)
	
	// Pool monitoring
	GetPoolStats() PoolStats
	GetActiveConnections() []*Connection
	
	// Pool lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// PoolStats represents connection pool statistics
type PoolStats struct {
	TotalConnections    int
	ActiveConnections   int
	PeakConnections     int
	TotalDisconnections int
	LastCleanup         time.Time
	MaxConnections      int
}

// EventHandler defines the interface for handling connection events
type EventHandler interface {
	// Event handling
	OnConnectionCreated(ctx context.Context, connection *Connection) error
	OnConnectionActivated(ctx context.Context, connection *Connection) error
	OnConnectionIdle(ctx context.Context, connection *Connection) error
	OnConnectionClosing(ctx context.Context, connection *Connection) error
	OnConnectionClosed(ctx context.Context, connection *Connection) error
	OnConnectionError(ctx context.Context, connection *Connection, err error) error
}

// MetricsCollector defines the interface for collecting connection metrics
type MetricsCollector interface {
	// Metrics collection
	RecordConnectionEvent(ctx context.Context, event ConnectionEvent) error
	RecordConnectionMetrics(ctx context.Context, connectionID ConnectionID, metrics ConnectionMetrics) error
	RecordConnectionError(ctx context.Context, connectionID ConnectionID, err error, code string) error
	
	// Metrics aggregation
	GetConnectionMetrics(ctx context.Context, appID pgtype.UUID, since time.Time) (*ConnectionMetrics, error)
	GetAppConnectionStats(ctx context.Context, appID pgtype.UUID) (*ConnectionStats, error)
}
