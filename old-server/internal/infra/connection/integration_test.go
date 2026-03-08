package connection

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"schma.ai/internal/domain/auth"
	"schma.ai/internal/domain/connection"
)

// MockQueries implements the minimal interface needed for testing
type MockQueries struct {
	// This would be implemented with actual mock methods
	// For now, we'll just use the mock repository
}

// TestConnectionInfrastructureIntegration demonstrates how the components work together
func TestConnectionInfrastructureIntegration(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Create connection pool
	pool := NewPool(100, 30*time.Second, mockRepo)
	
	// Create connection manager
	manager := NewManager(pool, mockRepo)
	
	// Test that all components are properly initialized
	assert.NotNil(t, pool)
	assert.NotNil(t, manager)
	
	// Test pool stats
	stats := pool.GetPoolStats()
	assert.Equal(t, 100, stats.MaxConnections)
	assert.Equal(t, 0, stats.ActiveConnections)
	
	// Test that manager can access pool stats through the pool
	poolStats := pool.GetPoolStats()
	assert.Equal(t, stats, poolStats)
}

// TestConnectionLifecycle demonstrates a complete connection lifecycle
func TestConnectionLifecycle(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Set up mock expectations for the connection lifecycle
	mockRepo.On("CreateConnectionLog", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpsertConnectionState", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpdateConnectionStateOnClose", mock.Anything, mock.Anything).Return(nil)
	// Note: EndConnectionLog is not called, CreateConnectionLog is used instead
	
	// Create connection pool with small capacity for testing
	pool := NewPool(5, 30*time.Second, mockRepo)
	
	// Create connection manager
	manager := NewManager(pool, mockRepo)
	
	// Create a test connection config
	config := connection.ConnectionConfig{
		WSSessionID:  "test-session-123",
		Principal:    auth.Principal{AppID: pgtype.UUID{}, AccountID: pgtype.UUID{}},
		RemoteAddr:   "127.0.0.1:8080",
		UserAgent:    "test-agent",
		Subprotocols: []string{"schma.ws.v1"},
		LLMMode:      connection.LLMModeFunctions,
	}
	
	// Test connection creation
	conn, err := manager.CreateConnection(context.Background(), config)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, connection.ConnectionStatusConnecting, conn.Status)
	
	// Test that connection is in pool
	poolConn, exists := pool.GetConnection(conn.ID)
	assert.True(t, exists)
	assert.Equal(t, conn.ID, poolConn.ID)
	
	// Test connection activation
	conn.Activate()
	assert.Equal(t, connection.ConnectionStatusActive, conn.Status)
	
	// Test connection stats update
	conn.RecordMessageSent()
	conn.RecordMessageReceived()
	assert.Equal(t, 1, conn.MessagesSent)
	assert.Equal(t, 1, conn.MessagesReceived)
	
	// Test connection closure
	err = manager.CloseConnection(context.Background(), conn.ID, "test completion")
	require.NoError(t, err)
	
	// Test that connection is removed from pool
	_, exists = pool.GetConnection(conn.ID)
	assert.False(t, exists)
	
	// Test pool stats after connection lifecycle
	stats := pool.GetPoolStats()
	assert.Equal(t, 1, stats.TotalConnections)
	assert.Equal(t, 0, stats.ActiveConnections)
	assert.Equal(t, 1, stats.TotalDisconnections)
	
	// Verify all mock expectations were met
	mockRepo.AssertExpectations(t)
}

// TestConnectionPoolCapacity tests the pool's capacity limits
func TestConnectionPoolCapacity(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Set up mock expectations for connection creation and cleanup
	// Use AnyTimes() to be more flexible about the number of calls
	mockRepo.On("CreateConnectionLog", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpsertConnectionState", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpdateConnectionStateOnClose", mock.Anything, mock.Anything).Return(nil)
	// Note: EndConnectionLog is not called, CreateConnectionLog is used instead
	
	// Create connection pool with capacity of 2
	pool := NewPool(2, 30*time.Second, mockRepo)
	
	// Create connection manager
	manager := NewManager(pool, mockRepo)
	
	// Create first connection
	config1 := connection.ConnectionConfig{
		WSSessionID: "session-1",
		Principal:   auth.Principal{AppID: pgtype.UUID{}, AccountID: pgtype.UUID{}},
		RemoteAddr:  "127.0.0.1:8080",
		UserAgent:   "test-agent",
		LLMMode:     connection.LLMModeFunctions,
	}
	
	conn1, err := manager.CreateConnection(context.Background(), config1)
	require.NoError(t, err)
	assert.NotNil(t, conn1)
	
	// Create second connection
	config2 := connection.ConnectionConfig{
		WSSessionID: "session-2",
		Principal:   auth.Principal{AppID: pgtype.UUID{}, AccountID: pgtype.UUID{}},
		RemoteAddr:  "127.0.0.1:8081",
		UserAgent:   "test-agent",
		LLMMode:     connection.LLMModeFunctions,
	}
	
	conn2, err := manager.CreateConnection(context.Background(), config2)
	require.NoError(t, err)
	assert.NotNil(t, conn2)
	
	// Test pool is at capacity
	stats := pool.GetPoolStats()
	assert.Equal(t, 2, stats.ActiveConnections)
	
	// Try to create third connection (should fail due to capacity)
	config3 := connection.ConnectionConfig{
		WSSessionID: "session-3",
		Principal:   auth.Principal{AppID: pgtype.UUID{}, AccountID: pgtype.UUID{}},
		RemoteAddr:  "127.0.0.1:8082",
		UserAgent:   "test-agent",
		LLMMode:     connection.LLMModeFunctions,
	}
	
	_, err = manager.CreateConnection(context.Background(), config3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add connection to pool: connection pool is full")
	
	// Clean up
	manager.CloseConnection(context.Background(), conn1.ID, "test cleanup")
	manager.CloseConnection(context.Background(), conn2.ID, "test cleanup")
	
	// Verify all mock expectations were met
	mockRepo.AssertExpectations(t)
}

// TestConnectionManagerOperations tests various manager operations
func TestConnectionManagerOperations(t *testing.T) {
	// Create mock repository
	mockRepo := &MockRepository{}
	
	// Set up mock expectations
	mockRepo.On("CreateConnectionLog", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpsertConnectionState", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpdateConnectionStateOnClose", mock.Anything, mock.Anything).Return(nil)
	// Note: EndConnectionLog is not called, CreateConnectionLog is used instead
	
	// Create connection pool
	pool := NewPool(10, 30*time.Second, mockRepo)
	
	// Create connection manager
	manager := NewManager(pool, mockRepo)
	
	// Create a test connection
	config := connection.ConnectionConfig{
		WSSessionID: "test-session",
		Principal:   auth.Principal{AppID: pgtype.UUID{}, AccountID: pgtype.UUID{}},
		RemoteAddr:  "127.0.0.1:8080",
		UserAgent:   "test-agent",
		LLMMode:     connection.LLMModeFunctions,
	}
	
	conn, err := manager.CreateConnection(context.Background(), config)
	require.NoError(t, err)
	
	// Activate the connection first for health check
	conn.Activate()
	
	// Test connection health check
	healthy, err := manager.IsConnectionHealthy(context.Background(), conn.ID)
	require.NoError(t, err)
	assert.True(t, healthy)
	
	// Test ping connection
	err = manager.PingConnection(context.Background(), conn.ID)
	require.NoError(t, err)
	
	// Clean up
	manager.CloseConnection(context.Background(), conn.ID, "test cleanup")
	
	// Verify all mock expectations were met
	mockRepo.AssertExpectations(t)
}
