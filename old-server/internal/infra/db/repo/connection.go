package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"schma.ai/internal/domain/connection"
	db "schma.ai/internal/infra/db/generated"
)

// ConnectionRepo implements connection.Repository using the database
type ConnectionRepo struct {
	queries *db.Queries
}

// NewConnectionRepo creates a new connection repository
func NewConnectionRepo(pool *pgxpool.Pool) connection.Repository {
	return &ConnectionRepo{
		queries: db.New(pool),
	}
}

// CreateConnectionLog creates a new connection log entry
func (r *ConnectionRepo) CreateConnectionLog(ctx context.Context, event connection.ConnectionEvent) error {
	// Convert domain event to database params
	eventData, err := json.Marshal(event.EventData)
	if err != nil {
		return err
	}

	_, err = r.queries.CreateConnectionLog(ctx, db.CreateConnectionLogParams{
		ConnectionID: string(event.ConnectionID),
		WsSessionID:  string(event.WSSessionID),
		AppID:        event.AppID,
		AccountID:    event.AccountID,
		EventType:    string(event.EventType),
		EventData:    eventData,
	})

	return err
}

// EndConnectionLog updates an existing connection log with end time and final metrics
func (r *ConnectionRepo) EndConnectionLog(ctx context.Context, connectionID connection.ConnectionID, endedAt time.Time, durationMS int, errorMessage *string, errorCode *string, finalMetrics *connection.ConnectionMetrics) error {
	// Get the latest connection log for this connection
	latestLog, err := r.queries.GetLatestConnectionLogByConnID(ctx, string(connectionID))
	if err != nil {
		return err
	}

	// Convert error messages to pgtype.Text
	var errorMsgText pgtype.Text
	if errorMessage != nil {
		errorMsgText = pgtype.Text{String: *errorMessage, Valid: true}
	}

	var errorCodeText pgtype.Text
	if errorCode != nil {
		errorCodeText = pgtype.Text{String: *errorCode, Valid: true}
	}

	// Safely handle finalMetrics optional fields
	var messagesSent, messagesReceived, audioChunks int32
	if finalMetrics != nil {
		messagesSent = int32(finalMetrics.MessagesSent)
		messagesReceived = int32(finalMetrics.MessagesReceived)
		audioChunks = int32(finalMetrics.AudioChunksProcessed)
	}

	// Update the connection log with end information
	_, err = r.queries.EndConnectionLog(ctx, db.EndConnectionLogParams{
		ID:                   latestLog.ID,
		EndedAt:             pgtype.Timestamp{Time: endedAt, Valid: true},
		DurationMs:          pgtype.Int4{Int32: int32(durationMS), Valid: true},
		ErrorMessage:        errorMsgText,
		ErrorCode:           errorCodeText,
		MessagesSent:        messagesSent,
		MessagesReceived:    messagesReceived,
		AudioChunksProcessed: audioChunks,
	})

	return err
}

// AppendConnectionEvent adds a new event to an existing connection
func (r *ConnectionRepo) AppendConnectionEvent(ctx context.Context, event connection.ConnectionEvent) error {
	eventData, err := json.Marshal(event.EventData)
	if err != nil {
		return err
	}

	_, err = r.queries.AppendConnectionEvent(ctx, db.AppendConnectionEventParams{
		ConnectionID: string(event.ConnectionID),
		WsSessionID:  string(event.WSSessionID),
		AppID:        event.AppID,
		AccountID:    event.AccountID,
		EventType:    string(event.EventType),
		EventData:    eventData,
	})

	return err
}

// UpsertConnectionState creates or updates a connection state
func (r *ConnectionRepo) UpsertConnectionState(ctx context.Context, state connection.ConnectionState) error {
	// Guard nil ActiveSessionID
	activeUUID := pgtype.UUID{}
	if state.ActiveSessionID != nil {
		activeUUID = pgtype.UUID{Bytes: state.ActiveSessionID.Bytes, Valid: true}
	}

	_, err := r.queries.UpsertConnectionState(ctx, db.UpsertConnectionStateParams{
		ConnectionID:            string(state.ConnectionID),
		WsSessionID:            string(state.WSSessionID),
		AppID:                  state.AppID,
		AccountID:              state.AccountID,
		LlmMode:                pgtype.Text{String: string(state.LLMMode), Valid: true},
		ActiveSessionID:        activeUUID,
		ConnectionStatus:       string(state.ConnectionStatus),
		SttProvider:            pgtype.Text{String: state.STTProvider, Valid: true},
		Column9:                int32(state.FunctionDefinitionsCount),
		Column10:               state.StructuredSchemaPresent,
		LastActivity:           pgtype.Timestamp{Time: state.LastActivity, Valid: true},
		PingLatencyMs:          pgtype.Int4{Int32: int32(state.PingLatencyMS), Valid: true},
		LastError:              pgtype.Text{String: state.LastError, Valid: true},
		Column14:               int32(state.ErrorCount),
	})

	return err
}

// UpdateConnectionStateOnClose marks a connection state as closed
func (r *ConnectionRepo) UpdateConnectionStateOnClose(ctx context.Context, connectionID connection.ConnectionID) error {
	return r.queries.UpdateConnectionStateOnClose(ctx, string(connectionID))
}

// GetConnectionState retrieves a connection state by ID
func (r *ConnectionRepo) GetConnectionState(ctx context.Context, connectionID connection.ConnectionID) (*connection.ConnectionState, error) {
	dbState, err := r.queries.GetConnectionStateByConnID(ctx, string(connectionID))
	if err != nil {
		return nil, err
	}

	// Convert database state to domain state
	state := &connection.ConnectionState{
		ConnectionID:            connection.ConnectionID(dbState.ConnectionID),
		WSSessionID:            connection.WSSessionID(dbState.WsSessionID),
		AppID:                  dbState.AppID,
		AccountID:              dbState.AccountID,
		LLMMode:                connection.LLMMode(dbState.LlmMode.String),
		ActiveSessionID:        &dbState.ActiveSessionID,
		ConnectionStatus:       connection.ConnectionStatus(dbState.ConnectionStatus),
		STTProvider:            dbState.SttProvider.String,
		FunctionDefinitionsCount: int(dbState.FunctionDefinitionsCount),
		StructuredSchemaPresent:  dbState.StructuredSchemaPresent,
		LastActivity:           dbState.LastActivity.Time,
		PingLatencyMS:          int(dbState.PingLatencyMs.Int32),
		LastError:              dbState.LastError.String,
		ErrorCount:             int(dbState.ErrorCount),
		CreatedAt:              dbState.CreatedAt.Time,
		UpdatedAt:              dbState.UpdatedAt.Time,
	}

	return state, nil
}

// ListConnectionLogsByApp retrieves connection logs for a specific app
func (r *ConnectionRepo) ListConnectionLogsByApp(ctx context.Context, appID pgtype.UUID, since time.Time, limit, offset int) ([]connection.ConnectionEvent, error) {
	dbLogs, err := r.queries.ListConnectionLogsByApp(ctx, db.ListConnectionLogsByAppParams{
		AppID:      appID,
		StartedAt:  pgtype.Timestamp{Time: since, Valid: true},
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, err
	}

	// Convert database logs to domain events
	events := make([]connection.ConnectionEvent, 0, len(dbLogs))
	for _, dbLog := range dbLogs {
		event := connection.ConnectionEvent{
			ConnectionID: connection.ConnectionID(dbLog.ConnectionID),
			WSSessionID:  connection.WSSessionID(dbLog.WsSessionID),
			AppID:        dbLog.AppID,
			AccountID:    dbLog.AccountID,
			EventType:    connection.EventType(dbLog.EventType),
			Timestamp:   dbLog.StartedAt.Time,
		}

		// Parse event data if present
		if dbLog.EventData != nil {
			var eventData map[string]interface{}
			if err := json.Unmarshal(dbLog.EventData, &eventData); err == nil {
				event.EventData = eventData
			}
		}

		events = append(events, event)
	}

	return events, nil
}

// ListActiveConnectionsByApp retrieves active connections for a specific app
func (r *ConnectionRepo) ListActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]connection.ConnectionEvent, error) {
	dbLogs, err := r.queries.ListActiveConnectionsByApp(ctx, db.ListActiveConnectionsByAppParams{
		AppID:  appID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	// Convert database logs to domain events
	events := make([]connection.ConnectionEvent, 0, len(dbLogs))
	for _, dbLog := range dbLogs {
		event := connection.ConnectionEvent{
			ConnectionID: connection.ConnectionID(dbLog.ConnectionID),
			WSSessionID:  connection.WSSessionID(dbLog.WsSessionID),
			AppID:        dbLog.AppID,
			AccountID:    dbLog.AccountID,
			EventType:    connection.EventType(dbLog.EventType),
			Timestamp:   dbLog.StartedAt.Time,
		}

		// Parse event data if present
		if dbLog.EventData != nil {
			var eventData map[string]interface{}
			if err := json.Unmarshal(dbLog.EventData, &eventData); err == nil {
				event.EventData = eventData
			}
		}

		events = append(events, event)
	}

	return events, nil
}

// CountActiveConnectionsByApp counts active connections for a specific app
func (r *ConnectionRepo) CountActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID) (int, error) {
	count, err := r.queries.CountActiveConnectionsByApp(ctx, appID)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// ListActiveConnectionStatesByApp retrieves active connection states for a specific app
func (r *ConnectionRepo) ListActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]connection.ConnectionState, error) {
	dbStates, err := r.queries.ListActiveConnectionStatesByApp(ctx, db.ListActiveConnectionStatesByAppParams{
		AppID:  appID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	// Convert database states to domain states
	states := make([]connection.ConnectionState, 0, len(dbStates))
	for _, dbState := range dbStates {
		state := connection.ConnectionState{
			ConnectionID:            connection.ConnectionID(dbState.ConnectionID),
			WSSessionID:            connection.WSSessionID(dbState.WsSessionID),
			AppID:                  dbState.AppID,
			AccountID:              dbState.AccountID,
			LLMMode:                connection.LLMMode(dbState.LlmMode.String),
			ActiveSessionID:        &dbState.ActiveSessionID,
			ConnectionStatus:       connection.ConnectionStatus(dbState.ConnectionStatus),
			STTProvider:            dbState.SttProvider.String,
			FunctionDefinitionsCount: int(dbState.FunctionDefinitionsCount),
			StructuredSchemaPresent:  dbState.StructuredSchemaPresent,
			LastActivity:           dbState.LastActivity.Time,
			PingLatencyMS:          int(dbState.PingLatencyMs.Int32),
			LastError:              dbState.LastError.String,
			ErrorCount:             int(dbState.ErrorCount),
			CreatedAt:              dbState.CreatedAt.Time,
			UpdatedAt:              dbState.UpdatedAt.Time,
		}

		states = append(states, state)
	}

	return states, nil
}

// CountActiveConnectionStatesByApp counts active connection states for a specific app
func (r *ConnectionRepo) CountActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID) (int, error) {
	count, err := r.queries.CountActiveConnectionStatesByApp(ctx, appID)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// CleanupStaleConnections removes stale connection states
func (r *ConnectionRepo) CleanupStaleConnections(ctx context.Context, olderThan time.Time) (int, error) {
	// TODO: Implement cleanup query
	// For now, return 0 as placeholder
	return 0, nil
}

// ArchiveConnectionLogs archives old connection logs
func (r *ConnectionRepo) ArchiveConnectionLogs(ctx context.Context, olderThan time.Time) (int, error) {
	// TODO: Implement archive query
	// For now, return 0 as placeholder
	return 0, nil
}
