package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/connection"
	db "schma.ai/internal/infra/db/generated"
)

// repository implements connection.Repository
type repository struct {
	queries *db.Queries
}

// NewRepository creates a new connection repository
func NewRepository(queries *db.Queries) connection.Repository {
	return &repository{
		queries: queries,
	}
}

// CreateConnectionLog creates a new connection log entry
func (r *repository) CreateConnectionLog(ctx context.Context, event connection.ConnectionEvent) error {
	// Convert domain event to database params
	eventData, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	_, err = r.queries.CreateConnectionLog(ctx, db.CreateConnectionLogParams{
		ConnectionID: string(event.ConnectionID),
		WsSessionID:  string(event.WSSessionID),
		AppID:        event.AppID,
		AccountID:    event.AccountID,
		EventType:    string(event.EventType),
		EventData:    eventData,
	})

	if err != nil {
		return fmt.Errorf("failed to create connection log: %w", err)
	}

	return nil
}

// EndConnectionLog updates an existing connection log with end time and final metrics
func (r *repository) EndConnectionLog(ctx context.Context, connectionID connection.ConnectionID, endedAt time.Time, durationMS int, errorMessage *string, errorCode *string, finalMetrics *connection.ConnectionMetrics) error {
	var err error

	// Get the latest connection log for this connection
	latestLog, err := r.queries.GetLatestConnectionLogByConnID(ctx, string(connectionID))
	if err != nil {
		return fmt.Errorf("failed to get latest connection log: %w", err)
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

	// Update the connection log with end information
	_, err = r.queries.EndConnectionLog(ctx, db.EndConnectionLogParams{
		ID:                   latestLog.ID,
		EndedAt:             pgtype.Timestamp{Time: endedAt, Valid: true},
		DurationMs:          pgtype.Int4{Int32: int32(durationMS), Valid: true},
		ErrorMessage:        errorMsgText,
		ErrorCode:           errorCodeText,
		MessagesSent:        int32(finalMetrics.MessagesSent),
		MessagesReceived:    int32(finalMetrics.MessagesReceived),
		AudioChunksProcessed: int32(finalMetrics.AudioChunksProcessed),
	})

	if err != nil {
		return fmt.Errorf("failed to end connection log: %w", err)
	}

	return nil
}

// AppendConnectionEvent adds a new event to an existing connection
func (r *repository) AppendConnectionEvent(ctx context.Context, event connection.ConnectionEvent) error {
	// Convert domain event to database params
	eventData, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	_, err = r.queries.AppendConnectionEvent(ctx, db.AppendConnectionEventParams{
		ConnectionID: string(event.ConnectionID),
		WsSessionID:  string(event.WSSessionID),
		AppID:        event.AppID,
		AccountID:    event.AccountID,
		EventType:    string(event.EventType),
		EventData:    eventData,
	})

	if err != nil {
		return fmt.Errorf("failed to append connection event: %w", err)
	}

	return nil
}

// UpsertConnectionState creates or updates a connection state
func (r *repository) UpsertConnectionState(ctx context.Context, state connection.ConnectionState) error {
	// Convert domain state to database params
	_, err := r.queries.UpsertConnectionState(ctx, db.UpsertConnectionStateParams{
		ConnectionID:            string(state.ConnectionID),
		WsSessionID:            string(state.WSSessionID),		
		AppID:                  state.AppID,
		AccountID:              state.AccountID,
		LlmMode:                pgtype.Text{String: string(state.LLMMode), Valid: true},
		ActiveSessionID:        pgtype.UUID{Bytes: state.ActiveSessionID.Bytes, Valid: state.ActiveSessionID != nil},
		ConnectionStatus:       string(state.ConnectionStatus),
		SttProvider:            pgtype.Text{String: state.STTProvider, Valid: true},
		Column9:                int32(state.FunctionDefinitionsCount),
		Column10:               state.StructuredSchemaPresent,
		LastActivity:           pgtype.Timestamp{Time: state.LastActivity, Valid: true},
		PingLatencyMs:          pgtype.Int4{Int32: int32(state.PingLatencyMS), Valid: true},
		LastError:              pgtype.Text{String: state.LastError, Valid: true},
		Column14:               int32(state.ErrorCount),
	})

	if err != nil {
		return fmt.Errorf("failed to upsert connection state: %w", err)
	}

	return nil
}

// UpdateConnectionStateOnClose marks a connection state as closed
func (r *repository) UpdateConnectionStateOnClose(ctx context.Context, connectionID connection.ConnectionID) error {
	err := r.queries.UpdateConnectionStateOnClose(ctx, string(connectionID))
	if err != nil {
		return fmt.Errorf("failed to update connection state on close: %w", err)
	}

	return nil
}

// GetConnectionState retrieves a connection state by ID
func (r *repository) GetConnectionState(ctx context.Context, connectionID connection.ConnectionID) (*connection.ConnectionState, error) {
	dbState, err := r.queries.GetConnectionStateByConnID(ctx, string(connectionID))
	if err != nil {
		return nil, fmt.Errorf("failed to get connection state: %w", err)
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
func (r *repository) ListConnectionLogsByApp(ctx context.Context, appID pgtype.UUID, since time.Time, limit, offset int) ([]connection.ConnectionEvent, error) {
	dbLogs, err := r.queries.ListConnectionLogsByApp(ctx, db.ListConnectionLogsByAppParams{
		AppID:      appID,
		StartedAt:  pgtype.Timestamp{Time: since, Valid: true},
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list connection logs: %w", err)
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
func (r *repository) ListActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]connection.ConnectionEvent, error) {
	dbLogs, err := r.queries.ListActiveConnectionsByApp(ctx, db.ListActiveConnectionsByAppParams{
		AppID:  appID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list active connections: %w", err)
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
func (r *repository) CountActiveConnectionsByApp(ctx context.Context, appID pgtype.UUID) (int, error) {
	count, err := r.queries.CountActiveConnectionsByApp(ctx, appID)
	if err != nil {
		return 0, fmt.Errorf("failed to count active connections: %w", err)
	}

	return int(count), nil
}

// ListActiveConnectionStatesByApp retrieves active connection states for a specific app
func (r *repository) ListActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]connection.ConnectionState, error) {
	dbStates, err := r.queries.ListActiveConnectionStatesByApp(ctx, db.ListActiveConnectionStatesByAppParams{
		AppID:  appID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list active connection states: %w", err)
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
func (r *repository) CountActiveConnectionStatesByApp(ctx context.Context, appID pgtype.UUID) (int, error) {
	count, err := r.queries.CountActiveConnectionStatesByApp(ctx, appID)
	if err != nil {
		return 0, fmt.Errorf("failed to count active connection states: %w", err)
	}

	return int(count), nil
}

// CleanupStaleConnections removes stale connection states
func (r *repository) CleanupStaleConnections(ctx context.Context, olderThan time.Time) (int, error) {
	// This would need a new query in the SQLC queries
	// For now, return 0 as placeholder
	return 0, nil
}

// ArchiveConnectionLogs archives old connection logs
func (r *repository) ArchiveConnectionLogs(ctx context.Context, olderThan time.Time) (int, error) {
	// This would need a new query in the SQLC queries
	// For now, return 0 as placeholder
	return 0, nil
}
