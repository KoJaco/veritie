package connection

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/auth"
	"schma.ai/internal/domain/connection"
	conn_infra "schma.ai/internal/infra/connection"
	"schma.ai/internal/pkg/logger"
)

// Service manages WebSocket connections using the connection infrastructure
type Service struct {
	manager connection.Manager
	pool    connection.Pool
	repo    connection.Repository
	
	// Configuration
	maxConnections int
	cleanupInterval time.Duration
	
	// Control
	mu      sync.RWMutex
	started bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewService creates a new connection service
func NewService(repo connection.Repository, maxConnections int, cleanupInterval time.Duration) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create infrastructure components
	pool := conn_infra.NewPool(maxConnections, cleanupInterval, repo)
	manager := conn_infra.NewManager(pool, repo)
	
	return &Service{
		manager:         manager,
		pool:           pool,
		repo:           repo,
		maxConnections: maxConnections,
		cleanupInterval: cleanupInterval,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the connection service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.started {
		return nil
	}
	
	logger.Infof("🔌 [CONNECTION] Starting connection service with max connections: %d", s.maxConnections)
	
	// Start the connection pool
	if err := s.pool.Start(ctx); err != nil {
		return err
	}
	
	s.started = true
	logger.Infof("✅ [CONNECTION] Connection service started successfully")
	
	return nil
}

// Stop stops the connection service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return nil
	}
	
	logger.Infof("🛑 [CONNECTION] Stopping connection service")
	
	// Stop the connection pool
	if err := s.pool.Stop(ctx); err != nil {
		logger.Errorf("❌ [CONNECTION] Failed to stop connection pool: %v", err)
	}
	
	s.started = false
	s.cancel()
	logger.Infof("✅ [CONNECTION] Connection service stopped successfully")
	
	return nil
}

// CreateConnection creates a new WebSocket connection
func (s *Service) CreateConnection(ctx context.Context, wsSessionID string, principal auth.Principal, remoteAddr, userAgent string, subprotocols []string, llmMode connection.LLMMode) (*connection.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.started {
		return nil, connection.ErrConnectionServiceNotStarted
	}
	
	config := connection.ConnectionConfig{
		WSSessionID:  connection.WSSessionID(wsSessionID),
		Principal:    principal,
		RemoteAddr:   remoteAddr,
		UserAgent:    userAgent,
		Subprotocols: subprotocols,
		LLMMode:      llmMode,
	}
	
	conn, err := s.manager.CreateConnection(ctx, config)
	if err != nil {
		logger.Errorf("❌ [CONNECTION] Failed to create connection: %v", err)
		return nil, err
	}
	
	logger.ServiceDebugf("CONNECTION", "✅ Created connection %s for app %s", conn.ID, principal.AppID.String())
	return conn, nil
}

// GetConnection retrieves a connection by ID
func (s *Service) GetConnection(ctx context.Context, connectionID connection.ConnectionID) (*connection.Connection, bool) {
	return s.manager.GetConnection(ctx, connectionID)
}

// GetConnectionByWSSession retrieves a connection by WebSocket session ID
func (s *Service) GetConnectionByWSSession(wsSessionID string) (*connection.Connection, bool) {
	// This is a convenience method that searches through active connections
	activeConnections := s.pool.GetActiveConnections()
	for _, conn := range activeConnections {
		if string(conn.WSSessionID) == wsSessionID {
			return conn, true
		}
	}
	return nil, false
}

// CloseConnection closes a connection
func (s *Service) CloseConnection(ctx context.Context, connectionID connection.ConnectionID, reason string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.started {
		return connection.ErrConnectionServiceNotStarted
	}
	
	logger.Infof("🔌 [CONNECTION] Closing connection %s: %s", connectionID, reason)
	
	if err := s.manager.CloseConnection(ctx, connectionID, reason); err != nil {
		logger.Errorf("❌ [CONNECTION] Failed to close connection %s: %v", connectionID, err)
		return err
	}
	
	logger.Infof("✅ [CONNECTION] Successfully closed connection %s", connectionID)
	return nil
}

// CloseConnectionByWSSession closes a connection by WebSocket session ID
func (s *Service) CloseConnectionByWSSession(ctx context.Context, wsSessionID string, reason string) error {
	conn, exists := s.GetConnectionByWSSession(wsSessionID)
	if !exists {
		logger.Warnf("⚠️ [CONNECTION] Connection not found for WSSessionID: %s", wsSessionID)
		return connection.ErrConnectionNotFound
	}
	
	return s.CloseConnection(ctx, conn.ID, reason)
}

// ListActiveConnections returns all active connections for a specific app
func (s *Service) ListActiveConnections(ctx context.Context, appID pgtype.UUID) ([]*connection.Connection, error) {
	return s.manager.ListActiveConnections(ctx, appID)
}

// GetConnectionStats returns connection statistics for an app
func (s *Service) GetConnectionStats(ctx context.Context, appID pgtype.UUID) (*connection.ConnectionStats, error) {
	return s.manager.GetConnectionStats(ctx, appID)
}

// GetPoolStats returns the current pool statistics
func (s *Service) GetPoolStats() connection.PoolStats {
	return s.pool.GetPoolStats()
}

// IsConnectionHealthy checks if a connection is healthy
func (s *Service) IsConnectionHealthy(ctx context.Context, connectionID connection.ConnectionID) (bool, error) {
	return s.manager.IsConnectionHealthy(ctx, connectionID)
}

// PingConnection sends a ping to a connection
func (s *Service) PingConnection(ctx context.Context, connectionID connection.ConnectionID) error {
	return s.manager.PingConnection(ctx, connectionID)
}

// UpdateConnectionLLMMode updates the LLM mode of a connection
func (s *Service) UpdateConnectionLLMMode(connectionID connection.ConnectionID, mode connection.LLMMode) error {
	conn, exists := s.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	conn.UpdateLLMMode(mode)
	
	// Update connection state in database
	if s.repo != nil {
		state := conn.ToConnectionState()
		if err := s.repo.UpsertConnectionState(context.Background(), state); err != nil {
			logger.Warnf("⚠️ [CONNECTION] Failed to update connection state: %v", err)
		}
	}
	
	logger.Infof("🔌 [CONNECTION] Updated connection %s LLM mode to %s", connectionID, mode)
	return nil
}

// SetConnectionActiveSession sets the active session for a connection
func (s *Service) SetConnectionActiveSession(connectionID connection.ConnectionID, sessionID pgtype.UUID) error {
	conn, exists := s.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	conn.SetActiveSession(sessionID)
	
	// Update connection state in database
	if s.repo != nil {
		state := conn.ToConnectionState()
		if err := s.repo.UpsertConnectionState(context.Background(), state); err != nil {
			logger.Warnf("⚠️ [CONNECTION] Failed to update connection state: %v", err)
		}
	}
	
	logger.Infof("🔌 [CONNECTION] Set active session %s for connection %s", sessionID.String(), connectionID)
	return nil
}

// ClearConnectionActiveSession clears the active session for a connection
func (s *Service) ClearConnectionActiveSession(connectionID connection.ConnectionID) error {
	conn, exists := s.pool.GetConnection(connectionID)
	if !exists {
		return connection.ErrConnectionNotFound
	}
	
	conn.ClearActiveSession()
	
	// Update connection state in database
	if s.repo != nil {
		state := conn.ToConnectionState()
		if err := s.repo.UpsertConnectionState(context.Background(), state); err != nil {
			logger.Warnf("⚠️ [CONNECTION] Failed to update connection state: %v", err)
		}
	}
	
	logger.Infof("🔌 [CONNECTION] Cleared active session for connection %s", connectionID)
	return nil
}

// GetConnectionCount returns the current connection count
func (s *Service) GetConnectionCount() int {
	stats := s.pool.GetPoolStats()
	return stats.ActiveConnections
}

// IsPoolFull returns true if the pool is at capacity
func (s *Service) IsPoolFull() bool {
	return s.pool.GetPoolStats().ActiveConnections >= s.maxConnections
}

// CleanupStaleConnections cleans up stale connections
func (s *Service) CleanupStaleConnections() {
	// This is handled automatically by the pool's background cleanup routine
	// But we can expose it for manual cleanup if needed
	logger.Debugf("🔌 [CONNECTION] Manual cleanup requested")
}

// GetConnectionByID returns a connection by its ID
func (s *Service) GetConnectionByID(connectionID connection.ConnectionID) (*connection.Connection, bool) {
	return s.pool.GetConnection(connectionID)
}
