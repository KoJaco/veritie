package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	batch_app "schma.ai/internal/app/batch"
	"schma.ai/internal/domain/batch"
	db_domain "schma.ai/internal/domain/db"
	"schma.ai/internal/domain/session"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/pkg/paths"
	"schma.ai/internal/transport/http/middleware"
)

type BatchHandler struct {
	jobRepo                   batch.JobRepo
	queueManager              *batch_app.QueueManager
	sessionManager            session.Manager
	functionSchemasRepo       db_domain.FunctionSchemasRepo
	structuredOutputSchemasRepo db_domain.StructuredOutputSchemasRepo
}

func NewBatchHandler(
	jobRepo batch.JobRepo,
	queueManager *batch_app.QueueManager,
	sessionManager session.Manager,
	functionSchemasRepo db_domain.FunctionSchemasRepo,
	structuredOutputSchemasRepo db_domain.StructuredOutputSchemasRepo,
) *BatchHandler {
	return &BatchHandler{
		jobRepo:                     jobRepo,
		queueManager:                queueManager,
		sessionManager:              sessionManager,
		functionSchemasRepo:         functionSchemasRepo,
		structuredOutputSchemasRepo: structuredOutputSchemasRepo,
	}
}

// TODO: add propery error handling back to client.
func (h *BatchHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	// Get authenticated principal
	principal, ok := middleware.GetPrincipal(r.Context())
	if !ok {	
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}	
	logger.ServiceDebugf("BATCH", "HandleUpload start: app_id=%s account_id=%s", principal.AppID, principal.AccountID)

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		logger.Errorf("❌ [BATCH] ParseMultipartForm failed: %v", err)
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}
	logger.ServiceDebugf("BATCH", "Multipart form parsed")

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		logger.Errorf("❌ [BATCH] FormFile(file) failed: %v", err)
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()
	logger.ServiceDebugf("BATCH", "Received file: name=%s size=%d", header.Filename, header.Size)

	// Get config
	configStr := r.FormValue("config")
	if configStr == "" {
		logger.Errorf("❌ [BATCH] Missing config form field")
		http.Error(w, "No config provided", http.StatusBadRequest)
		return
	}
	logger.ServiceDebugf("BATCH", "Raw config length=%d", len(configStr))

	var config map[string]any
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		logger.Errorf("❌ [BATCH] Invalid config JSON: %v", err)
		http.Error(w, "Invalid config JSON", http.StatusBadRequest)
		return
	}

	// Validate config: exactly one of function_config or structured_output_config
	_, hasFns := config["function_config"]
	_, hasStructured := config["structured_output_config"]

	if (hasFns && hasStructured) || (!hasFns && !hasStructured) {
		logger.Errorf("❌ [BATCH] Config must include exactly one of function_config or structured_output_config (hasFns=%v hasStructured=%v)", hasFns, hasStructured)
		http.Error(w, "Config must include exactly one of function_config or structured_output_config", http.StatusBadRequest)
		return
	}

	// Restrict parsing_strategy to {update-ms, end-of-session}; default to end-of-session
	validateStrategy := func(v any) (string, bool) {
		s, ok := v.(string)
		if !ok || s == "" { return "end-of-session", true }
		if s == "update-ms" || s == "end-of-session" { return s, true }
		return "", false
	}

	if hasFns {
		fn, ok := config["function_config"].(map[string]any)
		if !ok {
			logger.Errorf("❌ [BATCH] function_config must be an object")
			http.Error(w, "function_config must be an object", http.StatusBadRequest)
			return
		}
		if val, exists := fn["parsing_strategy"]; exists {
			if norm, ok := validateStrategy(val); ok {
				fn["parsing_strategy"] = norm
			} else {
				logger.Errorf("❌ [BATCH] Invalid parsing_strategy for function_config: %v", val)
				http.Error(w, "Invalid parsing_strategy for function_config. Allowed: update-ms, end-of-session", http.StatusBadRequest)
				return
			}
		} else {
			fn["parsing_strategy"] = "end-of-session"
		}
		config["function_config"] = fn
		logger.ServiceDebugf("BATCH", "Normalized function_config parsing_strategy=%v", config["function_config"].(map[string]any)["parsing_strategy"])
	}

	if hasStructured {
		sc, ok := config["structured_output_config"].(map[string]any)
		if !ok {
			logger.Errorf("❌ [BATCH] structured_output_config must be an object")
			http.Error(w, "structured_output_config must be an object", http.StatusBadRequest)
			return
		}
		if val, exists := sc["parsing_strategy"]; exists {
			if norm, ok := validateStrategy(val); ok {
				sc["parsing_strategy"] = norm
			} else {
				logger.Errorf("❌ [BATCH] Invalid parsing_strategy for structured_output_config: %v", val)
				http.Error(w, "Invalid parsing_strategy for structured_output_config. Allowed: update-ms, end-of-session", http.StatusBadRequest)
				return
			}
		} else {
			sc["parsing_strategy"] = "end-of-session"
		}
		config["structured_output_config"] = sc
		logger.ServiceDebugf("BATCH", "Normalized structured_output_config parsing_strategy=%v", config["structured_output_config"].(map[string]any)["parsing_strategy"])
	}

	// 1. Create batch session first
	sessionState, err := h.sessionManager.StartSession(r.Context(), session.WSSessionID(""), false, db.SessionKindEnumBatch, principal)
	if err != nil {
		logger.Errorf("❌ [BATCH] Failed to create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	sessionID := pgtype.UUID(sessionState.ID)
	logger.ServiceDebugf("BATCH", "Created batch session: id=%s", sessionID.String())

	// 2. Store configuration schemas with checksum deduplication
	if hasFns {
		funcCfgData := config["function_config"].(map[string]any)
		funcConfig, err := mapToFunctionConfig(funcCfgData)
		if err != nil {
			logger.Errorf("❌ [BATCH] Failed to parse function config: %v", err)
			http.Error(w, "Invalid function config", http.StatusBadRequest)
			return
		}

		// Store function schema with checksum deduplication
		schemaID, err := h.functionSchemasRepo.StoreOrGetFunctionSchema(r.Context(), principal.AppID, sessionID, funcConfig)
		if err != nil {
			logger.Errorf("❌ [BATCH] Failed to store function schema: %v", err)
			http.Error(w, "Failed to store function schema", http.StatusInternalServerError)
			return
		}

		// Link schema to session
		if err := h.functionSchemasRepo.LinkSchemaToSession(r.Context(), sessionID, schemaID); err != nil {
			logger.Errorf("❌ [BATCH] Failed to link function schema to session: %v", err)
			// Non-fatal - continue processing
		}
		logger.ServiceDebugf("BATCH", "Stored function schema: id=%s", schemaID.String())
	}

	if hasStructured {
		structCfgData := config["structured_output_config"].(map[string]any)
		structConfig, err := mapToStructuredOutputConfig(structCfgData)
		if err != nil {
			logger.Errorf("❌ [BATCH] Failed to parse structured output config: %v", err)
			http.Error(w, "Invalid structured output config", http.StatusBadRequest)
			return
		}

		logger.ServiceDebugf("BATCH", "Structured output config: %v", structConfig)

		// Store structured output schema with checksum deduplication
		schemaID, err := h.structuredOutputSchemasRepo.StoreOrGetSchema(r.Context(), principal.AppID, sessionID, structConfig)
		if err != nil {
			logger.Errorf("❌ [BATCH] Failed to store structured output schema: %v", err)
			http.Error(w, "Failed to store structured output schema", http.StatusInternalServerError)
			return
		}

		// Link schema to session
		if err := h.structuredOutputSchemasRepo.LinkSchemaToSession(r.Context(), sessionID, schemaID); err != nil {
			logger.Errorf("❌ [BATCH] Failed to link structured output schema to session: %v", err)
			// Non-fatal - continue processing
		}
		logger.ServiceDebugf("BATCH", "Stored structured output schema: id=%s", schemaID.String())
	}

	// 3. Save file to temporary location
	jobID := generateJobID()
	filePath := filepath.Join(paths.BatchDir(jobID), header.Filename)

	if err := saveUploadedFile(file, filePath); err != nil {
		logger.Errorf("❌ [BATCH] Failed to save uploaded file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	logger.ServiceDebugf("BATCH", "Saved upload to %s", filePath)

	// 4. Create batch job linked to session (no config stored here)
	job, err := h.jobRepo.Create(r.Context(), principal.AppID, principal.AccountID, sessionID, filePath, header.Size)
	if err != nil {
		logger.Errorf("❌ [BATCH] Failed to create job: %v", err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}
	logger.ServiceDebugf("BATCH", "Created job: id=%s session_id=%s status=%s", job.ID.String(), sessionID.String(), job.Status)

	// Enqueue job for processing
	h.queueManager.EnqueueJob(job.ID.String())
	logger.ServiceDebugf("BATCH", "Enqueued job: id=%s", job.ID.String())

	// Return 202 Accepted with job ID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	response := map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Job queued successfully",
	}

	_ = json.NewEncoder(w).Encode(response)
}

func (h *BatchHandler) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	// Get job ID from URL query parameter
	jobIDStr := r.URL.Query().Get("job_id")
	if jobIDStr == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	// Parse UUID
	var jobID pgtype.UUID
	if err := jobID.Scan(jobIDStr); err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	// Get job
	job, err := h.jobRepo.Get(r.Context(), jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Return job status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *BatchHandler) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	// Get authenticated principal
	principal, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse pagination parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0 // default
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get jobs for this app
	jobs, err := h.jobRepo.ListByApp(r.Context(), principal.AppID, limit, offset)
	if err != nil {
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	// Return jobs
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"jobs":   jobs,
		"limit":  limit,
		"offset": offset,
		"count":  len(jobs),
	}
	json.NewEncoder(w).Encode(response)
}

func saveUploadedFile(src io.Reader, dstPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

// mapToFunctionConfig converts a map to speech.FunctionConfigWithoutContext
func mapToFunctionConfig(data map[string]any) (speech.FunctionConfigWithoutContext, error) {
	// Marshal and unmarshal to ensure type safety
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return speech.FunctionConfigWithoutContext{}, err
	}

	var config speech.FunctionConfigWithoutContext
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return speech.FunctionConfigWithoutContext{}, err
	}

	return config, nil
}

// mapToStructuredOutputConfig converts a map to speech.StructuredOutputConfig
func mapToStructuredOutputConfig(data map[string]any) (speech.StructuredOutputConfig, error) {
	// Marshal and unmarshal to ensure type safety
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return speech.StructuredOutputConfig{}, err
	}

	var config speech.StructuredOutputConfig
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return speech.StructuredOutputConfig{}, err
	}

	return config, nil
}
