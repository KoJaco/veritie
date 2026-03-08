package connection

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"schma.ai/internal/domain/auth"
	"schma.ai/internal/domain/connection"
)

// MockRepository implements connection.Repository for testing
type MockRepository struct {
	// This would be implemented with actual mock methods
	// For now, we'll just use the mock repository from the infra package
}

func (m *MockRepository) CreateConnectionLog(ctx context.Context, event connection.ConnectionEvent) error {
	return nil
}

func (m *MockRepository) EndConnectionLog(ctx context.Context, connectionID connection.ConnectionID, endedAt time.Time, durationMS int, errorMessage *string, errorCode *string, finalMetrics *connection.ConnectionMetrics) error {
	return nil
}

func (m *MockRepository) AppendConnectionEvent(ctx context.Context, event connection.ConnectionEvent) error {
	return nil
}

func (m *MockRepository) UpsertConnectionState(ctx context.Context, state connection.ConnectionState) error {
	return nil
}

func (m *MockRepository) UpdateConnectionStateOnClose(ctx context.Context, connectionID connection.ConnectionID) error {
	return nil
}

func (m *MockRepository) GetConnectionState(ctx context.Context, connectionID connection.ConnectionID) (*connection.ConnectionState, error) {
	return nil, nil
}

func (m *MockRepository) ListConnectionLogsByApp(ctx context.Context, appID pgtype.UUID, since time.Time, limit, offset int) ([]connection.ConnectionEvent, error) {
	return nil, nil
}

func (m *MockRepository) ListActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]connection.ConnectionEvent, error) {
	return nil, nil
}

func (m *MockRepository) CountActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID) (int, error) {
	return 0, nil
}

func (m *MockRepository) ListActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]connection.ConnectionState, error) {
	return nil, nil
}

func (m *MockRepository) CountActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID) (int, error) {
	return 0, nil
}

func (m *MockRepository) CleanupStaleConnections(ctx context.Context, olderThan time.Time) (int, error) {
	return 0, nil
}

func (m *MockRepository) ArchiveConnectionLogs(ctx context.Context, olderThan time.Time) (int, error) {
	return 0, nil
}

// TestConnectionServiceLifecycle tests the complete lifecycle of the connection service
func TestConnectionServiceLifecycle(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Create connection service
	service := NewService(mockRepo, 10, 30*time.Second)
	
	// Test service creation
	assert.NotNil(t, service)
	assert.Equal(t, 10, service.maxConnections)
	assert.Equal(t, 30*time.Second, service.cleanupInterval)
	
	// Test service start
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	assert.True(t, service.started)
	
	// Test service stop
	err = service.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, service.started)
}

// TestConnectionServiceOperations tests basic connection operations
func TestConnectionServiceOperations(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Create connection service
	service := NewService(mockRepo, 5, 30*time.Second)
	
	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop(ctx)
	
	// Test connection creation
	principal := auth.Principal{
		AppID:     pgtype.UUID{},
		AccountID: pgtype.UUID{},
	}
	
	conn, err := service.CreateConnection(ctx, "test-session-123", principal, "127.0.0.1:8080", "test-agent", []string{"schma.ws.v1"}, connection.LLMModeFunctions)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, connection.ConnectionStatusConnecting, conn.Status)
	
	// Test connection retrieval
	retrievedConn, exists := service.GetConnection(ctx, conn.ID)
	assert.True(t, exists)
	assert.Equal(t, conn.ID, retrievedConn.ID)
	
	// Test connection by WSSessionID
	wsConn, exists := service.GetConnectionByWSSession("test-session-123")
	assert.True(t, exists)
	assert.Equal(t, conn.ID, wsConn.ID)
	
	// Test connection count
	count := service.GetConnectionCount()
	assert.Equal(t, 1, count)
	
	// Test pool stats
	stats := service.GetPoolStats()
	assert.Equal(t, 1, stats.ActiveConnections)
	assert.Equal(t, 5, stats.MaxConnections)
	
	// Test connection closure
	err = service.CloseConnection(ctx, conn.ID, "test completion")
	require.NoError(t, err)
	
	// Verify connection is removed
	_, exists = service.GetConnection(ctx, conn.ID)
	assert.False(t, exists)
	
	// Test connection count after closure
	count = service.GetConnectionCount()
	assert.Equal(t, 0, count)
}

// TestConnectionServiceCapacity tests the connection capacity limits
func TestConnectionServiceCapacity(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Create connection service with small capacity
	service := NewService(mockRepo, 2, 30*time.Second)
	
	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop(ctx)
	
	// Test pool capacity
	assert.False(t, service.IsPoolFull())
	
	principal := auth.Principal{
		AppID:     pgtype.UUID{},
		AccountID: pgtype.UUID{},
	}
	
	// Create first connection
	conn1, err := service.CreateConnection(ctx, "session-1", principal, "127.0.0.1:8080", "test-agent", []string{"schma.ws.v1"}, connection.LLMModeFunctions)
	require.NoError(t, err)
	assert.NotNil(t, conn1)
	
	// Create second connection
	conn2, err := service.CreateConnection(ctx, "session-2", principal, "127.0.0.1:8081", "test-agent", []string{"schma.ws.v1"}, connection.LLMModeFunctions)
	require.NoError(t, err)
	assert.NotNil(t, conn2)
	
	// Test pool is at capacity
	assert.True(t, service.IsPoolFull())
	
	// Try to create third connection (should fail due to capacity)
	_, err = service.CreateConnection(ctx, "session-3", principal, "127.0.0.1:8082", "test-agent", []string{"schma.ws.v1"}, connection.LLMModeFunctions)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection pool is full")
	
	// Clean up
	service.CloseConnection(ctx, conn1.ID, "test cleanup")
	service.CloseConnection(ctx, conn2.ID, "test cleanup")
}

// TestConnectionServiceHealth tests connection health monitoring
func TestConnectionServiceHealth(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Create connection service
	service := NewService(mockRepo, 10, 30*time.Second)
	
	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop(ctx)
	
	// Create a connection
	principal := auth.Principal{
		AppID:     pgtype.UUID{},
		AccountID: pgtype.UUID{},
	}
	
	conn, err := service.CreateConnection(ctx, "test-session", principal, "127.0.0.1:8080", "test-agent", []string{"schma.ws.v1"}, connection.LLMModeFunctions)
	require.NoError(t, err)
	
	// Activate the connection first
	conn.Activate()
	
	// Test connection health
	healthy, err := service.IsConnectionHealthy(ctx, conn.ID)
	require.NoError(t, err)
	assert.True(t, healthy)
	
	// Test ping connection
	err = service.PingConnection(ctx, conn.ID)
	require.NoError(t, err)
	
	// Clean up
	service.CloseConnection(ctx, conn.ID, "test cleanup")
}
