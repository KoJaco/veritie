package connection

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/connection"
	"schma.ai/internal/pkg/logger"
)

// manager implements connection.Manager
type manager struct {
	pool     connection.Pool
	repo     connection.Repository
	mu       sync.RWMutex
	stats    map[string]*connection.ConnectionStats
}

// NewManager creates a new connection manager
func NewManager(pool connection.Pool, repo connection.Repository) connection.Manager {
	return &manager{
		pool:  pool,
		repo:  repo,
		stats: make(map[string]*connection.ConnectionStats),
	}
}

// CreateConnection creates a new connection and adds it to the pool
func (m *manager) CreateConnection(ctx context.Context, config connection.ConnectionConfig) (*connection.Connection, error) {
	// Create new connection instance
	conn := connection.NewConnection(config)
	
	// Add to pool
	if err := m.pool.AddConnection(conn); err != nil {
		return nil, fmt.Errorf("failed to add connection to pool: %w", err)
	}
	
	// Log connection creation
	if m.repo != nil {
		event := conn.ToConnectionEvent(connection.EventTypeConnect, map[string]interface{}{
			"remote_addr":   config.RemoteAddr,
			"user_agent":    config.UserAgent,
			"subprotocols":  config.Subprotocols,
			"llm_mode":      config.LLMMode,
		})
		
		if err := m.repo.CreateConnectionLog(ctx, event); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to log connection creation: %v", err)
		}
		
		// Create initial connection state
		state := conn.ToConnectionState()
		if err := m.repo.UpsertConnectionState(ctx, state); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to create initial connection state: %v", err)
		}
	}
	
	// Update connection stats
	m.updateConnectionStats(conn.Principal.AppID.String(), conn, true)
	
	logger.Infof("🔌 [MANAGER] Created connection %s for app %s", 
		conn.ID, conn.Principal.AppID.String())
	
	return conn, nil
}

// GetConnection retrieves a connection by ID
func (m *manager) GetConnection(ctx context.Context, connectionID connection.ConnectionID) (*connection.Connection, bool) {
	return m.pool.GetConnection(connectionID)
}

// CloseConnection closes a connection and removes it from the pool
func (m *manager) CloseConnection(ctx context.Context, connectionID connection.ConnectionID, reason string) error {
	// Get connection from pool
	conn, exists := m.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	// Log connection close event
	if m.repo != nil {
		event := conn.ToConnectionEvent(connection.EventTypeDisconnect, map[string]interface{}{
			"reason":        reason,
			"duration_ms":   int(conn.GetDuration().Milliseconds()),
			"final_status":  conn.Status,
		})
		
		if err := m.repo.CreateConnectionLog(ctx, event); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to log connection close: %v", err)
		}
	}
	
	// Mark connection as closing
	conn.SetClosing()
	
	// Update final state
	if m.repo != nil {
		state := conn.ToConnectionState()
		if err := m.repo.UpsertConnectionState(ctx, state); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to update final connection state: %v", err)
		}
	}
	
	// Mark as closed
	conn.Close()
	
	// Remove from pool
	m.pool.RemoveConnection(connectionID)
	
	// Update connection stats
	m.updateConnectionStats(conn.Principal.AppID.String(), conn, false)
	
	logger.Infof("🔌 [MANAGER] Closed connection %s: %s", connectionID, reason)
	
	return nil
}

// ListActiveConnections returns all active connections for a specific app
func (m *manager) ListActiveConnections(ctx context.Context, appID pgtype.UUID) ([]*connection.Connection, error) {
	// Get from pool first (in-memory)
	poolConnections := m.pool.GetActiveConnections()
	
	// Filter active connections
	var activeConnections []*connection.Connection
	for _, conn := range poolConnections {
		if conn.IsActive() {
			activeConnections = append(activeConnections, conn)
		}
	}
	
	return activeConnections, nil
}

// GetConnectionStats returns aggregated connection statistics for an app
func (m *manager) GetConnectionStats(ctx context.Context, appID pgtype.UUID) (*connection.ConnectionStats, error) {
	appIDStr := appID.String()
	
	m.mu.RLock()
	stats, exists := m.stats[appIDStr]
	m.mu.RUnlock()
	
	if !exists {
		// Initialize stats if not exists
		stats = &connection.ConnectionStats{
			AppID:            appID,
			LastUpdated:      time.Now(),
		}
	}
	
	// Get current connection count from pool
	poolConnections := m.pool.GetActiveConnections()
	
	// Calculate current stats
	activeCount := 0
	idleCount := 0
	totalErrors := 0
	var totalDuration time.Duration
	connectionCount := 0
	
	for _, conn := range poolConnections {
		if conn.IsActive() {
			connectionCount++
			
			switch conn.Status {
			case connection.ConnectionStatusActive:
				activeCount++
			case connection.ConnectionStatusIdle:
				idleCount++
			}
			
			totalErrors += conn.ErrorCount
			totalDuration += conn.GetDuration()
		}
	}
	
	// Update stats
	stats.ActiveConnections = activeCount
	stats.IdleConnections = idleCount
	stats.TotalErrors = totalErrors
	stats.LastUpdated = time.Now()
	
	if connectionCount > 0 {
		stats.AverageConnectionDuration = totalDuration / time.Duration(connectionCount)
	}
	
	// Update peak connections if current count is higher
	if connectionCount > stats.PeakConnections {
		stats.PeakConnections = connectionCount
	}
	
	// Cache updated stats
	m.mu.Lock()
	m.stats[appIDStr] = stats
	m.mu.Unlock()
	
	return stats, nil
}

// IsConnectionHealthy checks if a connection is healthy
func (m *manager) IsConnectionHealthy(ctx context.Context, connectionID connection.ConnectionID) (bool, error) {
	conn, exists := m.pool.GetConnection(connectionID)
	if !exists {
		return false, connection.ErrConnectionNotFound
	}
	
	// Check if connection is active
	if !conn.IsActive() {
		return false, nil
	}
	
	// Check if connection has been inactive for too long (more than 5 minutes)
	if time.Since(conn.LastActivity) > 5*time.Minute {
		return false, nil
	}
	
	// Check error count (too many errors might indicate issues)
	if conn.ErrorCount > 10 {
		return false, nil
	}
	
	return true, nil
}

// PingConnection sends a ping to a connection to check health
func (m *manager) PingConnection(ctx context.Context, connectionID connection.ConnectionID) error {
	conn, exists := m.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	// Update last activity by calling a method that updates activity
	conn.RecordMessageReceived()
	
	// Update connection state in repository
	if m.repo != nil {
		state := conn.ToConnectionState()
		if err := m.repo.UpsertConnectionState(ctx, state); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to update connection state on ping: %v", err)
		}
	}
	
	return nil
}

// updateConnectionStats updates the connection statistics for an app
func (m *manager) updateConnectionStats(appID string, conn *connection.Connection, isNew bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	stats, exists := m.stats[appID]
	if !exists {
		stats = &connection.ConnectionStats{
			LastUpdated: time.Now(),
		}
		m.stats[appID] = stats
	}
	
	if isNew {
		stats.TotalConnections++
		if stats.TotalConnections > stats.PeakConnections {
			stats.PeakConnections = stats.TotalConnections
		}
	}
	
	stats.LastUpdated = time.Now()
}

// GetPoolStats returns the current pool statistics
func (m *manager) GetPoolStats() connection.PoolStats {
	return m.pool.GetPoolStats()
}

// GetConnectionByWSSessionID retrieves a connection by WebSocket session ID
func (m *manager) GetConnectionByWSSessionID(wsSessionID connection.WSSessionID) (*connection.Connection, bool) {
	// Search through all active connections to find by WSSessionID
	activeConnections := m.pool.GetActiveConnections()
	for _, conn := range activeConnections {
		if conn.WSSessionID == wsSessionID {
			return conn, true
		}
	}
	return nil, false
}

// UpdateConnectionLLMMode updates the LLM mode of a connection
func (m *manager) UpdateConnectionLLMMode(connectionID connection.ConnectionID, mode connection.LLMMode) error {
	conn, exists := m.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	conn.UpdateLLMMode(mode)
	
	// Update connection state in repository
	if m.repo != nil {
		state := conn.ToConnectionState()
		if err := m.repo.UpsertConnectionState(context.Background(), state); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to update connection state: %v", err)
		}
	}
	
	// Log mode change event
	if m.repo != nil {
		event := conn.ToConnectionEvent(connection.EventTypeInfo, map[string]interface{}{
			"llm_mode_change": mode,
			"previous_mode":   conn.LLMMode,
		})
		
		if err := m.repo.CreateConnectionLog(context.Background(), event); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to log LLM mode change: %v", err)
		}
	}
	
	return nil
}

// SetConnectionActiveSession sets the active session for a connection
func (m *manager) SetConnectionActiveSession(connectionID connection.ConnectionID, sessionID pgtype.UUID) error {
	conn, exists := m.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	conn.SetActiveSession(sessionID)
	
	// Update connection state in repository
	if m.repo != nil {
		state := conn.ToConnectionState()
		if err := m.repo.UpsertConnectionState(context.Background(), state); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to update connection state: %v", err)
		}
	}
	
	return nil
}

// ClearConnectionActiveSession clears the active session for a connection
func (m *manager) ClearConnectionActiveSession(connectionID connection.ConnectionID) error {
	conn, exists := m.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	conn.ClearActiveSession()
	
	// Update connection state in repository
	if m.repo != nil {
		state := conn.ToConnectionState()
		if err := m.repo.UpsertConnectionState(context.Background(), state); err != nil {
			logger.Warnf("⚠️ [MANAGER] Failed to update connection state: %v", err)
		}
	}
	
	return nil
}
