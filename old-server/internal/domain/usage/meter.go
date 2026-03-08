package usage

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Raw counts gathered during a session
type Meter struct {
	SessionID pgtype.UUID
	AccountID pgtype.UUID // for the authenticated user's session
	AppID     pgtype.UUID

	// accumulators
	StartedAt        time.Time
	AudioSeconds     float64 // STT billed seconds
	PromptTokens     int64   // LLM in
	CompletionTokens int64   // LLM out
	CPUActiveSeconds float64 // fly app
	CPUIdleSeconds   float64

	// pricing ref
	pr Pricing
}

func NewMeter(p Pricing) Meter { return Meter{pr: p, StartedAt: time.Now()} }

// helpers
func (m *Meter) AddSTT(sec float64)           { m.AudioSeconds += sec }
func (m *Meter) AddTokens(p, c int64)         { m.PromptTokens += p; m.CompletionTokens += c }
func (m *Meter) AddCPUActive(d time.Duration) { m.CPUActiveSeconds += d.Seconds() }
func (m *Meter) AddCPUIdle(d time.Duration)   { m.CPUIdleSeconds += d.Seconds() }
func (m *Meter) SetCPUIdle(sec float64)       { m.CPUIdleSeconds = sec }
func (m *Meter) TotalCPUSecs() float64        { return m.CPUActiveSeconds + m.CPUIdleSeconds }

// Immutable* price list (AUD or USD?)
type Pricing struct {
	Currency               string
	CostAudioPerMin        float64 // $/ min for STT
	CostGemPromptPer1M     float64 // $/ token for LLM Input
	CostGemCompletionPer1M float64 // $/ token for LLM Output
	CostFlyPerSec          float64 // $/ second for server runtime
	IdleDiscount           float64 // (e.g. 0.10 = 90% disc)
}

// TODO: update pricing objects for all STT providers. Probably grab their default pricing inside individual infra packages... 
var DefaultPricing = Pricing{
	Currency:               "USD",
	CostAudioPerMin:        0.0077, // This is deepgram, streaming PAYG. 
	CostGemPromptPer1M:     0.15, // Gemini 2.5 flash
	CostGemCompletionPer1M: 0.6, // Gemini 2.5 flash
	CostFlyPerSec:          0.00000095, // https://fly.io/docs/about/pricing/
	IdleDiscount:           0.1, // 10% discount for idle time
}

type Cost struct {
	AudioCost, LLMInCost, LLMOutCost, CPUCost float64
	TotalCost                                 float64
}

// Fast path for “whatever pricing the meter was created with”.
func (m Meter) CostUSD() float64 {
	p := m.pr
	return m.costWith(p).TotalCost
}

// Generic – useful if you ever want to recompute with a new price sheet.
func (m Meter) Cost(p Pricing) Cost { return m.costWith(p) }

func (m Meter) costWith(p Pricing) Cost {
	audio := (m.AudioSeconds / 60) * p.CostAudioPerMin
	inTok := float64(m.PromptTokens) / 1_000_000 * p.CostGemPromptPer1M
	outTok := float64(m.CompletionTokens) / 1_000_000 * p.CostGemCompletionPer1M

	active := m.CPUActiveSeconds * p.CostFlyPerSec
	idle := m.CPUIdleSeconds * p.CostFlyPerSec * p.IdleDiscount

	c := Cost{
		AudioCost:  audio,
		LLMInCost:  inTok,
		LLMOutCost: outTok,
		CPUCost:    active + idle,
	}
	c.TotalCost = c.AudioCost + c.LLMInCost + c.LLMOutCost + c.CPUCost
	return c
}
