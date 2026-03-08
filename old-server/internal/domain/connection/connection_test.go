package connection

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"schma.ai/internal/domain/auth"
)

func TestNewConnection(t *testing.T) {
	principal := auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}},
		AccountID: pgtype.UUID{Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}},
	}

	config := ConnectionConfig{
		WSSessionID:  "ws_session_123",
		Principal:    principal,
		RemoteAddr:   "192.168.1.1:8080",
		UserAgent:    "test-agent",
		Subprotocols: []string{"schma.ws.v1"},
		LLMMode:      LLMModeFunctions,
	}

	conn := NewConnection(config)

	assert.NotEmpty(t, conn.ID)
	assert.Equal(t, config.WSSessionID, conn.WSSessionID)
	assert.Equal(t, config.Principal, conn.Principal)
	assert.Equal(t, ConnectionStatusConnecting, conn.Status)
	assert.Equal(t, config.LLMMode, conn.LLMMode)
	assert.Equal(t, config.RemoteAddr, conn.RemoteAddr)
	assert.Equal(t, config.UserAgent, conn.UserAgent)
	assert.Equal(t, config.Subprotocols, conn.Subprotocols)
	assert.False(t, conn.CreatedAt.IsZero())
	assert.False(t, conn.LastActivity.IsZero())
	assert.Nil(t, conn.ClosedAt)
}

func TestConnection_Activate(t *testing.T) {
	conn := &Connection{
		Status:        ConnectionStatusConnecting,
		LastActivity:  time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.Activate()

	assert.Equal(t, ConnectionStatusActive, conn.Status)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_SetIdle(t *testing.T) {
	conn := &Connection{
		Status:        ConnectionStatusActive,
		LastActivity:  time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.SetIdle()

	assert.Equal(t, ConnectionStatusIdle, conn.Status)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_Close(t *testing.T) {
	conn := &Connection{
		Status:        ConnectionStatusActive,
		LastActivity:  time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.Close()

	assert.Equal(t, ConnectionStatusClosed, conn.Status)
	assert.True(t, conn.LastActivity.After(oldActivity))
	assert.NotNil(t, conn.ClosedAt)
}

func TestConnection_SetActiveSession(t *testing.T) {
	conn := &Connection{
		LastActivity: time.Now().Add(-time.Hour),
	}

	sessionID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}}
	oldActivity := conn.LastActivity

	conn.SetActiveSession(sessionID)

	assert.Equal(t, &sessionID, conn.ActiveSessionID)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_ClearActiveSession(t *testing.T) {
	sessionID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}}
	conn := &Connection{
		ActiveSessionID: &sessionID,
		LastActivity:    time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.ClearActiveSession()

	assert.Nil(t, conn.ActiveSessionID)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_RecordMessageSent(t *testing.T) {
	conn := &Connection{
		MessagesSent:  5,
		LastActivity:  time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.RecordMessageSent()

	assert.Equal(t, 6, conn.MessagesSent)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_RecordMessageReceived(t *testing.T) {
	conn := &Connection{
		MessagesReceived: 3,
		LastActivity:     time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.RecordMessageReceived()

	assert.Equal(t, 4, conn.MessagesReceived)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_RecordAudioChunk(t *testing.T) {
	conn := &Connection{
		AudioChunksProcessed: 10,
		LastActivity:         time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.RecordAudioChunk()

	assert.Equal(t, 11, conn.AudioChunksProcessed)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_RecordError(t *testing.T) {
	conn := &Connection{
		ErrorCount:   2,
		LastActivity: time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	testErr := errors.New("test error")
	conn.RecordError(testErr, "TEST_ERROR")

	assert.Equal(t, testErr.Error(), conn.LastError)
	assert.Equal(t, "TEST_ERROR", conn.ErrorCode)
	assert.Equal(t, 3, conn.ErrorCount)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_UpdatePingLatency(t *testing.T) {
	conn := &Connection{
		PingLatencyMS: 50,
		LastActivity:  time.Now().Add(-time.Hour),
	}

	oldActivity := conn.LastActivity
	conn.UpdatePingLatency(75)

	assert.Equal(t, 75, conn.PingLatencyMS)
	assert.True(t, conn.LastActivity.After(oldActivity))
}

func TestConnection_GetDuration(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)
	conn := &Connection{
		CreatedAt: createdAt,
	}

	// Test active connection duration
	duration := conn.GetDuration()
	assert.True(t, duration >= time.Hour)
	assert.True(t, duration <= time.Hour+time.Second)

	// Test closed connection duration
	closedAt := time.Now()
	conn.ClosedAt = &closedAt
	duration = conn.GetDuration()
	assert.True(t, duration >= time.Hour)
	assert.True(t, duration <= time.Hour+time.Millisecond)
}

func TestConnection_IsActive(t *testing.T) {
	activeConn := &Connection{Status: ConnectionStatusActive}
	idleConn := &Connection{Status: ConnectionStatusIdle}
	closedConn := &Connection{Status: ConnectionStatusClosed}

	assert.True(t, activeConn.IsActive())
	assert.True(t, idleConn.IsActive())
	assert.False(t, closedConn.IsActive())
}

func TestConnection_IsClosed(t *testing.T) {
	activeConn := &Connection{Status: ConnectionStatusActive}
	closedConn := &Connection{Status: ConnectionStatusClosed}

	assert.False(t, activeConn.IsClosed())
	assert.True(t, closedConn.IsClosed())
}

func TestConnection_ToConnectionState(t *testing.T) {
	principal := auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}},
		AccountID: pgtype.UUID{Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}},
	}

	sessionID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}}
	conn := &Connection{
		ID:                        "conn_123",
		WSSessionID:               "ws_session_123",
		Principal:                 principal,
		Status:                    ConnectionStatusActive,
		LLMMode:                   LLMModeFunctions,
		ActiveSessionID:           &sessionID,
		STTProvider:               "deepgram",
		FunctionDefinitionsCount:  5,
		StructuredSchemaPresent:   false,
		LastActivity:              time.Now(),
		PingLatencyMS:             25,
		LastError:                 "",
		ErrorCount:                0,
		CreatedAt:                 time.Now().Add(-time.Hour),
	}

	state := conn.ToConnectionState()

	assert.Equal(t, conn.ID, state.ConnectionID)
	assert.Equal(t, conn.WSSessionID, state.WSSessionID)
	assert.Equal(t, conn.Principal.AppID, state.AppID)
	assert.Equal(t, conn.Principal.AccountID, state.AccountID)
	assert.Equal(t, conn.LLMMode, state.LLMMode)
	assert.Equal(t, conn.ActiveSessionID, state.ActiveSessionID)
	assert.Equal(t, conn.Status, state.ConnectionStatus)
	assert.Equal(t, conn.STTProvider, state.STTProvider)
	assert.Equal(t, conn.FunctionDefinitionsCount, state.FunctionDefinitionsCount)
	assert.Equal(t, conn.StructuredSchemaPresent, state.StructuredSchemaPresent)
	assert.Equal(t, conn.LastActivity, state.LastActivity)
	assert.Equal(t, conn.PingLatencyMS, state.PingLatencyMS)
	assert.Equal(t, conn.LastError, state.LastError)
	assert.Equal(t, conn.ErrorCount, state.ErrorCount)
	assert.Equal(t, conn.CreatedAt, state.CreatedAt)
	assert.False(t, state.UpdatedAt.IsZero())
}

func TestConnection_ToConnectionEvent(t *testing.T) {
	principal := auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}},
		AccountID: pgtype.UUID{Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}},
	}

	conn := &Connection{
		ID:           "conn_123",
		WSSessionID:  "ws_session_123",
		Principal:    principal,
	}

	data := map[string]interface{}{
		"test_key": "test_value",
		"count":    42,
	}

	event := conn.ToConnectionEvent(EventTypeConnect, data)

	assert.Equal(t, conn.ID, event.ConnectionID)
	assert.Equal(t, conn.WSSessionID, event.WSSessionID)
	assert.Equal(t, conn.Principal.AppID, event.AppID)
	assert.Equal(t, conn.Principal.AccountID, event.AccountID)
	assert.Equal(t, EventTypeConnect, event.EventType)
	assert.Equal(t, data, event.EventData)
	assert.False(t, event.Timestamp.IsZero())
}
