package connection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"schma.ai/internal/domain/connection"
)

// TestInfrastructureCreation tests that all infrastructure components can be created
func TestInfrastructureCreation(t *testing.T) {
	// Test that we can create a repository (even without real database)
	// This will fail at runtime if there are compilation issues, but compiles fine
	
	// Test that we can create a pool
	pool := NewPool(100, 30*time.Second, nil) // nil repo for testing
	assert.NotNil(t, pool)
	
	// Test that we can create a manager
	manager := NewManager(pool, nil) // nil repo for testing
	assert.NotNil(t, manager)
	
	// Test pool stats
	stats := pool.GetPoolStats()
	assert.Equal(t, 100, stats.MaxConnections)
	assert.Equal(t, 0, stats.ActiveConnections)
}

// TestPoolOperations tests basic pool operations without database calls
func TestPoolOperations(t *testing.T) {
	// Create pool with nil repository (no database calls)
	pool := NewPool(2, 30*time.Second, nil)
	
	// Test pool is not full initially
	stats := pool.GetPoolStats()
	assert.Equal(t, 0, stats.ActiveConnections)
	assert.Equal(t, 2, stats.MaxConnections)
	
	// Test that we can start and stop the pool
	ctx := context.Background()
	err := pool.Start(ctx)
	assert.NoError(t, err)
	
	err = pool.Stop(ctx)
	assert.NoError(t, err)
}

// TestManagerOperations tests basic manager operations without database calls
func TestManagerOperations(t *testing.T) {
	// Create manager with nil repository (no database calls)
	pool := NewPool(10, 30*time.Second, nil)
	manager := NewManager(pool, nil)
	
	// Test that manager can access pool stats
	poolStats := pool.GetPoolStats()
	assert.Equal(t, 0, poolStats.ActiveConnections)
	
	// Test that manager can list active connections (empty)
	// Note: This would require a valid UUID in real usage
	// For testing, we'll just verify the method exists
	_ = manager.ListActiveConnections
}

// TestConnectionTypes tests that domain types are properly accessible
func TestConnectionTypes(t *testing.T) {
	// Test that we can access domain types
	assert.NotEmpty(t, connection.ConnectionStatusConnecting)
	assert.NotEmpty(t, connection.ConnectionStatusActive)
	assert.NotEmpty(t, connection.ConnectionStatusIdle)
	assert.NotEmpty(t, connection.ConnectionStatusClosing)
	assert.NotEmpty(t, connection.ConnectionStatusClosed)
	
	assert.NotEmpty(t, connection.LLMModeFunctions)
	assert.NotEmpty(t, connection.LLMModeStructured)
	assert.NotEmpty(t, connection.LLMModeNone)
	
	assert.NotEmpty(t, connection.EventTypeConnect)
	assert.NotEmpty(t, connection.EventTypeDisconnect)
	assert.NotEmpty(t, connection.EventTypeError)
	assert.NotEmpty(t, connection.EventTypeTimeout)
	assert.NotEmpty(t, connection.EventTypeInfo)
}
