package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"

	"schma.ai/internal/app/connection"
	"schma.ai/internal/app/draft"
	"schma.ai/internal/app/mappers"
	"schma.ai/internal/app/pipeline"
	"schma.ai/internal/app/prompts"
	silence_app "schma.ai/internal/app/silence"
	app_usage "schma.ai/internal/app/usage"
	domain_configwatcher "schma.ai/internal/domain/configwatcher"
	"schma.ai/internal/domain/silence"
	"schma.ai/internal/domain/speech"
	domain_usage "schma.ai/internal/domain/usage"
	"schma.ai/internal/infra/audio"
	infra_configwatcher "schma.ai/internal/infra/configwatcher"
	db "schma.ai/internal/infra/db/generated"
	"schma.ai/internal/infra/llmgemini"
	"schma.ai/internal/pkg/env"
	"schma.ai/internal/pkg/logger"

	domain_auth "schma.ai/internal/domain/auth"
	domconn "schma.ai/internal/domain/connection"
	domain_session "schma.ai/internal/domain/session"
	pkg_auth "schma.ai/internal/pkg/auth"

	http_middleware "schma.ai/internal/transport/http/middleware"
)

// WebSocket timeout configuration
type WSConfig struct {
	// Overall session timeout (how long the WebSocket can stay connected)
	SessionTimeout time.Duration
	// Read deadline (how long to wait for data from client)
	ReadTimeout time.Duration
	// Ping interval (how often to send ping messages)
	PingInterval time.Duration
	// Ping timeout (how long to wait for pong response)
	PingTimeout time.Duration
}

// getWSConfig loads WebSocket configuration from environment variables
func getWSConfig() WSConfig {
	// Default to conservative values for long sessions
	defaults := WSConfig{
		SessionTimeout: 2 * time.Hour,  // 2 hours max session
		ReadTimeout:    5 * time.Minute, // 5 minutes read timeout
		PingInterval:   30 * time.Second, // 30 seconds ping
		PingTimeout:    10 * time.Second, // 10 seconds pong wait
	}

	// Override with environment variables if set
	if sessionTimeout := os.Getenv("WS_SESSION_TIMEOUT_SECONDS"); sessionTimeout != "" {
		if seconds, err := strconv.Atoi(sessionTimeout); err == nil && seconds > 0 {
			defaults.SessionTimeout = time.Duration(seconds) * time.Second
		}
	}

	if readTimeout := os.Getenv("WS_READ_TIMEOUT_SECONDS"); readTimeout != "" {
		if seconds, err := strconv.Atoi(readTimeout); err == nil && seconds > 0 {
			defaults.ReadTimeout = time.Duration(seconds) * time.Second
		}
	}

	if pingInterval := os.Getenv("WS_PING_INTERVAL_SECONDS"); pingInterval != "" {
		if seconds, err := strconv.Atoi(pingInterval); err == nil && seconds > 0 {
			defaults.PingInterval = time.Duration(seconds) * time.Second
		}
	}

	if pingTimeout := os.Getenv("WS_PING_TIMEOUT_SECONDS"); pingTimeout != "" {
		if seconds, err := strconv.Atoi(pingTimeout); err == nil && seconds > 0 {
			defaults.PingTimeout = time.Duration(seconds) * time.Second
		}
	}

	return defaults
}

// parseStructuredJSONSchema converts a raw JSON schema into the domain speech.StructuredOutputSchema
func parseStructuredJSONSchema(raw json.RawMessage) (speech.StructuredOutputSchema, error) {
    // Accept nested GenaiSchema trees and both camelCase/snake_case for additionalProperties
    var obj struct {
        Name                 string                        `json:"name"`
        Description          string                        `json:"description,omitempty"`
        Type                 string                        `json:"type"`
        AdditionalProperties bool                          `json:"additionalProperties,omitempty"`
        Properties           map[string]speech.GenaiSchema `json:"properties,omitempty"`
        Required             []string                      `json:"required,omitempty"`
    }
    if err := json.Unmarshal(raw, &obj); err != nil {
        return speech.StructuredOutputSchema{}, err
    }
    // Back-compat: accept snake_case additional_properties
    if !obj.AdditionalProperties {
        var m map[string]any
        if err := json.Unmarshal(raw, &m); err == nil {
            if v, ok := m["additional_properties"]; ok {
                if b, ok2 := v.(bool); ok2 { obj.AdditionalProperties = b }
            }
        }
    }
    if obj.Type == "" {
        obj.Type = "object"
    }
    return speech.StructuredOutputSchema{
        Name:                 obj.Name,
        Description:          obj.Description,
        Type:                 obj.Type,
        AdditionalProperties: obj.AdditionalProperties,
        Properties:           obj.Properties,
        Required:             obj.Required,
    }, nil
}

// mustJSON marshals an object and returns bytes, ignoring errors (best-effort)
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// isExpectedCacheError checks if a cache error is expected (e.g., content too small)
func isExpectedCacheError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for expected cache errors that don't need warning logs
	return strings.Contains(errStr, "too small") || 
		   strings.Contains(errStr, "min_total_token_count") ||
		   strings.Contains(errStr, "INVALID_ARGUMENT")
}

// TODO: Update file contains

/**
* This file contains:
* - WebSocket handler for audio processing and function extraction
* - Connection lifecycle management (auth, setup, teardown)
* - Audio session management (start/stop cycles)
* - Pipeline orchestration
* - Dynamic config updates via config watcher
* - Real-time transcript and function streaming
**/

type rawFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"` // ← raw blob
}

type rawFunctionConfig struct {
	Name         string           `json:"name,omitempty"`
	Description  string           `json:"description,omitempty"`
	UpdateMS     int              `json:"update_ms,omitempty"`
	ParsingGuide string           `json:"parsing_guide,omitempty"`
	Definitions  []rawFunctionDef `json:"definitions"` // ← use raw defs
	ParsingConfig speech.ParsingConfig `json:"parsing_config,omitempty"`
}

// Schema will be a JSON schema with a depth of 1
type rawStructuredConfig struct {
	Schema json.RawMessage `json:"schema,omitempty"`
	ParsingGuide string      `json:"parsing_guide,omitempty"`
	UpdateMS     int         `json:"update_ms,omitempty"`
	ParsingConfig speech.ParsingConfig `json:"parsing_config,omitempty"`
}

type configEnvelope struct {
	Type         string            `json:"type"`
	IsTest       bool              `json:"is_test,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Language     string            `json:"language,omitempty"`
	STT          STTConfig         `json:"stt"`
	Functions    *rawFunctionConfig `json:"function_config,omitempty"`
	Structured   *rawStructuredConfig `json:"structured_output_config,omitempty"`
	InputContext *InputContext     `json:"input_context,omitempty"`
	Redaction    *RedactionConfig `json:"redaction_config,omitempty"`
}

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
	SessionStatusClosed SessionStatus = "closed"
	SessionStatusIdle   SessionStatus = "idle" // silence
)

// AudioSessionState represents an audio start/stop cycle, a database session, or a pipeline run (spin up / tare down). These are the same thing
type AudioSessionState struct {
	ID            pgtype.UUID
	Status        SessionStatus
	CreatedAt     time.Time
	ClosedAt      *time.Time
	LastActivity  time.Time
	ConfigHistory []domain_configwatcher.TrackedConfig
	FlushedToDB   bool
}

// TODO: on flush after audio end, mark all sessions as closed (throw errors if any are still active while iterating through)

// Helper: marks audio session status as closed
func (s *AudioSessionState) MarkAsClosed() {
	now := time.Now()
	oldStatus := s.Status
	s.Status = SessionStatusClosed
	s.ClosedAt = &now
	s.LastActivity = now
	logger.ServiceDebugf("WS", "Session %s status changed from %s to %s at %v", s.ID.String(), oldStatus, s.Status, now)
}

// Helper: updates last activity time
func (s *AudioSessionState) UpdateActivity() {
	oldActivity := s.LastActivity
	s.LastActivity = time.Now()
	logger.ServiceDebugf("WS", "Session %s activity updated from %v to %v", s.ID.String(), oldActivity, s.LastActivity)
}

// Helper: adds config change to history
func (s *AudioSessionState) AddConfigChange(config domain_configwatcher.TrackedConfig) {
	s.ConfigHistory = append(s.ConfigHistory, config)
	s.UpdateActivity()
	logger.ServiceDebugf("WS", "Session %s config change added, total config changes: %d", s.ID.String(), len(s.ConfigHistory))
}

// Live session is a bundle of per-session components (ring buffer, silence service, etc.)
type liveSession struct {
	id         pgtype.UUID
	audioIn    chan speech.AudioChunk
	audioSink  *AudioSinkAdapter
	silenceSvc *silence_app.Service
    pipeline   *pipeline.Pipeline
    sPipeline  *pipeline.StructuredPipeline
	usageAcc   *app_usage.UsageAccumulator
	ringBuffer *audio.RingBuffer
	outTr      <-chan speech.Transcript
	outFn      <-chan []speech.FunctionCall
	outDr      <-chan speech.FunctionCall
    outStruct <-chan speech.StructuredOutputUpdate
	// accumulated latest structured object across deltas for final persistence
	sessionManager domain_session.Manager
	
	// Audio session timing
	audioStartTime time.Time
	audioEndTime   time.Time
	
	// CPU tracking timing
	cpuStartTime time.Time

	closeOnce  sync.Once
}

// Helper: stops the live session
func (s *liveSession) stop(ctx context.Context) {
	s.closeOnce.Do(func() {
		logger.ServiceDebugf("WS", "Session stop called for session %s", s.id.String())
		
		// Record audio session duration
		s.audioEndTime = time.Now()
		logger.ServiceDebugf("WS", "Recording audio session duration for session %s", s.id.String())
		if !s.audioStartTime.IsZero() {
			totalAudioDuration := s.audioEndTime.Sub(s.audioStartTime).Seconds()
			logger.ServiceDebugf("WS", "Calculating audio duration: start=%v, end=%v, duration=%.3f seconds", 
				s.audioStartTime, s.audioEndTime, totalAudioDuration)
			
			// Add total audio session duration to usage accumulator
			if s.usageAcc != nil && totalAudioDuration > 0 {
				logger.ServiceDebugf("WS", "Adding audio duration to usage accumulator: %.3f seconds", totalAudioDuration)
				s.usageAcc.AddSTT(totalAudioDuration, "session_total")
				logger.ServiceDebugf("WS", "Total audio session duration: %.3f seconds (start: %v, end: %v)", 
					totalAudioDuration, s.audioStartTime, s.audioEndTime)
			}
		} else {
			logger.Warnf("⚠️ [WS] Audio start time is zero - cannot calculate audio duration")
		}
		
		// Calculate and record CPU active time for the entire audio session
		logger.ServiceDebugf("WS", "Recording CPU time for session %s", s.id.String())
		if s.usageAcc != nil && !s.cpuStartTime.IsZero() {
			cpuDuration := time.Since(s.cpuStartTime)
			logger.ServiceDebugf("WS", "Calculating CPU duration: start=%v, end=%v, duration=%.3f seconds", 
				s.cpuStartTime, s.audioEndTime, cpuDuration.Seconds())
			logger.ServiceDebugf("WS", "Adding CPU duration to usage accumulator: %.3f seconds", cpuDuration.Seconds())
			s.usageAcc.AddCPUActiveTime(cpuDuration)
			logger.ServiceDebugf("WS", "Total CPU active time: %.3f seconds (start: %v, end: %v)", 
				cpuDuration.Seconds(), s.cpuStartTime, s.audioEndTime)
			
			// Set CPU idle time to 0 as requested
			logger.ServiceDebugf("WS", "Setting CPU idle time to 0 for session end")
			s.usageAcc.SetCPUIdleToZero()
			logger.ServiceDebugf("WS", "CPU idle time set to 0 for session end")
		} else if s.usageAcc == nil {
			logger.Warnf("⚠️ [WS] Usage accumulator is nil - cannot record CPU time")
		} else if s.cpuStartTime.IsZero() {
			logger.Warnf("⚠️ [WS] CPU start time is zero - cannot calculate CPU duration")
		}
		
		// 1. stop services in reverse order of start
		logger.ServiceDebugf("SESSION", "Stopping silence service")
		if s.silenceSvc != nil {
			s.silenceSvc.Stop(ctx)
		}
		// 2. close audio channel so STT / pipeline finish gracefully
		logger.ServiceDebugf("SESSION", "Closing audio channels")
		if s.audioSink != nil {
			s.audioSink.Close()
		}
		if s.audioIn != nil {
			logger.ServiceDebugf("SESSION", "Closing audioIn channel")
			close(s.audioIn)
		}
		// 3. usage accumulator
		logger.ServiceDebugf("SESSION", "Stopping usage accumulator")
		if s.usageAcc != nil {
			s.usageAcc.Stop(ctx)
		}

		// TODO: confirm that function call session correctly linked the function schema provided to the session (if already exists, link existing)
		// 4. persist aggregates
		logger.ServiceDebugf("SESSION", "Storing session data")
		if s.pipeline != nil {
			s.pipeline.StoreSessionData(ctx)
		}
		
		// 5. persist structured output
		if s.sPipeline != nil {
			s.sPipeline.StoreSessionData(ctx)
		}

		// 6. Close the database session
		logger.ServiceDebugf("WS", "Closing database session")
		if s.sessionManager != nil {
			if err := s.sessionManager.CloseSession(ctx, domain_session.DBSessionID(s.id)); err != nil {
				logger.Errorf("❌ [WS] Failed to close session %s: %v", s.id.String(), err)
			} else {
				logger.ServiceDebugf("WS", "Successfully closed session %s", s.id.String())
			}
		}	
		
		// 7. Mark session as closed in connection state
		// Note: This is handled in the audio_stop case above
		
		logger.ServiceDebugf("WS", "Session stop completed for session %s", s.id.String())
	})		
}



// Connection state for this WebSocket connection, responsibility and orchestration of the connection lifecycle belongs to the web socket handler. This may need to be refactored in the future to be more granular with specific control belonging to a particular component, but for now this is a fine starting point I think.
type ConnectionState struct {
	WSSessionID      string                             // The websocket session ID that is issued back to the client on successful auth, upgrade, and connection.
	Principal        domain_auth.Principal              // the authenticated principal
	ActiveSessionID  *pgtype.UUID                       // an active audio_start -> audo_stop cycle / pipeline / DB session
	Sessions         map[string]*AudioSessionState      // map of all sessions for this connection.
	ConfigWatcher    domain_configwatcher.ConfigWatcher // per connection config watcher
	UsageAccumulator *app_usage.UsageAccumulator        // per connection usage accumulator
	mu               sync.RWMutex                       // concurrency, keep the session map safe

	// Mode for determining LLM usage (functions or structured output)
	LLMMode string // "functions" or "structured" or "none"

    // Staged function-config when updates arrive before an active session exists
    PendingFuncCfg   *speech.FunctionConfig
    // Staged structured-config when updates arrive before an active session exists
    PendingStructuredCfg *StructuredOutputConfig
}

// Keep the existing Handler struct unchanged for now
type Handler struct {
	deps          pipeline.Deps // Deps is shared between all types of pipelines
	pricing       domain_usage.Pricing
	upg           websocket.Upgrader
	queries       *db.Queries
	configWatcher domain_configwatcher.ConfigWatcher
	sessionManager domain_session.Manager
	authService domain_auth.AuthService
	connectionService *connection.Service
	state         ConnectionState
	sess          *liveSession
	connID       domconn.ConnectionID // current connection ID for this WS
}

// NewHandler creates a new Handler with the given dependencies
func NewHandler(d pipeline.Deps, p domain_usage.Pricing, upg websocket.Upgrader, q *db.Queries, sm domain_session.Manager, authService domain_auth.AuthService, connService *connection.Service) *Handler {
	return &Handler{
		deps:          d,
		pricing:       p,
		upg:           upg,
		queries:       q,
		sessionManager: sm,
		authService: authService,
		connectionService: connService,
		configWatcher: infra_configwatcher.New(q, pgtype.UUID{Bytes: [16]byte{}}),
		state: ConnectionState{},
	}
}

// ServeHTTP handles the WebSocket connection and orchestrates the connection lifecycle.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract token from subprotocols before upgrading
	offered := websocket.Subprotocols(r)

	logger.ServiceDebugf("WS", "subprotocols offered=%v", offered)

	var hasProto bool
	var tokenStr string

	for _, p := range offered {
		if p == "schma.ws.v1" { hasProto = true }

		if strings.HasPrefix(p, "auth.") {
			tokenStr = strings.TrimPrefix(p, "auth.")
		}
	}

	logger.ServiceDebugf("WS", "subprotocol=%s", hasProto)
	logger.ServiceDebugf("WS", "tokenStr=%s", tokenStr)

	if !hasProto || tokenStr == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	
		return
	}

	// Verify JWT
	claims, err := pkg_auth.ParseWsJWT(
		tokenStr,
		os.Getenv("JWT_ISSUER"),
	)

	logger.ServiceDebugf("WS", "🔐 [WS] ServeHTTP: claims=%+v, err=%v", claims, err)

	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	appId := claims.Subject // tenant/app id
	sid := claims.Sid // ws_session_id (server-generated during mint)


	// Load WebSocket configuration for long session support
	wsConfig := getWSConfig()
	logger.ServiceDebugf("WS", "WebSocket config: session_timeout=%v, read_timeout=%v, ping_interval=%v, ping_timeout=%v",
		wsConfig.SessionTimeout, wsConfig.ReadTimeout, wsConfig.PingInterval, wsConfig.PingTimeout)

	// Derived context for the lifetime of this handler
	// Configurable timeout to support super long sessions (up to 2 hours)
	ctx, cancel := context.WithTimeout(r.Context(), wsConfig.SessionTimeout)
	defer cancel()

	// lookup principal
	principal, err := http_middleware.PrincipalFromJWT(ctx, appId, h.authService)
	if err != nil {
		 http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
	}

	logger.ServiceDebugf("WS", "principal=%+v", principal)

	// Upgrade (echo back accepted protocol)
	// 1.2 Upgrade the websocket
	
	// let Gorilla set the sub protocol
	conn, err := h.upg.Upgrade(w, r, nil)
	
	
	if err != nil {
		logger.Errorf("❌ [WS] upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	
	logger.ServiceDebugf("WS", "✅ upgraded; subprotocol=%+v", conn.Subprotocol())

	// Create connection in connection service (initially with mode 'none')
	if h.connectionService != nil {
		ua := r.UserAgent()
		remote := r.RemoteAddr
		created, err := h.connectionService.CreateConnection(ctx, sid, principal, remote, ua, offered, domconn.LLMMode("none"))
		if err != nil {
			logger.Errorf("❌ [WS] Failed to register connection: %v", err)
			return
		}
		h.connID = created.ID
		defer func() {
			_ = h.connectionService.CloseConnection(context.Background(), h.connID, "handler teardown")
		}()
	}

	readLimit := int64(512 * 1024)
	conn.SetReadLimit(readLimit) // 512KB
	// Configurable read deadline for long sessions
	_ = conn.SetReadDeadline(time.Now().Add(wsConfig.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsConfig.ReadTimeout))
	})

	// send ACK immediately
	ack := AckMessage{Type: "ack", SessionID: sid}
	if err := conn.WriteJSON(ack); err != nil {
		logger.Errorf("❌ [WS] write ack failed: %v", err)
		return
	}
	
	done := make(chan struct{})
	go func() {
	// Configurable ping interval for long sessions
	t := time.NewTicker(wsConfig.PingInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(wsConfig.PingTimeout)); err != nil {
			close(done); return
		}
		case <-ctx.Done():
		close(done); return
		}
	}
	}()
	defer func() { <-done }() // wait best-effort before returning

	logger.ServiceDebugf("WS", "ServeHTTP - after ticker invoc")
	
	// Set handler state (use SID from JWT, not from client)
	prevMode := h.state.LLMMode
	prevPendingFn := h.state.PendingFuncCfg
	prevPendingStructured := h.state.PendingStructuredCfg

	
	h.state = ConnectionState{
		WSSessionID:   sid,
		Principal:     principal,
		Sessions:      make(map[string]*AudioSessionState),
		ConfigWatcher: h.configWatcher,
		mu:            sync.RWMutex{},
		LLMMode:       prevMode,
		PendingFuncCfg: prevPendingFn,
		PendingStructuredCfg: prevPendingStructured,
	}


	// 1.3 Initial Config Message: read first JSON frame  → ConfigMessage
	t, payload, err := conn.ReadMessage()
	logger.ServiceDebugf("WS", "ServeHTTP - post read message: type=%d payload=%s", t, string(payload))
	if err != nil {
		return
	}

	var cfgEnv configEnvelope
	if err := json.Unmarshal(payload, &cfgEnv); err != nil {
		logger.Infof("ServeHTTP - First-frame JSON unmarshal failed: %v\nRaw:\n%s\n", err, payload)
		// Log raw payload for debugging
		logger.Errorf("❌ [WS] First-frame JSON unmarshal failed: %v\nRaw:\n%s\n", err, payload)
		writeErr(conn, "config malformed")
		return
	}

	if cfgEnv.Type != "config" {
		logger.Errorf("❌ [WS] First frame must be type=config, received: %s", cfgEnv.Type)
		writeErr(conn, "first frame must be type=config")
		return
	}

	if cfgEnv.SessionID != "" && cfgEnv.SessionID != sid {
		writeErr(conn, "session mismatch")
		return
	}

    // 1.5 Check for funcs or structured output config
    hasFns := cfgEnv.Functions != nil && len(cfgEnv.Functions.Definitions) > 0
    hasStruct := cfgEnv.Structured != nil && len(cfgEnv.Structured.Schema) > 0
	hasTest := cfgEnv.IsTest || !cfgEnv.IsTest // default to false if not present

    // Log structured parsing config only when structured config is present
    if hasStruct {
        if pcBytes, err := json.MarshalIndent(cfgEnv.Structured.ParsingConfig, "", "  "); err == nil {
            logger.Infof("Structured ParsingConfig:\n%s", string(pcBytes))
        }
    }
    
    logger.ServiceDebugf("WS", "Initial config envelope: hasFns=%v hasStruct=%v hasTest=%v lang=%q stt.provider=%q",
        hasFns, hasStruct, hasTest, cfgEnv.Language, cfgEnv.STT.Provider)
    // Debug STT config including diarization
    logger.ServiceDebugf("WS", "STT config: sample_hz=%d encoding=%q diarization={enable=%v min=%d max=%d}",
        cfgEnv.STT.SampleHertz,
        cfgEnv.STT.Encoding,
        cfgEnv.STT.Diarization.EnableSpeakerDiarization,
        cfgEnv.STT.Diarization.MinSpeakerCount,
        cfgEnv.STT.Diarization.MaxSpeakerCount,
    )
	
    if hasStruct {
        logger.ServiceDebugf("WS", "Structured config present: schema_bytes=%d update_ms=%d guide_len=%d, strategy=%q",
            len(cfgEnv.Structured.Schema), cfgEnv.Structured.UpdateMS, len(cfgEnv.Structured.ParsingGuide), cfgEnv.Structured.ParsingConfig.ParsingStrategy)
        if pcBytes, err := json.MarshalIndent(cfgEnv.Structured.ParsingConfig, "", "  "); err == nil {
            logger.ServiceDebugf("WS", "Structured ParsingConfig:\n%s", string(pcBytes))
        }
        // Explicitly log boolean flags that may be omitted in JSON due to omitempty
        logger.ServiceDebugf("WS", "Structured policy: transcript_mode=%q window_token_size=%d tail_sentences=%d apply_previous_output=%v",
            cfgEnv.Structured.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode,
            cfgEnv.Structured.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize,
            cfgEnv.Structured.ParsingConfig.TranscriptInclusionPolicy.TailSentences,
            cfgEnv.Structured.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode,
        )
    } else if hasFns {
        logger.ServiceDebugf("WS", "Function config present: defs=%d update_ms=%d guide_len=%d",
            len(cfgEnv.Functions.Definitions), cfgEnv.Functions.UpdateMS, len(cfgEnv.Functions.ParsingGuide))
        if pcBytes, err := json.MarshalIndent(cfgEnv.Functions.ParsingConfig, "", "  "); err == nil {
            logger.ServiceDebugf("WS", "Functions ParsingConfig:\n%s", string(pcBytes))
        }
        logger.ServiceDebugf("WS", "Functions policy: transcript_mode=%q window_token_size=%d tail_sentences=%d apply_previous_output=%v",
            cfgEnv.Functions.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode,
            cfgEnv.Functions.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize,
            cfgEnv.Functions.ParsingConfig.TranscriptInclusionPolicy.TailSentences,
            	cfgEnv.Functions.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode,
        )
    }

	if hasFns && hasStruct {
		_ = conn.WriteJSON(ErrorMessage{Type: "error", Err: "config cannot have both functions and structured output"})
		return
	}

	// Update connection LLM mode based on initial config
	if h.connectionService != nil {
		var mode domconn.LLMMode
		if hasFns {
			mode = domconn.LLMMode("functions")
		} else if hasStruct {
			mode = domconn.LLMMode("structured")
		} else {
			mode = domconn.LLMMode("none")
		}
		_ = h.connectionService.UpdateConnectionLLMMode(h.connID, mode)
	}

	// Warn if no LLM capabilities are configured
	if !hasFns && !hasStruct {
		logger.Warnf("⚠️ [WS] Initial config has no functions or structured output - LLM capabilities disabled")
	}


	var cfgMsg ConfigMessage
	// 1.5.1 Setup LLM in function mode
    if hasFns {
		h.state.LLMMode = "functions"

        parsedCfg, err := h.parseConfigEnvToMsg(cfgEnv, h.state.LLMMode)
		if err != nil {
			writeErr(conn, err.Error())
			return
		}
		
		cfgMsg = parsedCfg


		if err := h.setupLLMForFunctions(ctx, conn, cfgMsg); err != nil {
			writeErr(conn, err.Error())
			return
		}

	// 1.5.2 Setup LLM in structured output mode 
    } else if hasStruct {
		h.state.LLMMode = "structured"

        parsedCfg, err := h.parseConfigEnvToMsg(cfgEnv, h.state.LLMMode)
		if err != nil {
			writeErr(conn, err.Error())
			return
		}
		
		cfgMsg = parsedCfg	

		if err := h.setupLLMForStructured(ctx, conn, cfgMsg); err != nil {
			writeErr(conn, err.Error())
			return
		}

	// TODO: introduce pure STT mode, enhanced text mode, or enhanced markdown mode (single instruction)
	// 1.5.3 bypass LLM setup
	} else {
		h.state.LLMMode = "none"
		logger.ServiceDebugf("WS", "No LLM mode configured - running in STT-only mode")
		
		// Create a minimal config message for STT-only mode
		cfgMsg = ConfigMessage{
			Type:        "config",
			IsTest:      cfgEnv.IsTest,
			WSSessionID: cfgEnv.SessionID,
			Language:    cfgEnv.Language,
			STT:         cfgEnv.STT,
		}
	}

	logger.ServiceDebugf("WS", "Config message: %+v", cfgMsg)

	// 1.6 Init the connection state chans and defer for cleanup
    writerClosed := make(chan struct{})

	// 2 Per-session setup

	// 2.1 Per-session components (ring buffer, silence service, etc.) are now
	// created inside the audio_start branch together with the liveSession
	// bundle.  The defer below simply makes sure any still-running session
	// is closed when the HTTP handler returns.  This is a bit of a hack, but
	// it's a good starting point for now.
	defer func() {
		if h.sess != nil {
			logger.ServiceDebugf("WS", "Stopping session %s in defer cleanup", h.sess.id.String())
			h.sess.stop(ctx)
			h.sess = nil
		}
		
		// Mark session as closed in Sessions map and clear ActiveSessionID
		if h.state.ActiveSessionID != nil {
			if session, exists := h.state.Sessions[h.state.ActiveSessionID.String()]; exists {
				session.MarkAsClosed()
				h.state.Sessions[h.state.ActiveSessionID.String()] = session
				logger.ServiceDebugf("WS", "Marked session %s as closed in defer cleanup", h.state.ActiveSessionID.String())
			} else {
				logger.Warnf("⚠️ [WS] Active session %s not found in Sessions map during defer cleanup", h.state.ActiveSessionID.String())
			}
			h.state.ActiveSessionID = nil
			logger.ServiceDebugf("WS", "Cleared active session ID in defer cleanup")
		} else {
			logger.ServiceDebugf("WS", "No active session ID to clear during defer cleanup")
		}

		if h.state.UsageAccumulator != nil {
			h.state.UsageAccumulator.Stop(ctx)
			h.state.UsageAccumulator = nil
		}
	}()

    // 3. ack back to client
    _ = conn.WriteJSON(AckMessage{Type: "ack", SessionID: cfgMsg.WSSessionID}) // use our ws session ID which is created for the lifecycle of the connection.
    logger.ServiceDebugf("WS", "Ack sent (ws_session_id=%s, mode=%s)", cfgMsg.WSSessionID, h.state.LLMMode)

	// 4. reader loop: binary frames → audioIn (control)
	readerCtx, readerCancel := context.WithCancel(ctx)
	defer readerCancel()
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("❌ [WS] PANIC in reader loop: %v", r)
				// Send error message to client
				_ = conn.WriteJSON(ErrorMessage{
					Type: "error",
					Err:  "Internal server error - session terminated",
				})
				// Close connection gracefully
				conn.Close()
			}
		}()
		
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				// Check if this is a timeout error vs other errors
				isTimeout := strings.Contains(err.Error(), "timeout") || 
							strings.Contains(err.Error(), "i/o timeout") ||
							strings.Contains(err.Error(), "deadline exceeded")
				
				if isTimeout {
					logger.ServiceDebugf("WS", "Reader: websocket timeout after %v (session duration: %v)", 
						wsConfig.ReadTimeout, time.Since(h.sess.audioStartTime))
				} else {
					logger.Errorf("📥 [WS] Reader: websocket read error: %v (closing audioIn)", err)
				}
				
				// Send connection close message to client before closing
				var message, status string
				if isTimeout {
					message = "Connection timed out - session can be resumed"
					status = "timeout"
				} else {
					message = "Connection closed due to error"
					status = "error"
				}
				closeMsg := ConnectionCloseMessage{
					Type:    "connection_close",
					Message: message,
					Status:  status,
				}
				if writeErr := conn.WriteJSON(closeMsg); writeErr != nil {
					logger.Errorf("📤 [WS] Failed to send connection close message: %v", writeErr)
				}
				
				// Stop session and cleanup gracefully
				if h.sess != nil {
					logger.ServiceDebugf("WS", "Stopping session %s due to error/timeout", h.sess.id.String())
					h.sess.stop(ctx)
					h.sess = nil
				}
				
				// Mark session as closed in Sessions map and clear ActiveSessionID
				if h.state.ActiveSessionID != nil {
					if session, exists := h.state.Sessions[h.state.ActiveSessionID.String()]; exists {
						session.MarkAsClosed()
						h.state.Sessions[h.state.ActiveSessionID.String()] = session
						logger.ServiceDebugf("WS", "Marked session %s as closed in defer cleanup", h.state.ActiveSessionID.String())
					} else {
						logger.Warnf("⚠️ [WS] Active session %s not found in Sessions map during defer cleanup", h.state.ActiveSessionID.String())
					}
					h.state.ActiveSessionID = nil
					logger.ServiceDebugf("WS", "Cleared active session ID in defer cleanup")
				} else {
					logger.ServiceDebugf("WS", "No active session ID to clear during defer cleanup")
				}
				
				// Cancel reader context to stop any ongoing operations
				readerCancel()
				// Close connection in service with reason
				if h.connectionService != nil && h.connID != "" {
					_ = h.connectionService.CloseConnection(context.Background(), h.connID, "reader error/timeout")
				}
				return
			}
			switch mt {
			case websocket.BinaryMessage:
				chunk := speech.AudioChunk(data)
				if h.sess != nil {
					h.sess.audioIn <- chunk
					// DISABLED: Server-side silence detection - now using client-side detection
					// if h.sess.silenceSvc != nil {
					// 	h.sess.silenceSvc.OnAudioReceived()
					// }
				} // else ignore frames sent outside an active cycle

				// Write to per-session ring buffer for fallback replay
				if h.sess != nil && h.sess.ringBuffer != nil {
					h.sess.ringBuffer.Write(chunk)
				}

			case websocket.TextMessage:
				// Quick peek: decode only the "type" field so we can dispatch without
				// failing on nested objects
				var envelope struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					logger.Errorf("📥 [WS] Reader: failed to decode envelope: %v", err)
					continue
				}

				logger.ServiceDebugf("WS", "Reader: envelope type=%q", envelope.Type)

				// TOOD: this is horrible, refactor.
				switch envelope.Type {
				// silence status from client
				case "silence_status":
					logger.ServiceDebugf("WS", "Received silence status from client")

					// Break cases
					if h.sess == nil  {
						logger.Warnf("⚠️ [WS] No active session to handle silence status")
						break
					}

					if h.sess.silenceSvc == nil {
						logger.Warnf("⚠️ [WS] No silence service to handle silence status")
						break
					}

					// Unmarshal silence message
					var silenceMsg SilenceMessage

					if err := json.Unmarshal(data, &silenceMsg); err != nil {
						logger.Errorf("❌ [WS] silence_status unmarshal error: %v", err)
						continue
					}
					
					logger.ServiceDebugf("WS", "Received silence status from client: inSilence=%v, duration=%s", 
						silenceMsg.InSilence, silenceMsg.Duration)
					
					// Handle silence status
					h.sess.silenceSvc.OnClientSilenceStatus(silenceMsg.InSilence, silenceMsg.Duration)
					
					// Handle after-silence parsing strategy
					// Check if we're using after-silence strategy
					var shouldHandleSilence bool

					if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
						shouldHandleSilence = h.sess.pipeline.GetParsingStrategy() == "after-silence"
					} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
						shouldHandleSilence = h.sess.sPipeline.GetParsingStrategy() == "after-silence"
					}

					if !shouldHandleSilence {
						logger.ServiceDebugf("WS", "Skipping silence handling for 'after-silence' strategy: inSilence=%v, mode=%s", silenceMsg.InSilence, h.state.LLMMode)
						break
					}

					// handle silence logic			
					// Only force LLM call when entering silence (not on every keep-alive)
					if silenceMsg.InSilence {
						// Check if we're already in silence state (to prevent multiple calls)
						var alreadyInSilence bool
						if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
							alreadyInSilence = h.sess.pipeline.IsInSilence()
								logger.ServiceDebugf("WS", "Functions pipeline already in silence: %v", alreadyInSilence)
						} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
							alreadyInSilence = h.sess.sPipeline.IsInSilence()
							logger.ServiceDebugf("WS", "Structured pipeline already in silence: %v", alreadyInSilence)
						}
						
						if !alreadyInSilence {
							logger.ServiceDebugf("WS", "Entering silence with 'after-silence' strategy, forcing LLM call")
							
							// Get accumulated transcript for LLM from redaction buffer
							var transcriptForLLM string
							if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
								transcriptForLLM = h.sess.pipeline.GetAccumulatedTranscript()
							} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
								transcriptForLLM = h.sess.sPipeline.GetAccumulatedTranscript()
							}
							
							if transcriptForLLM != "" {
								// Force LLM call based on mode
								if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
									if err := h.sess.pipeline.ForceLLMCall(readerCtx, transcriptForLLM); err != nil {
										logger.Errorf("❌ [WS] Failed to force LLM call on silence (functions): %v", err)
									} else {
										logger.ServiceDebugf("WS", "Successfully forced LLM call on silence (functions)")
									}
								} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
									if err := h.sess.sPipeline.ForceLLMCall(readerCtx, transcriptForLLM); err != nil {
										logger.Errorf("❌ [WS] Failed to force LLM call on silence (structured): %v", err)
									} else {
										logger.ServiceDebugf("WS", "Successfully forced LLM call on silence (structured)")
									}
								}
							} else {
								logger.Warnf("⚠️ [WS] No accumulated transcript available for silence-based LLM call")
							}
						} else {
							logger.ServiceDebugf("WS", "Already in silence state, skipping LLM call")
						}
					} else {
						// Audio resumed - reset silence state to allow future silence detection
						logger.ServiceDebugf("WS", "Audio resumed, resetting silence state")
					}
					
					// Update silence state in pipelines AFTER processing
					if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
						h.sess.pipeline.SetSilenceState(silenceMsg.InSilence)
						logger.ServiceDebugf("WS", "Set functions pipeline silence state to %v", silenceMsg.InSilence)
					} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
						h.sess.sPipeline.SetSilenceState(silenceMsg.InSilence)
						logger.ServiceDebugf("WS", "Set structured pipeline silence state to %v", silenceMsg.InSilence)
					}
					
					
				

				// init live session stuff
                case "audio_start":
					logger.ServiceDebugf("WS", "Processing audio_start message")
					if h.sess != nil { // stop stray sessions
						logger.ServiceDebugf("WS", "Stopping existing session %s before starting new one", h.sess.id.String())
						h.sess.stop(ctx)
						h.sess = nil
					}
					
					// Mark any previous active session as closed
					if h.state.ActiveSessionID != nil {
						if session, exists := h.state.Sessions[h.state.ActiveSessionID.String()]; exists {
							session.MarkAsClosed()
							h.state.Sessions[h.state.ActiveSessionID.String()] = session
							logger.ServiceDebugf("WS", "Marked previous session %s as closed", h.state.ActiveSessionID.String())
						} else {
							logger.Warnf("⚠️ [WS] Previous active session %s not found in Sessions map", h.state.ActiveSessionID.String())
						}
					} else {
						logger.ServiceDebugf("WS", "No previous active session to mark as closed")
					}

					switch h.state.LLMMode {
					case "functions":
						if h.state.PendingFuncCfg != nil {
							if cfgMsg.Functions == nil {
								cfgMsg.Functions = &FunctionConfig{}
							}
							cfgMsg.Functions.UpdateMS = h.state.PendingFuncCfg.UpdateMs
							cfgMsg.Functions.ParsingGuide = h.state.PendingFuncCfg.ParsingGuide
							cfgMsg.Functions.Definitions = h.state.PendingFuncCfg.Declarations
							logger.ServiceDebugf("WS", "audio_start using pending staged config: defs=%d", len(cfgMsg.Functions.Definitions))
							h.state.PendingFuncCfg = nil
						  } else if h.state.ConfigWatcher != nil {
                            if tracked, err := h.state.ConfigWatcher.GetCurrentConfig(readerCtx); err == nil && tracked != nil && tracked.Config != nil {
                                if cfgMsg.Functions == nil {
                                    cfgMsg.Functions = &FunctionConfig{}
                                }
                                cfgMsg.Functions.UpdateMS = tracked.Config.UpdateMs
                                cfgMsg.Functions.ParsingGuide = tracked.Config.ParsingGuide
                                cfgMsg.Functions.Definitions = tracked.Config.Declarations
                                logger.ServiceDebugf("WS", "audio_start using tracked config: defs=%d", len(cfgMsg.Functions.Definitions))
                            }
                        }
					case "structured":
						if h.state.PendingStructuredCfg != nil {
							cfgMsg.Structured = &StructuredOutputConfig{
								Schema:       h.state.PendingStructuredCfg.Schema,
								ParsingGuide: h.state.PendingStructuredCfg.ParsingGuide,
								UpdateMS:     h.state.PendingStructuredCfg.UpdateMS,
							}
							h.state.PendingStructuredCfg = nil
							logger.ServiceDebugf("WS", "audio_start using pending structured config")
						}
					}
                  

					// build every per-cycle object
					audioCh := make(chan speech.AudioChunk, 64)
					sink := NewAudioSinkAdapter(audioCh)
					rbCfg := speech.DefaultRingBufferConfig()
					ringBuf := audio.NewRingBuffer(rbCfg)
					silSvc := silence_app.NewService(silence.DefaultConfig(), sink,
						NewEventNotifierAdapter(conn))

                    // Setup session with panic recovery
                    var dbID pgtype.UUID
                    var pl *pipeline.Pipeline
                    var sPl *pipeline.StructuredPipeline
                    var outTr <-chan speech.Transcript
                    var outFn <-chan []speech.FunctionCall
                    var outDr <-chan speech.FunctionCall
                    var outStruct <-chan speech.StructuredOutputUpdate
                    var err error
                    
                    func() {
                        defer func() {
                            if r := recover(); r != nil {
                                logger.Errorf("❌ [WS] PANIC in setupSession: %v", r)
                                err = fmt.Errorf("session setup failed: %v", r)
                            }
                        }()
                        
                        dbID, pl, sPl, outTr, outFn, outDr, outStruct, err = h.setupSession(ctx, principal, audioCh, cfgMsg)
                    }()
                    
                    if err != nil {
                        logger.Errorf("❌ [WS] Session setup failed: %v", err)
                        writeErr(conn, fmt.Sprintf("Session setup failed: %v", err))
                        continue
                    }

                    // Debug: session mode and channel readiness
                    logger.ServiceDebugf("WS", "session setup complete (mode=%s, pl_nil=%v, outTr_nil=%v, outFn_nil=%v, outDr_nil=%v, outStruct_nil=%v)",
                        h.state.LLMMode,
                        pl == nil,
                        outTr == nil,
                        outFn == nil,
                        outDr == nil,
                        outStruct == nil,
                    )
                    logger.ServiceDebugf("WS", "Session setup completed successfully for session %s", dbID.String())

					// TODO: appropriately update session state using attached methods
					// mark active session for config updates
					h.state.ActiveSessionID = &dbID
					sessionState := &AudioSessionState{ID: dbID, Status: "active", CreatedAt: time.Now(), LastActivity: time.Now()}
					h.state.Sessions[dbID.String()] = sessionState
					logger.ServiceDebugf("WS", "Created new active session %s with status %s", dbID.String(), sessionState.Status)
					// Inform connection service about active session
					if h.connectionService != nil && h.connID != "" {
						_ = h.connectionService.SetConnectionActiveSession(h.connID, dbID)
					}

                    // initialise config watcher for this session
                    					if h.state.ConfigWatcher != nil && h.state.LLMMode == "functions" {
						_ = h.state.ConfigWatcher.WatchSession(readerCtx, dbID, speech.FunctionConfig{
							ParsingConfig: cfgMsg.Functions.ParsingConfig,
							UpdateMs: cfgMsg.Functions.UpdateMS,
							Declarations:      cfgMsg.Functions.Definitions,
							ParsingGuide:      cfgMsg.Functions.ParsingGuide,
						})
					}

                    now := time.Now()
                    logger.ServiceDebugf("WS", "Creating liveSession with start time %v", now)
                    h.sess = &liveSession{
						id:         dbID,
						audioIn:    audioCh,
						audioSink:  sink,
						silenceSvc: silSvc,
                        pipeline:   pl,
                        sPipeline:  sPl,
                        usageAcc:   h.state.UsageAccumulator,
						ringBuffer: ringBuf,
						outTr:      outTr,
						outFn:      outFn,
						outDr:      outDr,
						outStruct:  outStruct,
						sessionManager: h.sessionManager,
						audioStartTime: now,
						cpuStartTime:   now,
					}
                    
                    logger.ServiceDebugf("WS", "Session started at %v (audio: %v, CPU: %v)", now, now, now)
                    logger.ServiceDebugf("WS", "LiveSession created successfully for session %s", dbID.String())
					
					// Reset CPU idle time to 0 for new session
					if h.state.UsageAccumulator != nil {
						logger.ServiceDebugf("WS", "Resetting CPU idle time to 0 for new session")
						h.state.UsageAccumulator.SetCPUIdleToZero()
						logger.ServiceDebugf("WS", "CPU idle time reset to 0 for new session")
					} else {
						logger.Warnf("⚠️ [WS] Usage accumulator is nil - cannot reset CPU idle time")
					}

					// launch silence service *after* everything is ready
					_ = silSvc.Start(ctx)

					// launch writer for this cycle (new channel per session)
					writerClosed = make(chan struct{})
					go h.writer(conn, h.sess, writerClosed) // must provide writer loop with it's own closed channel

					// TODO: move this into above switch statement -- redundant? Should we set this up before silence service though?
                    // Proactively prepare cache at session start with the latest config, per mode
					switch h.state.LLMMode {
					case "functions":
						if prep, ok := h.deps.LLM.(speech.CachePreparer); ok {
							if _, err := prep.PrepareCache(ctx, &speech.FunctionConfig{
								ParsingConfig: cfgMsg.Functions.ParsingConfig,
								UpdateMs: cfgMsg.Functions.UpdateMS,
								Declarations:      cfgMsg.Functions.Definitions,
								ParsingGuide:      cfgMsg.Functions.ParsingGuide,
							}); err != nil {
								logger.Warnf("⚠️ [WS] session-start cache prepare failed (functions): %v", err)
							}
						}
					case "structured":	
						// Prepare structured cache (same pattern as functions)
						if sprep, ok := h.deps.LLM.(interface {
							PrepareStructuredCache(ctx context.Context, cfg *speech.StructuredConfig) (speech.CacheKey, error)
						}); ok {
							if _, err := sprep.PrepareStructuredCache(ctx, &speech.StructuredConfig{
								Schema:       mustJSON(cfgMsg.Structured.Schema),
								ParsingGuide: cfgMsg.Structured.ParsingGuide,
								UpdateMS:     cfgMsg.Structured.UpdateMS,
							}); err != nil {
								logger.Warnf("⚠️ [WS] session-start cache prepare failed (structured): %v", err)
							}
						}
					}

				// stop audio and kill audio session.
                case "audio_stop":
					logger.ServiceDebugf("WS", "Processing audio_stop message")

                    if h.sess == nil {
                        logger.ServiceDebugf("WS", "No active session to stop")
                        break
                    }
					
					// Handle end-of-session parsing strategy
					var shouldForceLLM bool
					if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
						shouldForceLLM = h.sess.pipeline.GetParsingStrategy() == "end-of-session"
					} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
						shouldForceLLM = h.sess.sPipeline.GetParsingStrategy() == "end-of-session"
					}
					
					if shouldForceLLM {
						logger.ServiceDebugf("WS", "Audio stop with 'end-of-session' strategy, forcing final LLM call")
						
						// Get accumulated transcript for final LLM call
						var transcriptForLLM string
						if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
							transcriptForLLM = h.sess.pipeline.GetAccumulatedTranscript()
						} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
							transcriptForLLM = h.sess.sPipeline.GetAccumulatedTranscript()
						}
						
						if transcriptForLLM != "" {
							// Force final LLM call based on mode
							if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
								if err := h.sess.pipeline.ForceLLMCall(readerCtx, transcriptForLLM); err != nil {
									logger.Errorf("❌ [WS] Failed to force final LLM call (functions): %v", err)
								} else {
									logger.ServiceDebugf("WS", "Successfully forced final LLM call (functions)")
								}
							} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
								if err := h.sess.sPipeline.ForceLLMCall(readerCtx, transcriptForLLM); err != nil {
									logger.Errorf("❌ [WS] Failed to force final LLM call (structured): %v", err)
								} else {
									logger.ServiceDebugf("WS", "Successfully forced final LLM call (structured)")
								}
							}
							
							// Wait a bit for the LLM call to complete and results to be sent
							time.Sleep(100 * time.Millisecond)
						} else {
							logger.Warnf("⚠️ [WS] No accumulated transcript available for end-of-session LLM call")
						}
					}

                    // Note: StoreSessionData is called in liveSession.stop() - no need to call it here
                    logger.ServiceDebugf("WS", "Stopping session %s", h.sess.id.String())

                    h.sess.stop(ctx)
                    h.sess = nil
                    
                    // Mark session as closed in Sessions map
                    if h.state.ActiveSessionID != nil {
                        if session, exists := h.state.Sessions[h.state.ActiveSessionID.String()]; exists {
                            session.MarkAsClosed()
                            h.state.Sessions[h.state.ActiveSessionID.String()] = session
                            logger.ServiceDebugf("WS", "Marked session %s as closed in audio_stop", h.state.ActiveSessionID.String())
                        } else {
                            logger.Warnf("⚠️ [WS] Active session %s not found in Sessions map during audio_stop", h.state.ActiveSessionID.String())
                        }
                    } else {
                        logger.Warnf("⚠️ [WS] No active session ID to mark as closed during audio_stop")
                    }
                    
                    // Clear active session ID since session is stopped
                    h.state.ActiveSessionID = nil
                    logger.ServiceDebugf("WS", "Cleared active session ID")
                    // Inform connection service
                    if h.connectionService != nil && h.connID != "" {
						_ = h.connectionService.ClearConnectionActiveSession(h.connID)
					}

				// TODO: adjust dynamic_config_update message type to be 'dynamic_function_update'
                // Dynamic config update for hot-swapping function schemas
                case "dynamic_config_update":
                    cfgSpan := logger.NewSpan("ws.config_watcher", map[string]any{"ws_session": h.state.WSSessionID})
                    defer cfgSpan.Finish("ok")
                    cfgSpan.Debug("received dynamic config update", nil)
                    
                    // Add comprehensive logging for debugging
                    logger.ServiceDebugf("CONFIG", "Dynamic config update received - LLMMode=%s, ActiveSessionID=%v, sess=%v, pipeline=%v", 
                        h.state.LLMMode, 
                        h.state.ActiveSessionID != nil,
                        h.sess != nil,
                        h.sess != nil && h.sess.pipeline != nil)
                    
                    // Check if initial config has been processed
                    if h.state.LLMMode == "" {
                        logger.Warnf("⚠️ [CONFIG] Received dynamic config update before initial config - ignoring")
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{
                            Type:    "config_update_ack",
                            Success: false,
                            Message: "Initial config must be sent before dynamic updates",
                        })
                        continue
                    }
                    
                    // Check if we're in the right mode for function config updates
                    if h.state.LLMMode != "functions" {
                        logger.Warnf("⚠️ [CONFIG] Received function config update but not in functions mode (current mode: %s)", h.state.LLMMode)
                        var message string
                        if h.state.LLMMode == "none" {
                            message = "Cannot update function config in STT-only mode. Please send initial config with function definitions first."
                        } else {
                            message = fmt.Sprintf("Cannot update function config in %s mode", h.state.LLMMode)
                        }
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{
                            Type:    "config_update_ack",
                            Success: false,
                            Message: message,
                        })
                        continue
                    }
					// Parse dynamic config update (keep parameters as raw JSON first)
					var updateEnv struct {
						Type           string `json:"type"`
						FunctionConfig struct {
							Name         string           `json:"name,omitempty"`
							Description  string           `json:"description,omitempty"`
							UpdateMS     int              `json:"update_ms,omitempty"`
							ParsingGuide string           `json:"parsing_guide,omitempty"`
							Definitions  []rawFunctionDef `json:"definitions"`
							ParsingConfig speech.ParsingConfig   `json:"parsing_config,omitempty"`
						} `json:"function_config"`
					}
					if err := json.Unmarshal(data, &updateEnv); err != nil {
						logger.Errorf("❌ [WS] dynamic_config_update unmarshal error: %v", err)
						_ = conn.WriteJSON(ConfigUpdateAckMessage{
							Type:    "config_update_ack",
							Success: false,
							Message: "Failed to parse config update",
						})
						continue
					}

					// Log full parsing config for dynamic function update
					if pcBytes, err := json.MarshalIndent(updateEnv.FunctionConfig.ParsingConfig, "", "  "); err == nil {
						logger.ServiceDebugf("WS", "Dynamic Functions ParsingConfig:\n%s", string(pcBytes))
					}

					// Convert raw defs → domain definitions
					newDefs := make([]speech.FunctionDefinition, 0, len(updateEnv.FunctionConfig.Definitions))
					for _, rdef := range updateEnv.FunctionConfig.Definitions {
						params, err := mappers.ParamsFromSchema(rdef.Parameters)
						if err != nil {
							logger.Errorf("❌ [WS] Unable to parse parameters for %s: %v", rdef.Name, err)
							continue
						}
						newDefs = append(newDefs, speech.FunctionDefinition{
							Name:        rdef.Name,
							Description: rdef.Description,
							Parameters:  params,
						})
					}
                    cfgSpan.Debug("parsed update payload", map[string]any{"defs": len(newDefs), "guide_len": len(updateEnv.FunctionConfig.ParsingGuide)})

					// Parse and validate parsing strategy
					parsingStrategy := updateEnv.FunctionConfig.ParsingConfig.ParsingStrategy
					if parsingStrategy == "" {
						parsingStrategy = "auto"
					}
					
					// Validate parsing strategy
					validStrategies := map[string]bool{
						"auto": true, "update-ms": true, "after-silence": true, "end-of-session": true, "manual": true,
					}
					if !validStrategies[parsingStrategy] {
						logger.Errorf("❌ [WS] Invalid parsing strategy: %s", parsingStrategy)
						_ = conn.WriteJSON(ConfigUpdateAckMessage{
							Type:    "config_update_ack",
							Success: false,
							Message: "Invalid parsing strategy",
						})
						continue
					}

					// TODO: remove 'real-time' references... always assumed streaming unless hitting batch endpoint.
                    // 1. initial okay and ack if not
                    if h.state.ActiveSessionID == nil {
                        // Stage the config to be applied on next audio_start
                        h.state.PendingFuncCfg = &speech.FunctionConfig{
                            ParsingConfig: updateEnv.FunctionConfig.ParsingConfig,
                            UpdateMs: updateEnv.FunctionConfig.UpdateMS,
                            Declarations:      newDefs,
                            ParsingGuide:      updateEnv.FunctionConfig.ParsingGuide,
                        }
                        cfgSpan.Debug("staged for next session", map[string]any{"defs": len(newDefs)})
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{
                            Type:    "config_update_ack",
                            Success: true,
                            Message: "Config staged; will apply on next session",
                        })
                        continue
					}

                    // Hot-swap the pipeline configuration without stopping audio flow
                    if h.sess != nil && h.sess.pipeline != nil && h.state.ActiveSessionID != nil {
                        cfgSpan.Debug("hot-swapping pipeline config", map[string]any{"defs": len(newDefs)})
                        logger.ServiceDebugf("CONFIG", "Starting pipeline hot-swap for session %s", h.sess.id.String())
                        
                        // Handle end-of-session strategy: force LLM call BEFORE switching schemas
                        if parsingStrategy == "end-of-session" && h.sess != nil && h.sess.pipeline != nil {
                            logger.ServiceDebugf("CONFIG", "Schema switch in 'end-of-session' mode, forcing LLM call with OLD schema before swap")
                            
                            // Get accumulated transcript for LLM from redaction buffer
                            transcriptForLLM := h.sess.pipeline.GetAccumulatedTranscript()
                            
                            if transcriptForLLM != "" {
                                // Force LLM call with OLD schema BEFORE swapping
                                if err := h.sess.pipeline.ForceLLMCall(readerCtx, transcriptForLLM); err != nil {
                                    logger.Errorf("❌ [CONFIG] Failed to force LLM call on schema switch (end-of-session): %v", err)
                                } else {
                                    logger.ServiceDebugf("CONFIG", "Successfully forced LLM call with OLD schema before swap")
                                }
                            } else {
                                logger.Warnf("⚠️ [CONFIG] No accumulated transcript available for schema switch LLM call")
                            }
                        } else if parsingStrategy == "end-of-session" {
                            logger.ServiceDebugf("CONFIG", "Schema switch in 'end-of-session' mode, but no active session/pipeline - skipping LLM call")
                        }
                        
                        // Create new draft index for the updated function definitions
                        logger.ServiceDebugf("CONFIG", "Creating new draft index with %d definitions", len(newDefs))
                        idx, err := draft.New(
                            newDefs,
                            h.deps.FP,
                            env.ModelDir(),
                        )
                        if err != nil {
                            cfgSpan.Error("draft index build failed", map[string]any{"err": err})
                            logger.Errorf("❌ [CONFIG] Draft index creation failed: %v", err)
                            continue
                        }
                        logger.ServiceDebugf("CONFIG", "Draft index created successfully")
                        
                        // Update the pipeline's draft index
                        logger.ServiceDebugf("CONFIG", "Updating pipeline draft index")
                        h.sess.pipeline.UpdateDraftIndex(idx)
                        logger.ServiceDebugf("CONFIG", "Pipeline draft index updated")
                        
                        // Update the pipeline's function config and system prompt
                        logger.ServiceDebugf("CONFIG", "Updating pipeline function config")
                        newPrompt := speech.Prompt(prompts.BuildFunctionsSystemInstructionPrompt(updateEnv.FunctionConfig.ParsingGuide))
                        h.sess.pipeline.UpdateFunctionConfig(newPrompt, &speech.FunctionConfig{
                            ParsingConfig: updateEnv.FunctionConfig.ParsingConfig,
                            UpdateMs:      updateEnv.FunctionConfig.UpdateMS,
                            Declarations:  newDefs,
                            ParsingGuide:  updateEnv.FunctionConfig.ParsingGuide,
                        })
                        logger.ServiceDebugf("CONFIG", "Pipeline function config updated")
                        
                        // Reset previously aggregated function calls context under the new schema
                        logger.ServiceDebugf("CONFIG", "Clearing previous functions context")
                        h.sess.pipeline.ClearPrevFunctions()
                        logger.ServiceDebugf("CONFIG", "Previous functions context cleared")
                        
                        cfgSpan.Debug("pipeline config hot-swapped successfully", map[string]any{"defs": len(newDefs)})
                        logger.ServiceDebugf("CONFIG", "Pipeline hot-swap completed successfully")
                        
                        // Proactively prepare cache for new configuration (if supported)
                        logger.ServiceDebugf("CONFIG", "Preparing cache for new configuration")
                        if prep, ok := h.deps.LLM.(speech.CachePreparer); ok {
                            if _, err := prep.PrepareCache(ctx, &speech.FunctionConfig{
                                ParsingConfig: updateEnv.FunctionConfig.ParsingConfig,
                                UpdateMs: updateEnv.FunctionConfig.UpdateMS,
                                Declarations:      newDefs,
                                ParsingGuide:      updateEnv.FunctionConfig.ParsingGuide,
                            }); err != nil {
                                // Only log as warning if it's not an expected "too small" error
                                if isExpectedCacheError(err) {
                                    cfgSpan.Debug("cache preparation skipped (expected)", map[string]any{"err": err})
                                    logger.ServiceDebugf("CONFIG", "Cache preparation skipped (content too small): %v", err)
                                } else {
                                    cfgSpan.Warn("proactive cache prepare failed", map[string]any{"err": err})
                                    logger.Warnf("⚠️ [CONFIG] Cache preparation failed: %v", err)
                                }
                            } else {
                                logger.ServiceDebugf("CONFIG", "Cache prepared successfully")
                            }
                        } else {
                            logger.ServiceDebugf("CONFIG", "LLM does not support cache preparation")
                        }
                    } else {
                        logger.Warnf("⚠️ [CONFIG] Cannot hot-swap: session=%v, pipeline=%v, activeSessionID=%v", 
                            h.sess != nil, 
                            h.sess != nil && h.sess.pipeline != nil,
                            h.state.ActiveSessionID != nil)
                        
                        // Send error response to client
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{
                            Type:    "config_update_ack",
                            Success: false,
                            Message: "Cannot hot-swap: no active session or pipeline",
                        })
                        continue
                    }

					// 2. Update config in watcher (only if we have an active session)
					if h.state.ActiveSessionID != nil {
						logger.ServiceDebugf("CONFIG", "Updating config watcher with new checksum")
						newChecksum, err := h.state.ConfigWatcher.UpdateSessionConfig(readerCtx, speech.FunctionConfig{
							ParsingConfig:  updateEnv.FunctionConfig.ParsingConfig,
							UpdateMs:       updateEnv.FunctionConfig.UpdateMS,
							Declarations:   newDefs,
							ParsingGuide:   updateEnv.FunctionConfig.ParsingGuide,
						})

						if err != nil {
							cfgSpan.Error("config watcher update failed", map[string]any{"err": err})
							logger.Errorf("❌ [CONFIG] Config watcher update failed: %v", err)
							_ = conn.WriteJSON(ConfigUpdateAckMessage{
								Type:    "config_update_ack",
								Success: false,
								Message: "Failed to update config",
							})
							continue
						}
						cfgSpan.Debug("config watcher updated", map[string]any{"checksum": newChecksum})
						logger.ServiceDebugf("CONFIG", "Config watcher updated with checksum: %s", newChecksum)

						// 2.5. Immediately store and link the new schema to the session
						logger.ServiceDebugf("CONFIG", "Storing and linking new schema to session")
						if h.deps.FunctionSchemasRepo != nil && h.state.ActiveSessionID != nil {
							var dbSessionID pgtype.UUID
							if dbSessionID.Scan(h.state.ActiveSessionID.String()) == nil {
								// Create FunctionConfigWithoutContext for storage (excludes PrevContext)
								functionConfig := speech.FunctionConfigWithoutContext{
									Name:               updateEnv.FunctionConfig.Name,
									Description:        updateEnv.FunctionConfig.Description,
									ParsingConfig:      updateEnv.FunctionConfig.ParsingConfig,
									UpdateMs:  updateEnv.FunctionConfig.UpdateMS,
									Declarations:       newDefs,
									ParsingGuide:       updateEnv.FunctionConfig.ParsingGuide,
								}
								
								if schemaID, err := h.deps.FunctionSchemasRepo.StoreOrGetFunctionSchema(ctx, h.state.Principal.AppID, dbSessionID, functionConfig); err == nil {
									// Link the schema to the session
									if err := h.deps.FunctionSchemasRepo.LinkSchemaToSession(ctx, dbSessionID, schemaID); err != nil {
										logger.Warnf("⚠️ [CONFIG] Failed to link new schema to session: %v", err)
									} else {
										logger.ServiceDebugf("CONFIG", "Successfully linked new schema %s to session %s", schemaID.String(), dbSessionID.String())
									}
								} else {
									logger.Warnf("⚠️ [CONFIG] Failed to store new function schema: %v", err)
								}
							} else {
								logger.Warnf("⚠️ [CONFIG] Invalid session ID for schema linking")
							}
						} else {
							logger.Warnf("⚠️ [CONFIG] FunctionSchemasRepo is nil or no active session; schema linking skipped")
						}

						// 3. Track config change in session state
						logger.ServiceDebugf("CONFIG", "Tracking config change in session state")
						h.state.mu.Lock()
						if session, exists := h.state.Sessions[h.state.ActiveSessionID.String()]; exists {
							currentConfig, err := h.state.ConfigWatcher.GetCurrentConfig(readerCtx)
							if err == nil {
								session.AddConfigChange(*currentConfig)
								h.state.Sessions[h.state.ActiveSessionID.String()] = session
								logger.ServiceDebugf("CONFIG", "Session config change tracked successfully")
							} else {
								logger.Warnf("⚠️ [CONFIG] Failed to get current config for tracking: %v", err)
							}
						} else {
							logger.Warnf("⚠️ [CONFIG] Active session not found in state for tracking")
						}
						h.state.mu.Unlock()

						// 4. Success ack back
						logger.ServiceDebugf("CONFIG", "Sending success acknowledgment to client")
						_ = conn.WriteJSON(ConfigUpdateAckMessage{
							Type:    "config_update_ack",
							Success: true,
							Message: fmt.Sprintf("Config updated with checksum: %s", newChecksum),
						})
						// Let the span finish with an OK summary now that the update is applied
						cfgSpan.Finish("ok")
						logger.ServiceDebugf("CONFIG", "Config update process completed successfully")
					} else {
						// No active session, just acknowledge the config update
						logger.ServiceDebugf("CONFIG", "No active session, acknowledging config update without watcher update")
						_ = conn.WriteJSON(ConfigUpdateAckMessage{
							Type:    "config_update_ack",
							Success: true,
							Message: "Config update acknowledged (no active session)",
						})
						cfgSpan.Finish("ok")
					}

                case "dynamic_structured_update":
                    // Handle dynamic structured updates
                    
                    // Check if initial config has been processed
                    if h.state.LLMMode == "" {
                        logger.Warnf("⚠️ [CONFIG] Received dynamic structured update before initial config - ignoring")
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{
                            Type:    "config_update_ack",
                            Success: false,
                            Message: "Initial config must be sent before dynamic updates",
                        })
                        continue
                    }
                    
                    if h.state.LLMMode != "structured" {
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{Type: "config_update_ack", Success: false, Message: "not in structured mode"})
                        continue
                    }

                    var updateEnv struct {
                        Type               string                 `json:"type"`
                        StructuredConfig   rawStructuredConfig    `json:"structured_output_config"`
                        PreserveContext    bool                   `json:"preserve_context"`
                        Redaction          *RedactionConfig       `json:"redaction_config,omitempty"`
                    }
                    if err := json.Unmarshal(data, &updateEnv); err != nil {
                        logger.Errorf("❌ [WS] dynamic_structured_update unmarshal error: %v", err)
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{Type: "config_update_ack", Success: false, Message: "Failed to parse structured update"})
                        continue
                    }

                    // Log full parsing config for dynamic structured update
                    if pcBytes, err := json.MarshalIndent(updateEnv.StructuredConfig.ParsingConfig, "", "  "); err == nil {
                        logger.ServiceDebugf("WS", "Dynamic Structured ParsingConfig:\n%s", string(pcBytes))
                    }
                    logger.ServiceDebugf("WS", "Dynamic Structured policy: transcript_mode=%q window_token_size=%d tail_sentences=%d apply_previous_output=%v",
                        updateEnv.StructuredConfig.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode,
                        updateEnv.StructuredConfig.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize,
                        updateEnv.StructuredConfig.ParsingConfig.TranscriptInclusionPolicy.TailSentences,
                        updateEnv.StructuredConfig.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode,
                    )

                    logger.ServiceDebugf("WS", "dynamic_structured_update received: schema_bytes=%d update_ms=%d guide_len=%d active_session=%v",
                        len(updateEnv.StructuredConfig.Schema), updateEnv.StructuredConfig.UpdateMS, len(updateEnv.StructuredConfig.ParsingGuide), h.state.ActiveSessionID != nil)

                    // Parse raw JSON schema into domain struct
                    parsedSchema, err := parseStructuredJSONSchema(updateEnv.StructuredConfig.Schema)
                    if err != nil {
                        logger.Errorf("❌ [WS] failed to parse structured JSON schema: %v", err)
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{Type: "config_update_ack", Success: false, Message: "invalid structured schema"})
                        continue
                    }

                    // Stage if no active session; else apply live swap
                    if h.state.ActiveSessionID == nil {
                        h.state.PendingStructuredCfg = &StructuredOutputConfig{
                            Schema:       parsedSchema,
                            ParsingGuide: updateEnv.StructuredConfig.ParsingGuide,
                            UpdateMS:     updateEnv.StructuredConfig.UpdateMS,
                        }
                        logger.ServiceDebugf("WS", "Structured config staged for next session: schema_bytes=%d",
                            len(updateEnv.StructuredConfig.Schema))
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{Type: "config_update_ack", Success: true, Message: "Structured config staged; will apply on next session"})
                        continue
                    }
                    // Live swap structured config
                    if h.sess != nil && h.sess.sPipeline != nil {
                        logger.ServiceDebugf("CONFIG", "Hot-swapping structured config (preserve_context=%v)", updateEnv.PreserveContext)
                        newCfg := &speech.StructuredOutputConfig{
                            UpdateMs:        updateEnv.StructuredConfig.UpdateMS,
                            Schema:          parsedSchema,
                            ParsingGuide:    updateEnv.StructuredConfig.ParsingGuide,
                            ParsingConfig:   updateEnv.StructuredConfig.ParsingConfig,
                        }
                        h.sess.sPipeline.UpdateStructuredConfig(newCfg, updateEnv.PreserveContext)
                        // Update redaction toggle if provided
                        if updateEnv.Redaction != nil && h.sess.sPipeline != nil {
                            h.sess.sPipeline.SetDisablePHI(updateEnv.Redaction.DisablePHI)
                        }

                        // Proactively prepare structured cache if supported
                        if sprep, ok := h.deps.LLM.(interface {
                            PrepareStructuredCache(ctx context.Context, cfg *speech.StructuredConfig) (speech.CacheKey, error)
                        }); ok {
                            schemaBytes, _ := json.Marshal(parsedSchema)
                            _, _ = sprep.PrepareStructuredCache(readerCtx, &speech.StructuredConfig{
                                Schema:       schemaBytes,
                                ParsingGuide: newCfg.ParsingGuide,
                                UpdateMS:     newCfg.UpdateMs,
                            })
                        }

                        _ = conn.WriteJSON(ConfigUpdateAckMessage{Type: "config_update_ack", Success: true, Message: "Structured config updated"})
                    } else {
                        _ = conn.WriteJSON(ConfigUpdateAckMessage{Type: "config_update_ack", Success: false, Message: "No active structured pipeline to update"})
                    }

                case "final_ack":
					// client confirms session_end
					// TODO: implement

				// TODO: adjust client SDK to be able to send buffer_stats message and receive back stats... or maybe just have silence write message?
				case "buffer_stats":
					// Send ring buffer statistics to client
					var stats map[string]interface{}
					if h.sess != nil && h.sess.ringBuffer != nil && h.sess.silenceSvc != nil {
						stats = map[string]interface{}{
							"type":        "buffer_stats",
							"ring_buffer": h.sess.ringBuffer.Stats(),
							"silence": map[string]interface{}{
								"in_silence": h.sess.silenceSvc.IsInSilence(),
								"duration":   h.sess.silenceSvc.SilenceDuration().String(),
							},
						}
						} else {
							stats = map[string]interface{}{"type": "buffer_stats", "error": "no active session"}
						}

					logger.ServiceDebugf("WS", "buffer_stats received: %v", stats)
					_ = conn.WriteJSON(stats)

				case "parse_content_manually":
					// Manual generation trigger. Only acts when strategy is manual.
					if h.sess == nil {
						logger.Warnf("⚠️ [WS] manual parse requested but no active session")
						continue
					}
					if h.state.LLMMode == "functions" && h.sess.pipeline != nil {
						if h.sess.pipeline.GetParsingStrategy() != "manual" {
							logger.ServiceDebugf("WS", "Ignoring manual parse (functions): strategy=%s", h.sess.pipeline.GetParsingStrategy())
							continue
						}
						tr := h.sess.pipeline.GetAccumulatedTranscript()
						if tr != "" {
							if err := h.sess.pipeline.ForceLLMCall(readerCtx, tr); err != nil {
								logger.Errorf("❌ [WS] manual functions parse failed: %v", err)
							}
						}
					} else if h.state.LLMMode == "structured" && h.sess.sPipeline != nil {
						if h.sess.sPipeline.GetParsingStrategy() != "manual" {
							logger.ServiceDebugf("WS", "Ignoring manual parse (structured): strategy=%s", h.sess.sPipeline.GetParsingStrategy())
							continue
						}
						tr := h.sess.sPipeline.GetAccumulatedTranscript()
						if tr != "" {
							if err := h.sess.sPipeline.ForceLLMCall(readerCtx, tr); err != nil {
								logger.Errorf("❌ [WS] manual structured parse failed: %v", err)
							}
						}
					}
				}
			}
		}
	}()

	// Finalise
    logger.ServiceDebugf("WS", "Main: reader done - waiting for writer...")
	<-writerClosed // pipeline has fully flushed
	logger.ServiceDebugf("WS", "Main: writer closed")

	// End session with graceful timeout
	_ = conn.WriteJSON(struct{ Type string }{Type: "session_end"})
	logger.ServiceDebugf("WS", "Session ended gracefully")

	// Close connection record on normal end
	if h.connectionService != nil && h.connID != "" {
		_ = h.connectionService.CloseConnection(context.Background(), h.connID, "normal close")
	}

	// Wait for client acknowledgment with configurable timeout
	conn.SetReadDeadline(time.Now().Add(wsConfig.PingTimeout))
	logger.ServiceDebugf("WS", "Main: session ended gracefully")
}


// Writer loop in separate function. Provide the live session and done chan so we can explicitly signal start writer
func (h *Handler) writer(conn *websocket.Conn, sess *liveSession, done chan struct{}) {
	defer close(done)

    trC, fnC, drC := sess.outTr, sess.outFn, sess.outDr
    var soC <-chan speech.StructuredOutputUpdate
    if sess.outStruct != nil {
        soC = sess.outStruct
    }
    for trC != nil || fnC != nil || drC != nil || soC != nil {
		select {
		case tr, ok := <-trC:
			if !ok {
				
				// Notify silence service that STT connection is closed
				if sess.silenceSvc != nil {
					sess.silenceSvc.MarkSTTInactive()
				}
				trC = nil
			} else {

				// transcriptSendTime := time.Now()
				if err := conn.WriteJSON(TranscriptMessage{
					Type:       "transcript",
					Text:       tr.Text,
					Final:      tr.IsFinal,
					Confidence: tr.Confidence,
					Words:      tr.Words,
					// Diarization
					Turns:      tr.Turns,
					Channel:    tr.Channel,
					// new management for normalization and redaction purposes
					PhrasesDisplay: tr.PhrasesDisplay,
				}); err != nil {
					logger.Errorf("❌ [WS] Failed to write transcript: %v", err)
					return
				}
			}

		case draft, ok := <-drC:
			if !ok {
				drC = nil
			} else {
				logger.ServiceDebugf("WS", "Writer: draft→client name=%s similarity=%.2f args=%v",
					draft.Name, draft.SimilarityScore, draft.Args)

				// Track draft function in usage accumulator
				draftArgs := make(map[string]interface{})
				if draft.Args != nil {
					draftArgs = draft.Args
				}

				if h.state.UsageAccumulator != nil {
					h.state.UsageAccumulator.AddDraftFunction(draft.Name, draft.SimilarityScore, draftArgs)
				}

				if err := conn.WriteJSON(DraftFunctionMessage{
					Type:  "function_draft_extracted",
					Draft: draft,
				}); err != nil {
					logger.Errorf("❌ [WS] Failed to write draft function: %v", err)
					return
				}
			}
		
		case calls, ok := <-fnC:
			if !ok {
				fnC = nil // keep transcripts flowing if only funcs closed
			} else {

				logger.ServiceDebugf("WS", "Writer: %d function(s)", len(calls))

				if len(calls) > 0 {
					// Track redacted functions in usage accumulator (not the reconstructed ones sent to client)
					if h.state.UsageAccumulator != nil && h.sess != nil && h.sess.pipeline != nil {
						redactedCalls := h.sess.pipeline.GetLatestRedactedFunctionCalls()
						if len(redactedCalls) > 0 {
							// Get the latest redacted calls for metrics
							finalFunctions := make([]map[string]interface{}, len(redactedCalls))
							for i, call := range redactedCalls {
								finalFunctions[i] = map[string]interface{}{
									"name": call.Name,
									"args": call.Args,
								}
							}
							h.state.UsageAccumulator.AddFinalFunctions(finalFunctions)
							logger.ServiceDebugf("WS", "Tracked %d redacted function calls for metrics", len(redactedCalls))
							// Clear the latest batch to avoid double-counting
							h.sess.pipeline.ClearLatestRedactedFunctionCalls()
						}
					}

					_ = conn.WriteJSON(FunctionMessage{
						Type:      "functions",
						Functions: calls,
					})
				}
			}

        case su, ok := <-soC:
            if !ok {
                soC = nil
            } else {
                // Track redacted structured output in usage accumulator (not the reconstructed ones sent to client)
                if h.state.UsageAccumulator != nil && h.sess != nil && h.sess.sPipeline != nil {
                    redactedStructured := h.sess.sPipeline.GetLatestRedactedStructured()
                    if len(redactedStructured) > 0 {
                        // Get the latest redacted structured output for metrics
                        for _, redacted := range redactedStructured {
                            h.state.UsageAccumulator.AddStructuredOutput(redacted.Rev, redacted.Delta, redacted.Final)
                        }
                        logger.ServiceDebugf("WS", "Tracked %d redacted structured output updates for metrics", len(redactedStructured))
                        // Clear the latest batch to avoid double-counting
                        h.sess.sPipeline.ClearLatestRedactedStructured()
                    }
                }

                // Send to client
                msg := StructuredOutputMessage{Type: "structured_output", Rev: int64(su.Rev), Delta: su.Delta, Final: su.Final}
                
                logger.ServiceDebugf("WS", "Writer: Sending structured output to client: rev=%d, delta_keys=%v, final_keys=%v", 
                    su.Rev, getMapKeys(su.Delta), getMapKeys(su.Final))
                
                if err := conn.WriteJSON(msg); err != nil {
                    logger.Errorf("❌ [WS] Failed to write structured output: %v", err)
                    return
                }
            }
		}
	}
}

// -----------------------------------------------------------------------------
// helpers: writeErr, setupPipeline, TaredownPipeline
// -----------------------------------------------------------------------------
func writeErr(c *websocket.Conn, msg string) {
	_ = c.WriteJSON(ErrorMessage{Type: "error", Err: msg})
}

// getMapKeys returns the keys of a map as a slice of strings for logging
func getMapKeys(m map[string]any) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (h *Handler) parseConfigEnvToMsg(cfgEnv configEnvelope, mode string) (ConfigMessage, error) {
	if mode == "functions" {
		defs := make([]speech.FunctionDefinition, 0, len(cfgEnv.Functions.Definitions))
		for _, r := range cfgEnv.Functions.Definitions {
			params, err := mappers.ParamsFromSchema(r.Parameters)
			if err != nil {
				logger.Errorf("❌ [WS] Cannot parse parameters of %s: %v", r.Name, err)
				continue
			}
			defs = append(defs, speech.FunctionDefinition{
				Name:        r.Name,
				Description: r.Description,
				Parameters:  params, // ← now the slice that domain expects
			})
		}

		// Parse and validate parsing strategy
		parsingStrategy := cfgEnv.Functions.ParsingConfig.ParsingStrategy
		if parsingStrategy == "" {
			parsingStrategy = "auto" // Default to auto
		}
		
		// Validate parsing strategy
		validStrategies := map[string]bool{
			"auto": true, "update-ms": true, "after-silence": true, "end-of-session": true, "manual": true,
		}
		if !validStrategies[parsingStrategy] {
			return ConfigMessage{}, fmt.Errorf("invalid parsing strategy: %s", parsingStrategy)
		}

		cfgMsg := ConfigMessage{
			Type:        "config",
			IsTest:      cfgEnv.IsTest,
			WSSessionID: cfgEnv.SessionID,
			Language:    cfgEnv.Language,
			STT:         cfgEnv.STT,
			Functions: &FunctionConfig{
				Name:            cfgEnv.Functions.Name,
				Description:     cfgEnv.Functions.Description,
				UpdateMS:        cfgEnv.Functions.UpdateMS,
				ParsingGuide:    cfgEnv.Functions.ParsingGuide ,
				Definitions:     defs, // ← now the slice
				ParsingConfig:   cfgEnv.Functions.ParsingConfig,
			},
			InputContext: cfgEnv.InputContext,
			Redaction: cfgEnv.Redaction,
		}

		return cfgMsg, nil
    } else if mode == "structured" {
        // Parse structured output schema from raw JSON to domain model
        parsedSchema, err := parseStructuredJSONSchema(cfgEnv.Structured.Schema)
        if err != nil {
            return ConfigMessage{}, fmt.Errorf("invalid structured schema: %w", err)
        }
        
        // Parse and validate parsing strategy
        parsingStrategy := cfgEnv.Structured.ParsingConfig.ParsingStrategy
        if parsingStrategy == "" {
            parsingStrategy = "auto" // Default to auto
        }
        
        // Validate parsing strategy
        validStrategies := map[string]bool{
            "auto": true, "update-ms": true, "after-silence": true, "end-of-session": true, "manual": true,
        }
     
        if !validStrategies[parsingStrategy] {
            return ConfigMessage{}, fmt.Errorf("invalid parsing strategy: %s", parsingStrategy)
        }
        
        cfgMsg := ConfigMessage{
            Type:        "config",
            WSSessionID: cfgEnv.SessionID,
            Language:    cfgEnv.Language,
            STT:         cfgEnv.STT,
            Structured: &StructuredOutputConfig{
                Schema:          parsedSchema,
                ParsingGuide:    cfgEnv.Structured.ParsingGuide,
                UpdateMS:        cfgEnv.Structured.UpdateMS,
                ParsingConfig:   cfgEnv.Structured.ParsingConfig,
            },
            InputContext: cfgEnv.InputContext,
			Redaction: cfgEnv.Redaction,
        }
        return cfgMsg, nil
	} else {
		return ConfigMessage{}, fmt.Errorf("invalid mode: %s", mode)
	}

}

func (h *Handler) setupLLMForStructured(ctx context.Context, conn *websocket.Conn, cfgMsg ConfigMessage) error {
    logger.ServiceDebugf("WS", "Building Gemini session (structured)...")

    sess, err := llmgemini.NewSession(
        os.Getenv("GEMINI_API_KEY"),
        "gemini-2.5-flash",
    )
    if err != nil {
        writeErr(conn, "gemini init failed")
        return err
    }

    if cfgMsg.Structured != nil {
        // Configure session for schema-constrained JSON output
        guideLen := len(cfgMsg.Structured.ParsingGuide)
        logger.ServiceDebugf("WS", "Configuring structured session: guide_len=%d", guideLen)
        // Try v2 signature taking json.RawMessage; include structured system instructions
        if cs2, ok := interface{}(sess).(interface{ ConfigureStructuredOnce(schema json.RawMessage, systemGuide string) }); ok {
            b, _ := json.Marshal(cfgMsg.Structured.Schema)
            sys := prompts.BuildStructuredSystemInstructionPrompt(cfgMsg.Structured.ParsingGuide)
            logger.ServiceDebugf("WS", "Structured system prompt:\n%s", sys)
            cs2.ConfigureStructuredOnce(b, sys)
            logger.ServiceDebugf("WS", "Structured session configured (json.RawMessage)")
        } else if cs1, ok := interface{}(sess).(interface{ ConfigureStructuredOnce(schema speech.StructuredOutputSchema, guide string) }); ok {
            // Fallback to older signature
            sys := prompts.BuildStructuredSystemInstructionPrompt(cfgMsg.Structured.ParsingGuide)
            logger.ServiceDebugf("WS", "Structured system prompt:\n%s", sys)
            cs1.ConfigureStructuredOnce(cfgMsg.Structured.Schema, sys)
            logger.ServiceDebugf("WS", "Structured session configured (schema struct)")
        } else {
            logger.Warnf("⚠️ [WS] Gemini session does not support ConfigureStructuredOnce yet")
        }
    }

    if setter, ok := h.deps.LLM.(speech.SessionSetter); ok {
        setter.SetSession(sess)
        logger.ServiceDebugf("WS", "SetSession injected for structured mode")
    } else {
        logger.Warnf("⚠️ [WS] deps.LLM does not expose SetSession -- skipped")
    }

    logger.ServiceDebugf("WS", "Gemini ready (structured)")
    return nil
}


func (h *Handler) setupLLMForFunctions(ctx context.Context, conn *websocket.Conn, cfgMsg ConfigMessage) error {
	// 1.4 Connection-level LLM setup. TODO: in the future, function config may not be included (LLM may not be used? hmmm).
	logger.ServiceDebugf("WS", "Building Gemini session & tools...")
		
	sess, err := llmgemini.NewSession(
		os.Getenv("GEMINI_API_KEY"),
		"gemini-2.0-flash",
	)
	if err != nil {
		writeErr(conn, "gemini init failed")
		return err
	}

	if defs := cfgMsg.Functions.Definitions; len(defs) > 0 {
		logger.ServiceDebugf("WS", "Configuring Gemini session & tools...")
		sess.ConfigureOnce(defs, cfgMsg.Functions.ParsingGuide)
	}

	if setter, ok := h.deps.LLM.(speech.SessionSetter); ok {
		setter.SetSession(sess) // inject live session
	} else {
		logger.Warnf("⚠️ [WS] deps.LLM does not expose SetSession -- skipped")
	}

	logger.ServiceDebugf("WS", "Gemini ready (functions: %d)", len(cfgMsg.Functions.Definitions))
	return nil
}


func (h *Handler) setupSession(ctx context.Context, principal domain_auth.Principal, audioIn <-chan speech.AudioChunk, cfgMsg ConfigMessage) (
    pgtype.UUID,
    *pipeline.Pipeline,
    *pipeline.StructuredPipeline,
    <-chan speech.Transcript,
    <-chan []speech.FunctionCall,
    <-chan speech.FunctionCall,
    <-chan speech.StructuredOutputUpdate,
    error,
) {
	logger.ServiceDebugf("WS", "Setting up session... (LLMMode=%s)", h.state.LLMMode)

	// 1. Create database session using session manager
	sessionState, err := h.sessionManager.StartSession(ctx, domain_session.WSSessionID(h.state.WSSessionID), cfgMsg.IsTest, db.SessionKindEnumStream, principal)
    if err != nil {
        return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to create database session: %w", err)
    }
    dbSessionID := pgtype.UUID(sessionState.ID)

    // 2. Create pipeline(s) based on mode
    var pl *pipeline.Pipeline
    var sPl *pipeline.StructuredPipeline
    var outTr <-chan speech.Transcript
    var outFn <-chan []speech.FunctionCall
    var outDr <-chan speech.FunctionCall

	// TODO: refactor, switch on LLM mode and use two setupPipeline functions. 
    if h.state.LLMMode == "functions" {
        // Validate that we have function config
        if cfgMsg.Functions == nil {
            logger.Errorf("❌ [WS] Functions mode requested but no function config provided")
            return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("functions mode requires function config")
        }
        pl, err = h.setupPipeline(ctx, dbSessionID, principal, cfgMsg)
		if err != nil {
				return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to create pipeline: %w", err)
			}
        // Run functions pipeline
        outTr, outFn, outDr, err = pl.Run(ctx, audioIn)
        if err != nil {
            return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to run pipeline: %w", err)
        }
        // Start usage accumulator (defensive nil checks)
        usageAccumulator := pl.UsageAccumulator()
        if usageAccumulator == nil {
            logger.Warnf("⚠️ [WS] functions pipeline returned nil usage accumulator")
        } else {
            logger.ServiceDebugf("USAGE", "Starting usage accumulator for session %s", dbSessionID.String())
            usageAccumulator.Start(ctx)
            h.state.UsageAccumulator = usageAccumulator
        }
        return dbSessionID, pl, nil, outTr, outFn, outDr, nil, nil
    } else if h.state.LLMMode == "structured" {
        // Validate that we have structured config
        if cfgMsg.Structured == nil {
            logger.Errorf("❌ [WS] Structured mode requested but no structured config provided")
            return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("structured mode requires structured config")
        }
        
        // Structured mode
        // Set conservative default for structured output frequency to avoid 500 errors
        updateMS := cfgMsg.Structured.UpdateMS
    if updateMS <= 0 {
        updateMS = 3000 // Default to 3 seconds for structured output
        logger.ServiceDebugf("WS", "Using default UpdateMS=%d for structured output (client provided %d)", updateMS, cfgMsg.Structured.UpdateMS)
    }
    
    // Set default UpdateMS for "auto" strategy if not provided
    if cfgMsg.Structured.ParsingConfig.ParsingStrategy == "auto" && updateMS <= 0 {
        updateMS = 3000 // Default to 3 seconds for auto strategy
        logger.ServiceDebugf("WS", "Using default UpdateMS=%d for auto strategy (structured)", updateMS)
    }
    
    // Log model being used for structured output (helpful for debugging 500 errors)
    logger.ServiceDebugf("WS", "Structured output using model: gemini-2.0-flash (stable structured output with 4096 token caching)")
    
    sCfg := pipeline.ConfigStructured{
        DBSessionID: dbSessionID.String(),
        AccountID:   principal.AccountID.String(),
        AppID:       principal.AppID.String(),
        Pricing:     h.pricing,
        Prompt:      "", // not used yet
        StructuredCfg: &speech.StructuredOutputConfig{
            UpdateMs: updateMS,
            Schema:            cfgMsg.Structured.Schema,
            ParsingGuide:      cfgMsg.Structured.ParsingGuide,
            ParsingConfig:     cfgMsg.Structured.ParsingConfig,
        },
        PrevStructuredJSON: "",
        InputGuide:         cfgMsg.Structured.ParsingGuide,
        DisablePHI:         cfgMsg.Redaction != nil && cfgMsg.Redaction.DisablePHI,
    }
    
    // Persist and link structured schema for this session
    if h.deps.StructuredOutputSchemasRepo != nil && cfgMsg.Structured != nil {
        // Debug: log structured config being stored
        schemaBytes, _ := json.Marshal(cfgMsg.Structured.Schema)
        logger.ServiceDebugf("WS", "Storing structured schema: update_ms=%d strategy=%q schema_bytes=%d guide_len=%d",
            updateMS, cfgMsg.Structured.ParsingConfig.ParsingStrategy, len(schemaBytes), len(cfgMsg.Structured.ParsingGuide))

        if schemaID, err := h.deps.StructuredOutputSchemasRepo.StoreOrGetSchema(ctx, principal.AppID, dbSessionID, speech.StructuredOutputConfig{
            UpdateMs:        updateMS,
            Schema:          cfgMsg.Structured.Schema,
            ParsingGuide:    cfgMsg.Structured.ParsingGuide,
            ParsingConfig:   cfgMsg.Structured.ParsingConfig,
        }); err == nil {
            _ = h.deps.StructuredOutputSchemasRepo.LinkSchemaToSession(ctx, dbSessionID, schemaID)
            logger.ServiceDebugf("WS", "Structured schema stored/linked (session=%s)", dbSessionID.String())
        } else {
            logger.Warnf("⚠️ [WS] Failed to store/link structured schema: %v", err)
        }
    } else if cfgMsg.Structured != nil {
        logger.Warnf("⚠️ [WS] StructuredOutputSchemasRepo is nil; structured outputs may fail to persist (missing schema link)")
    }

    // Create session-specific STT client (structured mode also needs per-session STT)
    sttProvider := cfgMsg.STT.Provider
    if sttProvider == "" {
        sttProvider = "deepgram"
    }
    diarization := cfgMsg.STT.Diarization
    if diarization.MinSpeakerCount == 0 {
        diarization.MinSpeakerCount = 2
    }
    if diarization.MaxSpeakerCount == 0 {
        diarization.MaxSpeakerCount = 6
    }
    logger.ServiceDebugf("WS", "(structured) Creating STT client: provider=%s diarize.enable=%v min=%d max=%d",
        sttProvider,
        diarization.EnableSpeakerDiarization,
        diarization.MinSpeakerCount,
        diarization.MaxSpeakerCount,
    )
    sttClient, err := h.deps.STTFactory.CreateSTTClient(ctx, sttProvider, diarization)
    if err != nil {
        logger.Errorf("❌ [WS] (structured) Failed to create STT client for provider %s: %v", sttProvider, err)
        logger.ServiceDebugf("WS", "(structured) Falling back to default STT client")
        sttClient = h.deps.STT
    }
    // Build session-specific deps overriding only STT
    sDeps := pipeline.Deps{
        STT:                 sttClient,
        STTFactory:          h.deps.STTFactory,
        FP:                  h.deps.FP,
        LLM:                 h.deps.LLM,
		Normalizer:          h.deps.Normalizer,
        RedactionService:    h.deps.RedactionService, // Include redaction service
        UsageMeterRepo:      h.deps.UsageMeterRepo,
        UsageEventRepo:      h.deps.UsageEventRepo,
        DraftAggRepo:        h.deps.DraftAggRepo,
        FunctionCallsRepo:   h.deps.FunctionCallsRepo,
        FunctionSchemasRepo: h.deps.FunctionSchemasRepo,
        TranscriptsRepo:     h.deps.TranscriptsRepo,
        StructuredOutputsRepo: h.deps.StructuredOutputsRepo,
        StructuredOutputSchemasRepo: h.deps.StructuredOutputSchemasRepo,
    }
    sPl, err = pipeline.NewStructuredPipeline(sCfg, sDeps)
    if err != nil {
        return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to create structured pipeline: %w", err)
    }
    sOutTr, sOutStructured, err := sPl.Run(ctx, audioIn)
    if err != nil {
        return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to run structured pipeline: %w", err)
    }
    // Start usage accumulator (defensive nil checks)
    sAcc := sPl.UsageAccumulator()
    if sAcc == nil {
        logger.Warnf("⚠️ [WS] structured pipeline returned nil usage accumulator")
    } else {
        logger.ServiceDebugf("USAGE", "Starting usage accumulator for session %s", dbSessionID.String())
        sAcc.Start(ctx)
        h.state.UsageAccumulator = sAcc
    }
    // Return with functions channels nil
    return dbSessionID, nil, sPl, sOutTr, nil, nil, sOutStructured, nil
    } else if h.state.LLMMode == "none" {
        // STT-only mode - no pipelines needed
        logger.ServiceDebugf("WS", "STT-only mode - no LLM pipelines created")
        
        // Create a simple STT-only session with just transcript output
        // For now, we'll return nil channels for LLM outputs
        return dbSessionID, nil, nil, nil, nil, nil, nil, nil
    } else {
        // Unknown mode
        logger.Errorf("❌ [WS] Unknown LLM mode: %s", h.state.LLMMode)
        return pgtype.UUID{}, nil, nil, nil, nil, nil, nil, fmt.Errorf("unknown LLM mode: %s", h.state.LLMMode)
    }
}

func (h *Handler) setupPipeline(ctx context.Context, dbSessionID pgtype.UUID, principal domain_auth.Principal, cfgMsg ConfigMessage) (*pipeline.Pipeline, error) {

	// Set default UpdateMS for "auto" strategy if not provided
	updateMS := cfgMsg.Functions.UpdateMS
	if cfgMsg.Functions.ParsingConfig.ParsingStrategy == "auto" && updateMS <= 0 {
		updateMS = 3000 // Default to 3 seconds for auto strategy
		logger.ServiceDebugf("WS", "Using default UpdateMS=%d for auto strategy (functions)", updateMS)
	}

	pCfg := pipeline.ConfigFunctions{
		DBSessionID: dbSessionID.String(),
		AccountID:   principal.AccountID.String(),
		AppID:       principal.AppID.String(),
		Pricing:     h.pricing,
		Prompt:      speech.Prompt(prompts.BuildFunctionsSystemInstructionPrompt(cfgMsg.Functions.ParsingGuide)),
		FuncCfg: &speech.FunctionConfig{
			ParsingConfig:   cfgMsg.Functions.ParsingConfig,
			UpdateMs: updateMS,
			Declarations:      cfgMsg.Functions.Definitions,
			ParsingGuide:      cfgMsg.Functions.ParsingGuide,
		},
		PrevFunctionsJSON: "",
		InputGuide:        cfgMsg.Functions.ParsingGuide,
	}

	if ic := cfgMsg.InputContext; ic != nil {
		if len(ic.CurrentFunctions) > 0 {
			b, _ := json.Marshal(ic.CurrentFunctions)
			pCfg.PrevFunctionsJSON = string(b)
		}
	}

	idx, err := draft.New(
		cfgMsg.Functions.Definitions,
		h.deps.FP,
		env.ModelDir(),
	)

	if err != nil {
		return nil, err
	}

	pCfg.DraftIndex = idx

	// Persist and link function schemas for this session
	if h.deps.FunctionSchemasRepo != nil && cfgMsg.Functions != nil && len(cfgMsg.Functions.Definitions) > 0 {
		// Store the entire function config instead of individual definitions
		functionConfig := speech.FunctionConfigWithoutContext{
			Name:               cfgMsg.Functions.Name,
			Description:        cfgMsg.Functions.Description,
			ParsingConfig:      cfgMsg.Functions.ParsingConfig,
			UpdateMs:  cfgMsg.Functions.UpdateMS,
			Declarations:       cfgMsg.Functions.Definitions,
			ParsingGuide:       cfgMsg.Functions.ParsingGuide,
		}
		
		if schemaID, err := h.deps.FunctionSchemasRepo.StoreOrGetFunctionSchema(ctx, principal.AppID, dbSessionID, functionConfig); err == nil {
			_ = h.deps.FunctionSchemasRepo.LinkSchemaToSession(ctx, dbSessionID, schemaID)
			logger.ServiceDebugf("WS", "Function config stored/linked (session=%s)", dbSessionID.String())
		} else {
			logger.Warnf("⚠️ [WS] Failed to store/link function config: %v", err)
		}
	} else if cfgMsg.Functions != nil && len(cfgMsg.Functions.Definitions) > 0 {
		logger.Warnf("⚠️ [WS] FunctionSchemasRepo is nil; function schemas may fail to persist (missing schema link)")
	}

	// Create STT client based on client's provider preference
	sttProvider := cfgMsg.STT.Provider
	if sttProvider == "" {
		sttProvider = "deepgram" // Default to Deepgram if not specified
	}

	diarization := cfgMsg.STT.Diarization
	if diarization.EnableSpeakerDiarization {
		logger.ServiceDebugf("WS", "Creating STT client for provider: %s with diarization", sttProvider)
	} else {
		logger.ServiceDebugf("WS", "Creating STT client for provider: %s without diarization", sttProvider)
	}

	if diarization.MinSpeakerCount == 0 {
		diarization.MinSpeakerCount = 2
	}

	if diarization.MaxSpeakerCount == 0 {
		diarization.MaxSpeakerCount = 6
	}

	logger.ServiceDebugf("WS", "Creating STT client for provider: %s", sttProvider)
	sttClient, err := h.deps.STTFactory.CreateSTTClient(ctx, sttProvider, diarization)
	if err != nil {
		logger.Errorf("❌ [WS] Failed to create STT client for provider %s: %v", sttProvider, err)
		logger.ServiceDebugf("WS", "Falling back to default STT client")
		sttClient = h.deps.STT // Use fallback STT client
	} else {
		logger.ServiceDebugf("WS", "Successfully created STT client for provider: %s", sttProvider)
	}

	// Create custom dependencies with the session-specific STT client
	sessionDeps := pipeline.Deps{
		STT:                 sttClient, // Use session-specific STT client
		STTFactory:          h.deps.STTFactory,
		FP:                  h.deps.FP,
		LLM:                 h.deps.LLM,
		Normalizer:          h.deps.Normalizer,
		RedactionService:    h.deps.RedactionService, // Include redaction service
		UsageMeterRepo:      h.deps.UsageMeterRepo,
		UsageEventRepo:      h.deps.UsageEventRepo,
		DraftAggRepo:        h.deps.DraftAggRepo,
		FunctionCallsRepo:   h.deps.FunctionCallsRepo,
		FunctionSchemasRepo: h.deps.FunctionSchemasRepo,
		TranscriptsRepo:     h.deps.TranscriptsRepo,
	}

	pl, err := pipeline.New(pCfg, sessionDeps)
	if err != nil {
		return nil, err
	}

	return pl, nil
}
