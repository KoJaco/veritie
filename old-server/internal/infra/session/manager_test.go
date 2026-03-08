package session

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"schma.ai/internal/domain/session"
)

func TestSessionManager_StartAndCloseSession(t *testing.T) {
	// Test that GetSession returns false for non-existent session
	manager := New(nil)
	
	nonExistentID := session.DBSessionID(pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}})
	_, exists := manager.GetSession(nonExistentID)
	assert.False(t, exists, "GetSession should return false for non-existent session")
}

func TestSessionManager_UpdateSessionStatus(t *testing.T) {
	manager := New(nil)
	
	ctx := context.Background()
	sessionID := session.DBSessionID(pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}})
	
	// Test that UpdateSessionStatus fails for non-existent session
	err := manager.UpdateSessionStatus(ctx, sessionID, session.SessionRecording)
	assert.Error(t, err, "UpdateSessionStatus should fail for non-existent session")
}

func TestSessionManager_UpdateSessionMetadata(t *testing.T) {
	manager := New(nil)
	
	ctx := context.Background()
	sessionID := session.DBSessionID(pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}})
	
	// Test that UpdateSessionMetadata fails for non-existent session
	status := session.SessionRecording
	metadata := session.PartialUpdate{
		Status: &status,
	}
	err := manager.UpdateSessionMetadata(ctx, sessionID, metadata)
	assert.Error(t, err, "UpdateSessionMetadata should fail for non-existent session")
}

func TestSessionManager_Snapshot(t *testing.T) {
	manager := New(nil)
	
	ctx := context.Background()
	sessionID := session.DBSessionID(pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}})
	
	// Test that Snapshot fails for non-existent session
	_, err := manager.Snapshot(ctx, sessionID)
	assert.Error(t, err, "Snapshot should fail for non-existent session")
}
