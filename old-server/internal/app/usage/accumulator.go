package usage

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/usage"
	"schma.ai/internal/pkg/logger"
)

// UsageAccumulator manages session usage metering with periodic flushing
type UsageAccumulator struct {
	meter           usage.Meter
	meterRepo       usage.UsageMeterRepo
	eventRepo       usage.UsageEventRepo
	draftAggregator *DraftAggregator

	// Channels for async processing
	eventChan chan usage.UsageEvent
	stopChan  chan struct{}
	doneChan  chan struct{}

	// Synchronization
	mu            sync.RWMutex
	isRunning     bool
	lastFlushTime time.Time

	// Configuration
	flushInterval time.Duration
	batchMode     bool // If true, disable periodic flushing and batch at end

	// Batching storage
	pendingEvents []usage.UsageEvent

    // Savings (aggregated within session)
    savedPromptTokensTotal int64
    savedPromptCostTotal   float64
    
    // LLM mode for metrics
    llmMode string // "functions", "structured", "none"
}

// NewUsageAccumulator creates a new usage accumulator for a session
func NewUsageAccumulator(
	sessionID, appID, accountID pgtype.UUID,
	meterRepo usage.UsageMeterRepo,
	eventRepo usage.UsageEventRepo,
	draftRepo usage.DraftAggRepo,
	batchMode bool,
	llmMode string,
) *UsageAccumulator {
	meter := usage.NewMeter(usage.DefaultPricing)
	meter.SessionID = sessionID
	meter.AppID = appID
	meter.AccountID = accountID

	// Create draft aggregator
	draftAggregator := NewDraftAggregator(sessionID, appID, accountID, draftRepo, batchMode)

	return &UsageAccumulator{
		meter:           meter,
		meterRepo:       meterRepo,
		eventRepo:       eventRepo,
		draftAggregator: draftAggregator,
		eventChan:       make(chan usage.UsageEvent, 100), // Buffered channel
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
		flushInterval:   5 * time.Second,
		lastFlushTime:   time.Now(),
		batchMode:       batchMode,
		pendingEvents:   make([]usage.UsageEvent, 0),
		llmMode:         llmMode,
	}
}

// Start begins the usage accumulation process with periodic flushing
func (ua *UsageAccumulator) Start(ctx context.Context) {
	ua.mu.Lock()
	if ua.isRunning {
		ua.mu.Unlock()
		return
	}
	ua.isRunning = true
	ua.mu.Unlock()

	logger.ServiceDebugf("USAGE", "Starting usage accumulator for session %s", ua.meter.SessionID)

    // Start draft aggregator
	ua.draftAggregator.Start(ctx)

    go ua.accumulatorLoop(ctx)
    // Start lightweight CPU sampler (active vs idle) for observability billing
    go ua.cpuSamplerLoop(ctx)
}

// Stop stops the accumulator and performs final flush
func (ua *UsageAccumulator) Stop(ctx context.Context) {
	ua.mu.Lock()
	if !ua.isRunning {
		ua.mu.Unlock()
		return
	}
	ua.isRunning = false
	ua.mu.Unlock()

	logger.ServiceDebugf("USAGE", "Stopping usage accumulator for session %s", ua.meter.SessionID)

	// Signal stop and wait for completion
	close(ua.stopChan)
	<-ua.doneChan

	// Stop draft aggregator
	ua.draftAggregator.Stop(ctx)

	// Final flush
	if ua.batchMode {
		ua.finalBatchFlush(ctx)
	} else {
		ua.flushTotals(ctx)
	}

	logger.ServiceDebugf("USAGE", "Usage accumulator stopped for session %s", ua.meter.SessionID)
}

// cpuSamplerLoop periodically samples CPU active/idle seconds based on event backlog and timer
func (ua *UsageAccumulator) cpuSamplerLoop(ctx context.Context) {
    // CPU tracking is now handled directly in the WebSocket handler
    // during audio_start -> audio_stop cycles
    // This loop is kept for backward compatibility but no longer tracks CPU time
    
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ua.stopChan:
            return
        case <-ticker.C:
            // CPU tracking is now handled directly in WebSocket handler
            // No more background CPU sampling
            continue
        }
    }
}

// processOrBufferEvent mirrors accumulatorLoop handling when we opportunistically pulled an event
func (ua *UsageAccumulator) processOrBufferEvent(ctx context.Context, event usage.UsageEvent) {
    if ua.batchMode {
        ua.mu.Lock()
        ua.pendingEvents = append(ua.pendingEvents, event)
        ua.mu.Unlock()
        return
    }
    ua.processEvent(ctx, event)
}

// AddSTT records STT usage
func (ua *UsageAccumulator) AddSTT(audioSeconds float64, provider string) {
	ua.mu.Lock()
	ua.meter.AddSTT(audioSeconds)
	ua.mu.Unlock()

	// Log detailed event with different handling for session_total vs chunk-based
	eventType := "stt"
	if provider == "session_total" {
		eventType = "audio_session"
		logger.ServiceDebugf("USAGE", "Session total audio duration: %.3f seconds", audioSeconds)
	} else {
		logger.ServiceDebugf("USAGE", "STT chunk duration: %.3f seconds (provider: %s)", audioSeconds, provider)
	}

	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      eventType,
		Metric: map[string]interface{}{
			"audio_seconds": audioSeconds,
			"provider":      provider,
			"cost":          audioSeconds / 60 * usage.DefaultPricing.CostAudioPerMin,
		},
		LoggedAt: time.Now(),
	})
}

// AddLLM records LLM usage
func (ua *UsageAccumulator) AddLLM(promptTokens, completionTokens int64, provider string, model string) {
	ua.mu.Lock()
	ua.meter.AddTokens(promptTokens, completionTokens)
	ua.mu.Unlock()

	// Calculate costs
	promptCost := float64(promptTokens) / 1_000_000 * usage.DefaultPricing.CostGemPromptPer1M
	completionCost := float64(completionTokens) / 1_000_000 * usage.DefaultPricing.CostGemCompletionPer1M

	// Log detailed event
	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      "llm",
		Metric: map[string]interface{}{
			"llm_mode":           ua.llmMode,
			"prompt_tokens":      promptTokens,
			"completion_tokens":  completionTokens,
			"provider":           provider,
			"model":              model,
			"prompt_cost":        promptCost,
			"completion_cost":    completionCost,
			"total_cost":         promptCost + completionCost,
		},
		LoggedAt: time.Now(),
	})
}

// AddLLMWithSavings records LLM usage with token savings metadata from caching
func (ua *UsageAccumulator) AddLLMWithSavings(promptTokens, completionTokens, savedPromptTokens int64, provider string, model string) {
	ua.mu.Lock()
	ua.meter.AddTokens(promptTokens, completionTokens)
	// Accumulate saved tokens and cost for session totals
	ua.savedPromptTokensTotal += savedPromptTokens
	savedCost := float64(savedPromptTokens) / 1_000_000 * usage.DefaultPricing.CostGemPromptPer1M
	ua.savedPromptCostTotal += savedCost
	ua.mu.Unlock()

	// Calculate costs
	promptCost := float64(promptTokens) / 1_000_000 * usage.DefaultPricing.CostGemPromptPer1M
	completionCost := float64(completionTokens) / 1_000_000 * usage.DefaultPricing.CostGemCompletionPer1M

	logger.ServiceDebugf("USAGE", "LLM with savings: prompt=%d completion=%d saved=%d (session_saved_total=%d)", 
		promptTokens, completionTokens, savedPromptTokens, ua.savedPromptTokensTotal)

	// Log detailed event with savings
	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      "llm_cached",
		Metric: map[string]interface{}{
			"llm_mode":              ua.llmMode,
			"prompt_tokens":         promptTokens,
			"completion_tokens":     completionTokens,
			"saved_prompt_tokens":   savedPromptTokens,
			"provider":              provider,
			"model":                 model,
			"prompt_cost":           promptCost,
			"completion_cost":       completionCost,
			"saved_prompt_cost":     savedCost,
			"total_cost":            promptCost + completionCost,
			"cache_savings":         savedCost,
		},
		LoggedAt: time.Now(),
	})
}

// AddLLMError records LLM error for metrics and analytics
func (ua *UsageAccumulator) AddLLMError(provider string, model string, errorType string, errorMessage string) {
	// Log detailed error event
	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      "llm_error",
		Metric: map[string]interface{}{
			"llm_mode":     ua.llmMode,
			"provider":     provider,
			"model":        model,
			"error_type":   errorType,
			"error_message": errorMessage,
		},
		LoggedAt: time.Now(),
	})
}

// AddCPUActiveTime records CPU active time directly (used by WebSocket handler)
func (ua *UsageAccumulator) AddCPUActiveTime(duration time.Duration) {
	ua.mu.Lock()
	ua.meter.AddCPUActive(duration)
	ua.mu.Unlock()

	logger.ServiceDebugf("USAGE", "CPU Active: +%.3f seconds (total: %.3f)", duration.Seconds(), ua.meter.CPUActiveSeconds)
}

// SetCPUIdleToZero sets CPU idle time to 0 (used by WebSocket handler)
func (ua *UsageAccumulator) SetCPUIdleToZero() {
	ua.mu.Lock()
	ua.meter.SetCPUIdle(0)
	ua.mu.Unlock()

	logger.ServiceDebugf("USAGE", "CPU Idle time set to 0")
}

// AddCPU records CPU usage
func (ua *UsageAccumulator) AddCPU(activeDuration, idleDuration time.Duration) {
	ua.mu.Lock()
	ua.meter.AddCPUActive(activeDuration)
	ua.meter.AddCPUIdle(idleDuration)
	ua.mu.Unlock()

	// Log detailed event
	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      "cpu",
		Metric: map[string]interface{}{
			"active_seconds": activeDuration.Seconds(),
			"idle_seconds":   idleDuration.Seconds(),
			"active_cost":    activeDuration.Seconds() * usage.DefaultPricing.CostFlyPerSec,
			"idle_cost":      idleDuration.Seconds() * usage.DefaultPricing.CostFlyPerSec * usage.DefaultPricing.IdleDiscount,
		},
		LoggedAt: time.Now(),
	})
}

// AddFunctionCall records function call usage (both draft and final)
func (ua *UsageAccumulator) AddFunctionCall(functionName string, isDraft bool, args map[string]interface{}, similarity float64) {
	// Record function call event
	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      "function_call",
		Metric: map[string]interface{}{
			"function_name": functionName,
			"is_draft":      isDraft,
			"similarity":    similarity,
			"args":          args,
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		},
		LoggedAt: time.Now(),
	})
}

// AddDraftFunction records draft function detection
func (ua *UsageAccumulator) AddDraftFunction(functionName string, similarity float64, args map[string]interface{}) {
	// Record in usage events
	ua.AddFunctionCall(functionName, true, args, similarity)

	// Record in draft aggregator
	ua.draftAggregator.RecordDraftFunction(functionName, similarity, args)
}

// AddStructuredOutput records structured output events
func (ua *UsageAccumulator) AddStructuredOutput(revision int, delta map[string]interface{}, final map[string]interface{}) {
	// Extract keys from delta and final for metrics (avoid storing full objects)
	deltaKeys := make([]string, 0, len(delta))
	for key := range delta {
		deltaKeys = append(deltaKeys, key)
	}
	
	finalKeys := make([]string, 0, len(final))
	for key := range final {
		finalKeys = append(finalKeys, key)
	}

	// Record structured output event
	ua.logEvent(usage.UsageEvent{
		SessionID: ua.meter.SessionID,
		AppID:     ua.meter.AppID,
		AccountID: ua.meter.AccountID,
		Type:      "structured_output",
		Metric: map[string]interface{}{
			"revision":     revision,
			"delta_keys":   deltaKeys,
			"final_keys":   finalKeys,
			"delta_count":  len(delta),
			"final_count":  len(final),
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		},
		LoggedAt: time.Now(),
	})
}

// AddFinalFunctions records final function calls from LLM
func (ua *UsageAccumulator) AddFinalFunctions(functions []map[string]interface{}) {
	for _, fn := range functions {
		if name, ok := fn["name"].(string); ok {
			args, _ := fn["args"].(map[string]interface{})
			// Record in usage events
			ua.AddFunctionCall(name, false, args, 1.0) // Final functions have 100% confidence

			// Record in draft aggregator
			ua.draftAggregator.RecordFinalFunction(name, args)
		}
	}
}

// GetCurrentTotals returns the current usage totals (thread-safe)
func (ua *UsageAccumulator) GetCurrentTotals() (usage.Meter, usage.Cost) {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	meter := ua.meter // Copy
    cost := meter.Cost(usage.DefaultPricing)
    // Adjust total cost by subtracting cached prompt savings
    cost.TotalCost -= ua.savedPromptCostTotal
	return meter, cost
}

// accumulatorLoop runs the main accumulation loop
func (ua *UsageAccumulator) accumulatorLoop(ctx context.Context) {
	defer close(ua.doneChan)

	ticker := time.NewTicker(ua.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.ServiceDebugf("USAGE", "Context cancelled for usage accumulator %s", ua.meter.SessionID)
			return

		case <-ua.stopChan:
			logger.ServiceDebugf("USAGE", "Stop signal received for usage accumulator %s", ua.meter.SessionID)
			return

		case <-ticker.C:
			if !ua.batchMode {
				ua.flushTotals(ctx)
			}

		case event := <-ua.eventChan:
			if ua.batchMode {
				// Store event in memory for batch processing
				ua.mu.Lock()
				ua.pendingEvents = append(ua.pendingEvents, event)
				ua.mu.Unlock()
			} else {
				ua.processEvent(ctx, event)
			}
		}
	}
}

// flushTotals performs periodic flush of accumulated totals
func (ua *UsageAccumulator) flushTotals(ctx context.Context) {
	ua.mu.RLock()
	meter := ua.meter // Copy for safety
    savedTokens := ua.savedPromptTokensTotal
    savedCost := ua.savedPromptCostTotal
	ua.mu.RUnlock()

    cost := meter.Cost(usage.DefaultPricing)
    // Adjust total cost by subtracting cached prompt savings
    cost.TotalCost -= savedCost

    if _, err := ua.meterRepo.Save(ctx, meter, cost, savedTokens, savedCost); err != nil {
		logger.Errorf("❌ [USAGE] Error flushing usage totals for session %s: %v", meter.SessionID, err)
		return
	}

    // Emit aggregate savings event alongside totals (until totals table stores savings columns)
    ua.logEvent(usage.UsageEvent{
        SessionID: meter.SessionID,
        AppID:     meter.AppID,
        AccountID: meter.AccountID,
        Type:      "llm_savings_totals",
        Metric: map[string]interface{}{
            "saved_prompt_tokens_total": savedTokens,
            "saved_prompt_cost_total":   savedCost,
        },
        LoggedAt: time.Now(),
    })

	ua.mu.Lock()
	ua.lastFlushTime = time.Now()
	ua.mu.Unlock()

	logger.ServiceDebugf("USAGE", "Flushed usage totals for session %s: audio=%.2fs, tokens=%d/%d, saved_tokens=%d, cost=$%.6f",
		meter.SessionID, meter.AudioSeconds, meter.PromptTokens, meter.CompletionTokens, savedTokens, cost.TotalCost)
}

// GetSavedPromptTotals returns aggregate savings captured during the session
func (ua *UsageAccumulator) GetSavedPromptTotals() (tokens int64, cost float64) {
    ua.mu.RLock()
    defer ua.mu.RUnlock()
    return ua.savedPromptTokensTotal, ua.savedPromptCostTotal
}

// logEvent queues an event for async processing
func (ua *UsageAccumulator) logEvent(event usage.UsageEvent) {
	// Skip high-volume events in batch mode to reduce database load
	if ua.batchMode {
		// Type assert the metric to map for access
		if metricMap, ok := event.Metric.(map[string]interface{}); ok {
			switch event.Type {
			case "stt":
				// Only log significant STT events, not every small chunk
				if audioSecs, ok := metricMap["audio_seconds"].(float64); ok && audioSecs < 0.1 {
					return // Skip very short audio chunks (< 0.1 seconds)
				}
			case "function_call":
				// Only log draft function calls with high confidence, skip low-confidence ones
				if isDraft, ok := metricMap["is_draft"].(bool); ok && isDraft {
					if similarity, ok := metricMap["similarity"].(float64); ok && similarity < 0.8 {
						return // Skip low-confidence draft function calls
					}
				}
			case "structured_output":
				// Only log structured output events with significant changes
				if deltaCount, ok := metricMap["delta_count"].(int); ok && deltaCount == 0 {
					return // Skip structured output events with no changes
				}
			}
		}
	}

	select {
	case ua.eventChan <- event:
		// Event queued successfully
	default:
		// Channel full, log warning but don't block
		logger.Warnf("⚠️ [USAGE] Usage event channel full for session %s, dropping event", ua.meter.SessionID)
	}
}

// processEvent handles individual usage events
func (ua *UsageAccumulator) processEvent(ctx context.Context, event usage.UsageEvent) {
	if err := ua.eventRepo.LogEvent(ctx, event); err != nil {
		// Log error but don't fail - detailed events are supplementary
		metricBytes, _ := json.Marshal(event.Metric)
		logger.Errorf("❌ [USAGE] Error logging usage event for session %s: %v (event: %s %s)",
			event.SessionID, err, event.Type, string(metricBytes))
	}
}

// finalBatchFlush performs a final batch write of all accumulated data at session end
func (ua *UsageAccumulator) finalBatchFlush(ctx context.Context) {
	ua.mu.RLock()
	meter := ua.meter // Copy for safety
	events := make([]usage.UsageEvent, len(ua.pendingEvents))
	copy(events, ua.pendingEvents)
	ua.mu.RUnlock()

	// Calculate final cost
	cost := meter.Cost(usage.DefaultPricing)

    // Write usage totals to database
    st, sc := ua.GetSavedPromptTotals()
    if _, err := ua.meterRepo.Save(ctx, meter, cost, st, sc); err != nil {
		logger.Errorf("❌ [USAGE] Error writing final usage totals for session %s: %v", meter.SessionID, err)
	}

	// Write all accumulated events to database
	for _, event := range events {
		if err := ua.eventRepo.LogEvent(ctx, event); err != nil {
			metricBytes, _ := json.Marshal(event.Metric)
			logger.Errorf("❌ [USAGE] Error writing usage event for session %s: %v (event: %s %s)",
				event.SessionID, err, event.Type, string(metricBytes))
		}
	}

	logger.ServiceDebugf("USAGE", "Final batch flush completed for session %s: audio=%.2fs, tokens=%d/%d, saved_tokens=%d, cost=$%.6f, events=%d",
		meter.SessionID, meter.AudioSeconds, meter.PromptTokens, meter.CompletionTokens, st, cost.TotalCost, len(events))
}
