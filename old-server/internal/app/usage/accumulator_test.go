package usage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	dusage "schma.ai/internal/domain/usage"
	db "schma.ai/internal/infra/db/generated"
)

// --- fakes ---

type fakeMeterRepo struct{
    lastSaved struct{
        meter  dusage.Meter
        cost   dusage.Cost
        savedT int64
        savedC float64
    }
}

func (f *fakeMeterRepo) Save(ctx context.Context, m dusage.Meter, c dusage.Cost, savedPromptTokens int64, savedPromptCost float64) (db.SessionUsageTotal, error) {
    f.lastSaved.meter = m
    f.lastSaved.cost = c
    f.lastSaved.savedT = savedPromptTokens
    f.lastSaved.savedC = savedPromptCost
    return db.SessionUsageTotal{}, nil
}

type fakeEventRepo struct{
    events []dusage.UsageEvent
}

func (f *fakeEventRepo) LogEvent(ctx context.Context, e dusage.UsageEvent) error {
    f.events = append(f.events, e)
    return nil
}
func (f *fakeEventRepo) ListEventsBySession(ctx context.Context, _ pgtype.UUID) ([]dusage.UsageEvent, error) {
    return f.events, nil
}

type fakeDraftRepo struct{}
func (f *fakeDraftRepo) UpsertDraftAgg(ctx context.Context, agg dusage.DraftAgg) error { return nil }
func (f *fakeDraftRepo) GetDraftAggsBySession(ctx context.Context, s pgtype.UUID) ([]dusage.DraftAgg, error) { return nil, nil }
func (f *fakeDraftRepo) UpsertDraftAggStats(ctx context.Context, stats dusage.DraftAggStats) error { return nil }
func (f *fakeDraftRepo) GetDraftAggStats(ctx context.Context, s pgtype.UUID) (dusage.DraftAggStats, error) { return dusage.DraftAggStats{}, nil }
func (f *fakeDraftRepo) DeleteDraftAggs(ctx context.Context, s pgtype.UUID) error { return nil }
// --- test ---

func TestAccumulator_SavingsAndTotals(t *testing.T){
    // Build accumulator with fakes
    sessionID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
    appID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
    accountID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}

    meterRepo := &fakeMeterRepo{}
    eventRepo := &fakeEventRepo{}
    draftRepo := &fakeDraftRepo{}

    ua := &UsageAccumulator{
        meter: dusage.NewMeter(dusage.DefaultPricing),
        meterRepo: meterRepo,
        eventRepo: eventRepo,
        draftAggregator: NewDraftAggregator(sessionID, appID, accountID, draftRepo, false),
        eventChan: make(chan dusage.UsageEvent, 10),
        stopChan: make(chan struct{}),
        doneChan: make(chan struct{}),
        flushInterval: 10 * time.Millisecond,
    }
    ua.meter.SessionID = sessionID
    ua.meter.AppID = appID
    ua.meter.AccountID = accountID

    // Simulate usage: 120s audio, 1000 prompt tokens, 500 completion tokens
    ua.AddSTT(120, "deepgram")
    ua.AddLLM(1000, 500, "gemini", "gemini-2.0-flash")

    // Simulate savings (e.g., cached context saved 800 tokens worth of prompt)
    ua.AddLLMWithSavings(0, 0, 800, "gemini", "gemini-2.0-flash")

    // Flush and validate
    ua.flushTotals(context.Background())

    savedTokens, savedCost := ua.GetSavedPromptTotals()
    if savedTokens != 800 {
        t.Fatalf("expected saved tokens 800, got %d", savedTokens)
    }
    if savedCost <= 0 {
        t.Fatalf("expected saved cost > 0, got %f", savedCost)
    }

    // Repo should receive net cost (total - saved)
    got := meterRepo.lastSaved
    gross := got.cost.AudioCost + got.cost.LLMInCost + got.cost.LLMOutCost + got.cost.CPUCost
    if got.cost.TotalCost != gross - savedCost {
        t.Fatalf("expected net total cost %f, got %f", gross - savedCost, got.cost.TotalCost)
    }
    if got.savedT != 800 || got.savedC != savedCost {
        t.Fatalf("expected saved (800,%f), got (%d,%f)", savedCost, got.savedT, got.savedC)
    }
}


