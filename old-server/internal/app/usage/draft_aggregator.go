package usage

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	domain_usage "schma.ai/internal/domain/usage"
	"schma.ai/internal/pkg/logger"
)

// DraftAggregator handles real-time aggregation of draft function calls
type DraftAggregator struct {
	sessionID pgtype.UUID
	appID     pgtype.UUID
	accountID pgtype.UUID
	draftRepo domain_usage.DraftAggRepo

	// In-memory aggregation state
	mu            sync.RWMutex
	functionStats map[string]*FunctionStats // function name -> stats
	sessionStats  *SessionStats

	// Channels for async processing
	eventChan chan DraftEvent
	stopChan  chan struct{}

	// Configuration
	flushInterval time.Duration
	batchMode     bool // If true, disable periodic flushing and batch at end
}

// FunctionStats tracks per-function aggregation data
type FunctionStats struct {
	Name            string
	TotalDetections int64
	HighestScore    float64
	ScoreSum        float64 // For calculating average
	FirstDetected   time.Time
	LastDetected    time.Time
	BestArgs        interface{}
	VersionCount    int64
	FinalCallCount  int64
	DraftEvents     []time.Time // For latency calculations
	FinalEvents     []time.Time
}

// SessionStats tracks session-level statistics
type SessionStats struct {
	TotalDraftFunctions int64
	TotalFinalFunctions int64
	UniqueFunctions     map[string]bool
	FunctionFrequency   map[string]int64
}

// DraftEvent represents a function-related event
type DraftEvent struct {
	Type         string // "draft", "final"
	FunctionName string
	Score        float64 // For draft events
	Args         interface{}
	Timestamp    time.Time
}

func NewDraftAggregator(sessionID, appID, accountID pgtype.UUID, draftRepo domain_usage.DraftAggRepo, batchMode bool) *DraftAggregator {
	return &DraftAggregator{
		sessionID:     sessionID,
		appID:         appID,
		accountID:     accountID,
		draftRepo:     draftRepo,
		functionStats: make(map[string]*FunctionStats),
		sessionStats: &SessionStats{
			UniqueFunctions:   make(map[string]bool),
			FunctionFrequency: make(map[string]int64),
		},
		eventChan:     make(chan DraftEvent, 100),
		stopChan:      make(chan struct{}),
		flushInterval: 10 * time.Second, // Flush every 10 seconds
		batchMode:     batchMode,        // Store batch mode flag
	}
}

func (da *DraftAggregator) Start(ctx context.Context) {
	go da.processingLoop(ctx)
}

func (da *DraftAggregator) Stop(ctx context.Context) {
	close(da.stopChan)
	// Final flush
	da.flushAggregations(ctx)
}

// RecordDraftFunction records a draft function detection
func (da *DraftAggregator) RecordDraftFunction(functionName string, score float64, args interface{}) {
	select {
	case da.eventChan <- DraftEvent{
		Type:         "draft",
		FunctionName: functionName,
		Score:        score,
		Args:         args,
		Timestamp:    time.Now(),
	}:
	default:
		logger.Warnf("⚠️ [USAGE] Draft aggregator event channel full for session %s", da.sessionID)
	}
}

// RecordFinalFunction records a final function call
func (da *DraftAggregator) RecordFinalFunction(functionName string, args interface{}) {
	select {
	case da.eventChan <- DraftEvent{
		Type:         "final",
		FunctionName: functionName,
		Args:         args,
		Timestamp:    time.Now(),
	}:
	default:
		logger.Warnf("⚠️ [USAGE] Draft aggregator event channel full for session %s", da.sessionID)
	}
}

func (da *DraftAggregator) processingLoop(ctx context.Context) {
	ticker := time.NewTicker(da.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-da.stopChan:
			return
		case <-ticker.C:
			if !da.batchMode {
				da.flushAggregations(ctx)
			}
		case event := <-da.eventChan:
			da.processEvent(event)
		}
	}
}

func (da *DraftAggregator) processEvent(event DraftEvent) {
	da.mu.Lock()
	defer da.mu.Unlock()

	// Get or create function stats
	stats, exists := da.functionStats[event.FunctionName]
	if !exists {
		stats = &FunctionStats{
			Name:          event.FunctionName,
			FirstDetected: event.Timestamp,
			DraftEvents:   make([]time.Time, 0),
			FinalEvents:   make([]time.Time, 0),
		}
		da.functionStats[event.FunctionName] = stats
		da.sessionStats.UniqueFunctions[event.FunctionName] = true
	}

	// Update function stats based on event type
	switch event.Type {
	case "draft":
		stats.TotalDetections++
		stats.ScoreSum += event.Score
		stats.LastDetected = event.Timestamp
		stats.DraftEvents = append(stats.DraftEvents, event.Timestamp)

		// Update highest score and best args
		if event.Score > stats.HighestScore {
			stats.HighestScore = event.Score
			stats.BestArgs = event.Args
		}

		// Update session stats
		da.sessionStats.TotalDraftFunctions++
		da.sessionStats.FunctionFrequency[event.FunctionName]++

	case "final":
		stats.FinalCallCount++
		stats.FinalEvents = append(stats.FinalEvents, event.Timestamp)
		da.sessionStats.TotalFinalFunctions++
	}
}

func (da *DraftAggregator) flushAggregations(ctx context.Context) {
	da.mu.RLock()
	defer da.mu.RUnlock()

	// Flush function-level aggregations
	for _, stats := range da.functionStats {
		avgScore := 0.0
		if stats.TotalDetections > 0 {
			avgScore = stats.ScoreSum / float64(stats.TotalDetections)
		}

		agg := domain_usage.DraftAgg{
			SessionID:       da.sessionID,
			AppID:           da.appID,
			AccountID:       da.accountID,
			FunctionName:    stats.Name,
			TotalDetections: stats.TotalDetections,
			HighestScore:    stats.HighestScore,
			AvgScore:        avgScore,
			FirstDetected:   stats.FirstDetected,
			LastDetected:    stats.LastDetected,
			SampleArgs:      stats.BestArgs,
			VersionCount:    1, // TODO: Implement argument variation detection
			FinalCallCount:  stats.FinalCallCount,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		if err := da.draftRepo.UpsertDraftAgg(ctx, agg); err != nil {
			logger.Errorf("❌ [USAGE] Error upserting draft aggregation for %s: %v", stats.Name, err)
		}
	}

	// Calculate and flush session-level statistics
	da.flushSessionStats(ctx)
}

func (da *DraftAggregator) flushSessionStats(ctx context.Context) {
	// Calculate draft-to-final ratio
	ratio := 0.0
	if da.sessionStats.TotalFinalFunctions > 0 {
		ratio = float64(da.sessionStats.TotalDraftFunctions) / float64(da.sessionStats.TotalFinalFunctions)
	}

	// Find most frequent function
	var topFunction string
	var maxFreq int64
	for funcName, freq := range da.sessionStats.FunctionFrequency {
		if freq > maxFreq {
			maxFreq = freq
			topFunction = funcName
		}
	}

	// Calculate average detection latency (simplified)
	avgLatency := da.calculateAverageLatency()

	stats := domain_usage.DraftAggStats{
		SessionID:           da.sessionID,
		AppID:               da.appID,
		AccountID:           da.accountID,
		TotalDraftFunctions: da.sessionStats.TotalDraftFunctions,
		TotalFinalFunctions: da.sessionStats.TotalFinalFunctions,
		DraftToFinalRatio:   ratio,
		UniqueFunction:      int64(len(da.sessionStats.UniqueFunctions)),
		AvgDetectionLatency: avgLatency,
		TopFunction:         topFunction,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := da.draftRepo.UpsertDraftAggStats(ctx, stats); err != nil {
		logger.Errorf("❌ [USAGE] Error upserting draft aggregation stats: %v", err)
	}
}

func (da *DraftAggregator) calculateAverageLatency() float64 {
	// Simplified latency calculation
	// In a more sophisticated implementation, you'd track draft->final correlations
	totalLatency := 0.0
	totalPairs := 0

	for _, stats := range da.functionStats {
		if len(stats.DraftEvents) > 0 && len(stats.FinalEvents) > 0 {
			// Simple heuristic: time from first draft to first final
			latency := stats.FinalEvents[0].Sub(stats.DraftEvents[0]).Seconds()
			if latency > 0 {
				totalLatency += latency
				totalPairs++
			}
		}
	}

	if totalPairs > 0 {
		return totalLatency / float64(totalPairs)
	}
	return 0.0
}
