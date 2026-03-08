package connection

import (
	"context"
	"sync"
	"time"

	"schma.ai/internal/domain/connection"
	"schma.ai/internal/pkg/logger"
)

// pool implements connection.Pool
type pool struct {
	connections map[connection.ConnectionID]*connection.Connection
	mu          sync.RWMutex
	stats       connection.PoolStats
	
	// Configuration
	maxConnections int
	cleanupInterval time.Duration
	
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Repository for persistence
	repo connection.Repository
}

// NewPool creates a new connection pool
func NewPool(maxConnections int, cleanupInterval time.Duration, repo connection.Repository) connection.Pool {
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &pool{
		connections:     make(map[connection.ConnectionID]*connection.Connection),
		maxConnections:  maxConnections,
		cleanupInterval: cleanupInterval,
		ctx:            ctx,
		cancel:         cancel,
		repo:           repo,
		stats: connection.PoolStats{
			MaxConnections: maxConnections,
		},
	}
	
	// Start background cleanup goroutine
	pool.startCleanupRoutine()
	
	return pool
}

// AddConnection adds a new connection to the pool
func (p *pool) AddConnection(conn *connection.Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Check connection limit
	if len(p.connections) >= p.maxConnections {
		return connection.ErrConnectionPoolFull
	}
	
	// Add connection
	p.connections[conn.ID] = conn
	
	// Update stats
	p.stats.TotalConnections++
	p.stats.ActiveConnections++
	if len(p.connections) > int(p.stats.PeakConnections) {
		p.stats.PeakConnections = int(len(p.connections))
	}
	
	// Log connection event
	if p.repo != nil {
		event := conn.ToConnectionEvent(connection.EventTypeConnect, map[string]interface{}{
			"remote_addr":   conn.RemoteAddr,
			"subprotocols": conn.Subprotocols,
			"llm_mode":      conn.LLMMode,
		})
		
		if err := p.repo.CreateConnectionLog(context.Background(), event); err != nil {
			logger.Warnf("⚠️ [POOL] Failed to log connection event: %v", err)
		}
	}
	
	logger.ServiceDebugf("POOL", "Connection %s added. Active: %d/%d", 
		conn.ID, p.stats.ActiveConnections, p.maxConnections)
	
	return nil
}

// RemoveConnection removes a connection from the pool
func (p *pool) RemoveConnection(connectionID connection.ConnectionID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if conn, exists := p.connections[connectionID]; exists {
		delete(p.connections, connectionID)
		p.stats.ActiveConnections--
		p.stats.TotalDisconnections++
		
		// Log connection end event
		if p.repo != nil {
			duration := int(conn.GetDuration().Milliseconds())
			
			event := conn.ToConnectionEvent(connection.EventTypeDisconnect, map[string]interface{}{
				"duration_ms":           duration,
				"messages_sent":         conn.MessagesSent,
				"messages_received":     conn.MessagesReceived,
				"audio_chunks_processed": conn.AudioChunksProcessed,
				"final_status":          conn.Status,
			})
			
			if err := p.repo.CreateConnectionLog(context.Background(), event); err != nil {
				logger.Warnf("⚠️ [POOL] Failed to log connection end event: %v", err)
			}
			
			// Update connection state as closed
			if err := p.repo.UpdateConnectionStateOnClose(context.Background(), connectionID); err != nil {
				logger.Warnf("⚠️ [POOL] Failed to update connection state: %v", err)
			}
		}
		
		logger.ServiceDebugf("POOL", "Connection %s removed. Active: %d/%d", 
			connectionID, p.stats.ActiveConnections, p.maxConnections)
	}
	
	return nil
}

// GetConnection retrieves a connection by ID
func (p *pool) GetConnection(connectionID connection.ConnectionID) (*connection.Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	conn, exists := p.connections[connectionID]
	return conn, exists
}

// GetPoolStats returns current pool statistics
func (p *pool) GetPoolStats() connection.PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return p.stats
}

// GetActiveConnections returns all active connections
func (p *pool) GetActiveConnections() []*connection.Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	connections := make([]*connection.Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	
	return connections
}

// Start starts the connection pool
func (p *pool) Start(ctx context.Context) error {
	logger.Infof("🔌 [POOL] Starting connection pool with max connections: %d", p.maxConnections)
	return nil
}

// Stop stops the connection pool and closes all connections
func (p *pool) Stop(ctx context.Context) error {
	p.cancel() // Stop cleanup routine
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	logger.ServiceDebugf("POOL", "Stopping connection pool with %d active connections", len(p.connections))
	
	// Close all connections gracefully
	for id, conn := range p.connections {
		logger.ServiceDebugf("POOL", "Closing connection %s", id)
		
		// Mark connection as closing
		conn.SetClosing()
		
		// Log final state
		if p.repo != nil {
			state := conn.ToConnectionState()
			if err := p.repo.UpsertConnectionState(ctx, state); err != nil {
				logger.Warnf("⚠️ [POOL] Failed to persist final connection state: %v", err)
			}
		}
		
		// Mark as closed
		conn.Close()
		
		delete(p.connections, id)
	}
	
	p.stats.ActiveConnections = 0
	
	// Wait for cleanup routine to finish
	p.wg.Wait()
	
	logger.Infof("🔌 [POOL] Connection pool stopped successfully")
	return nil
}

// startCleanupRoutine starts background cleanup of stale connections
func (p *pool) startCleanupRoutine() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		
		ticker := time.NewTicker(p.cleanupInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.cleanupStaleConnections()
			}
		}
	}()
}

// cleanupStaleConnections removes connections that are no longer active
func (p *pool) cleanupStaleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	now := time.Now()
	staleCount := 0
	
	for id, conn := range p.connections {
		if p.isConnectionStale(conn, now) {
			logger.ServiceDebugf("POOL", "Removing stale connection %s", id)
			
			// Log cleanup event
			if p.repo != nil {
				event := conn.ToConnectionEvent(connection.EventTypeTimeout, map[string]interface{}{
					"reason": "stale_connection",
					"last_activity": conn.LastActivity,
				})
				
				if err := p.repo.CreateConnectionLog(context.Background(), event); err != nil {
					logger.Warnf("⚠️ [POOL] Failed to log cleanup event: %v", err)
				}
			}
			
			delete(p.connections, id)
			p.stats.ActiveConnections--
			p.stats.TotalDisconnections++
			staleCount++
		}
	}
	
	if staleCount > 0 {
		p.stats.LastCleanup = now
		logger.Infof("🔌 [POOL] Cleaned up %d stale connections. Active: %d", 
			staleCount, p.stats.ActiveConnections)
	}
}

// isConnectionStale checks if a connection is stale
func (p *pool) isConnectionStale(conn *connection.Connection, now time.Time) bool {
	// Consider connection stale if inactive for more than 10 minutes
	return now.Sub(conn.LastActivity) > 10*time.Minute
}

// UpdateConnectionState updates the state of a connection in the pool
func (p *pool) UpdateConnectionState(conn *connection.Connection) error {
	if p.repo == nil {
		return nil
	}
	
	state := conn.ToConnectionState()
	return p.repo.UpsertConnectionState(context.Background(), state)
}

// LogConnectionEvent logs an event for a specific connection
func (p *pool) LogConnectionEvent(conn *connection.Connection, eventType connection.EventType, data map[string]interface{}) error {
	if p.repo == nil {
		return nil
	}
	
	event := conn.ToConnectionEvent(eventType, data)
	return p.repo.CreateConnectionLog(context.Background(), event)
}

// GetConnectionByWSSessionID retrieves a connection by WebSocket session ID
func (p *pool) GetConnectionByWSSessionID(wsSessionID connection.WSSessionID) (*connection.Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	for _, conn := range p.connections {
		if conn.WSSessionID == wsSessionID {
			return conn, true
		}
	}
	
	return nil, false
}

// ListConnectionsByApp returns all connections for a specific app
func (p *pool) ListConnectionsByApp(appID string) []*connection.Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	var appConnections []*connection.Connection
	for _, conn := range p.connections {
		if conn.Principal.AppID.String() == appID {
			appConnections = append(appConnections, conn)
		}
	}
	
	return appConnections
}

// GetConnectionCount returns the current connection count
func (p *pool) GetConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return len(p.connections)
}

// IsPoolFull returns true if the pool is at capacity
func (p *pool) IsPoolFull() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return len(p.connections) >= p.maxConnections
}
