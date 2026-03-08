package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/app/prompts"
	"schma.ai/internal/app/usage"
	"schma.ai/internal/domain/speech"
	dbgen "schma.ai/internal/infra/db/generated"
	"schma.ai/internal/pkg/logger"
	pkg_speech "schma.ai/internal/pkg/speech"
)

type StructuredPipeline struct {
	// Deps and conf
	cfg ConfigStructured
	deps Deps
	usageAccumulator *usage.UsageAccumulator

	// Channs
	outTr chan speech.Transcript
	outStructured chan speech.StructuredOutputUpdate

	// Internal
	spellingCache map[string]string // chunk -> spelled names
    // bufferedTr string
	
	// Redaction
	sessionRedactionBuffer *SessionRedactionBuffer
	
	// Session-end aggregation 
	prevStructured map[string]any // latest value, used to build out dynamic prompts.
	revision int64 // updated per llm call to latest rev
	lastLLM time.Time

    // one-time link guard
    schemaLinked bool
    schemaID pgtype.UUID
    
    	// Session-end transcript aggregation (store once at session completion)
    finalTranscripts     []speech.Transcript               // accumulate final transcripts for session
    completePhrases      []speech.Phrase                   // accumulate all phrases from final transcripts
    completePhrasesLight []speech.PhraseLight              // accumulate all phrases from final transcripts
    
    // Redacted structured output tracking (for metrics and storage)
    sessionRedactedStructured []speech.StructuredOutputUpdate  // all redacted structured output for metrics (from LLM)
    latestRedactedStructured  []speech.StructuredOutputUpdate  // latest batch of redacted structured output for metrics
    
    // Silence state tracking for "after-silence" strategy
    inSilence bool

    // LLM context epoch: starting transcript segment index for current config
    epochStartSegmentIdx int
}

// SetDisablePHI updates the PHI redaction toggle without changing epoch
func (p *StructuredPipeline) SetDisablePHI(disable bool) {
	p.cfg.DisablePHI = disable
	logger.ServiceDebugf("S_PIPELINE", "Set DisablePHI=%v", disable)
}

// helper: rebuild structred output
// func (p *Pipeline) reconstructStructuredOutput(sessionID string, output map[string]interface{}) map[string]interface{} {
// 	buffer, exists := p.sessionRedactionBuffer[sessionID]
// 	if !exists {
// 		return output
// 	}

// 	// Convert output to JSON string, reconstruct placeholders, convert back
// 	outputJSON, _ := json.Marshal(output)
// 	reconstructedJSON := p.reconstructText(string(outputJSON), buffer)

// 	var reconstructedOutput map[string]interface{}
// 	json.Unmarshal([]byte(reconstructedJSON), &reconstructedOutput)
// 	return reconstructedOutput
// }


// TODO: generate spelling cache from transcript, same as function pipeline

// TODO: Pipeline is outputting response in Output as 'text: {.. schema}'... we should flatten this into a single object and push maybe?
func NewStructuredPipeline(cfg ConfigStructured, deps Deps) (*StructuredPipeline, error) {
	// Parse UUIDs from config strings
	var dbSessionID pgtype.UUID
	if err := dbSessionID.Scan(cfg.DBSessionID); err != nil {
		return nil, err
	}

	var accountID pgtype.UUID
	if err := accountID.Scan(cfg.AccountID); err != nil {
		return nil, err
	}

	var appID pgtype.UUID
	if err := appID.Scan(cfg.AppID); err != nil {
		return nil, err
	}
	
	// Create usage accumulator for this session (batch mode like functions pipeline)
	usageAccumulator := usage.NewUsageAccumulator(
		dbSessionID,
		appID,
		accountID,
		deps.UsageMeterRepo,
		deps.UsageEventRepo,
		deps.DraftAggRepo,
		true, // batch mode
		"structured", // LLM mode
	)

	return &StructuredPipeline{
		cfg:              cfg,
		deps:             deps,
		outTr:            make(chan speech.Transcript, 128),
		outStructured:    make(chan speech.StructuredOutputUpdate, 64),
		// Initialize redaction buffer
		sessionRedactionBuffer: NewSessionRedactionBuffer(),

		// internal
		usageAccumulator: usageAccumulator,
		spellingCache:    map[string]string{},
		prevStructured:   map[string]any{},
		revision:         0,
		// Initialize transcript aggregation fields
		finalTranscripts:   make([]speech.Transcript, 0),
		completePhrases:    make([]speech.Phrase, 0),
		completePhrasesLight: make([]speech.PhraseLight, 0),
		
		// Initialize redacted structured output tracking
		sessionRedactedStructured: make([]speech.StructuredOutputUpdate, 0),
		latestRedactedStructured:  make([]speech.StructuredOutputUpdate, 0),
		epochStartSegmentIdx: 0,
	}, nil
}

// UsageAccumulator accessor (may be nil until fully wired)
func (p *StructuredPipeline) UsageAccumulator() *usage.UsageAccumulator { return p.usageAccumulator }
func (p *StructuredPipeline) CostUSD() float64 {
	_, cost := p.usageAccumulator.GetCurrentTotals()
	return cost.TotalCost
}

// LatestStructured returns a snapshot of the latest structured object and its revision
func (p *StructuredPipeline) LatestStructured() (rev int64, obj map[string]any) {
    if len(p.prevStructured) == 0 {
        return p.revision, nil
    }
    snap := make(map[string]any, len(p.prevStructured))
    for k, v := range p.prevStructured {
        snap[k] = v
    }
    return p.revision, snap
}

// GetLatestRedactedStructured returns the latest batch of redacted structured output for metrics
func (p *StructuredPipeline) GetLatestRedactedStructured() []speech.StructuredOutputUpdate {
	return p.latestRedactedStructured
}

// ClearLatestRedactedStructured clears the latest batch after metrics tracking
func (p *StructuredPipeline) ClearLatestRedactedStructured() {
	p.latestRedactedStructured = make([]speech.StructuredOutputUpdate, 0)
}

// GetParsingStrategy returns the current parsing strategy
func (p *StructuredPipeline) GetParsingStrategy() string {
	if p.cfg.StructuredCfg != nil {
		return p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy
	}
	return ""
}

// GetAccumulatedTranscript returns the accumulated masked transcript for LLM
func (p *StructuredPipeline) GetAccumulatedTranscript() string {
	return p.sessionRedactionBuffer.getAccumulatedMaskedTranscript(p.cfg.DBSessionID)
}

// SetSilenceState updates the silence state for "after-silence" strategy
func (p *StructuredPipeline) SetSilenceState(inSilence bool) {
	p.inSilence = inSilence
}

// IsInSilence returns the current silence state
func (p *StructuredPipeline) IsInSilence() bool {
	return p.inSilence
}

// StoreFinalStructuredOutput persists the last accumulated structured object for this session
func (p *StructuredPipeline) StoreFinalStructuredOutput(ctx context.Context) {
    if p.deps.StructuredOutputsRepo == nil {
        logger.Warnf("⚠️ [S_PIPELINE] StructuredOutputsRepo is nil; cannot store final structured output")
        return
    }
    // Store complete transcript if we have any final transcripts
    if len(p.finalTranscripts) == 0 {
        logger.ServiceDebugf("S_PIPELINE", "No structured fields accumulated; skipping final store")
        return
    }
    var dbSessionID pgtype.UUID
    if err := dbSessionID.Scan(p.cfg.DBSessionID); err != nil {
        logger.Warnf("⚠️ [S_PIPELINE] invalid session id for structured store: %v", err)
        return
    }

    // Use redacted structured output for storage (not reconstructed)
    // The prevStructured already contains redacted data since we update it with processedObj (redacted from LLM)
    update := speech.StructuredOutputUpdate{Rev: int(p.revision), Final: p.prevStructured}

    
    if p.schemaID.Valid {
        if repoWithExplicit, ok := interface{}(p.deps.StructuredOutputsRepo).(interface{ StoreStructuredOutputWithSchema(ctx context.Context, sessionID, schemaID pgtype.UUID, output speech.StructuredOutputUpdate) (dbgen.StructuredOutput, error) }); ok {
            if row, err := repoWithExplicit.StoreStructuredOutputWithSchema(ctx, dbSessionID, p.schemaID, update); err != nil {
                logger.Warnf("⚠️ [S_PIPELINE] failed to store final structured output (explicit schema): %v (schema_linked=%v)", err, p.schemaLinked)
            } else {
                logger.ServiceDebugf("S_PIPELINE", "💾 stored final structured output id=%s rev=%d (explicit schema)", row.ID.String(), p.revision)
            }
            return
        }
        logger.Warnf("⚠️ [S_PIPELINE] repo does not support explicit schema save; falling back to discovery path")
    }
    if row, err := p.deps.StructuredOutputsRepo.StoreStructuredOutput(ctx, dbSessionID, update); err != nil {
        logger.Warnf("⚠️ [S_PIPELINE] failed to store final structured output (discovered schema): %v (schema_linked=%v)", err, p.schemaLinked)
    } else {
        logger.ServiceDebugf("S_PIPELINE", "💾 stored final structured output id=%s rev=%d (discovered schema)", row.ID.String(), p.revision)
    }
}

// StoreCompleteTranscript stores the aggregated complete transcript at session end
func (p *StructuredPipeline) StoreCompleteTranscript(ctx context.Context) {
	if p.deps.TranscriptsRepo == nil { return }

	maskedAll := strings.TrimSpace(
		p.sessionRedactionBuffer.getAccumulatedMaskedTranscript(p.cfg.DBSessionID),
	)

	if maskedAll == "" {
		return // nothing to persist
	}

	// Create complete transcript
	completeTranscript := speech.Transcript{
		Text:        strings.TrimSpace(maskedAll),
		IsFinal:     true,
		Confidence:  1.0, // Complete transcript has full confidence
		ChunkDurSec: 0,   // Will be calculated from usage data
		Words:       nil,
		Turns:       nil,
		Phrases:     p.completePhrasesLight, 
	}

	var dbSessionID pgtype.UUID
	if dbSessionID.Scan(p.cfg.DBSessionID) == nil {
		if _, err := p.deps.TranscriptsRepo.StoreTranscript(ctx, dbSessionID, completeTranscript); err != nil {
			logger.Warnf("⚠️ [S_PIPELINE] failed to store complete transcript: %v", err)
		} else {
			logger.ServiceDebugf("S_PIPELINE", "💾 stored complete transcript (%d phrases)", 
				len(p.completePhrases),)
		}
	}
}

// StoreSessionData persists aggregated data at session end (structured only)
func (p *StructuredPipeline) StoreSessionData(ctx context.Context) {
	p.StoreCompleteTranscript(ctx)
	p.StoreFinalStructuredOutput(ctx)
	
	// Clear redaction buffers for this session
	if p.sessionRedactionBuffer != nil {
		p.sessionRedactionBuffer.ClearAll()
		logger.ServiceDebugf("S_PIPELINE", "🔍 cleared redaction buffers for session")
	}
}


// Run starts the structured pipeline (skeleton). It will be expanded to wire STT and LLM.
func (p *StructuredPipeline) Run(
	ctx context.Context,
	upstream <-chan speech.AudioChunk,
) (<-chan speech.Transcript, <-chan speech.StructuredOutputUpdate, error) {
	go func() {
		defer close(p.outTr)
		defer close(p.outStructured)

		// Log initial structured configuration (schema + guide)
		if p.cfg.StructuredCfg != nil {
			schemaBytes, _ := json.Marshal(p.cfg.StructuredCfg.Schema)
			logger.ServiceDebugf("S_PIPELINE", "Session start structured cfg: schema_bytes=%d update_ms=%d strategy=%q disable_phi=%v guide_len=%d",
				len(schemaBytes), p.cfg.StructuredCfg.UpdateMs, p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy, p.cfg.DisablePHI, len(p.cfg.StructuredCfg.ParsingGuide))
		} else {
			logger.ServiceDebugf("S_PIPELINE", "Session start: no structured cfg present")
		}

		// 0) Ensure structured schema is stored/linked to this session once
		if !p.schemaLinked && p.deps.StructuredOutputSchemasRepo != nil && p.cfg.StructuredCfg != nil {
            var sid, appID pgtype.UUID
            if err := sid.Scan(p.cfg.DBSessionID); err == nil && appID.Scan(p.cfg.AppID) == nil {
                // Store or get schema, then link to session (repo also attaches on hit)
                if schemaID, err := p.deps.StructuredOutputSchemasRepo.StoreOrGetSchema(ctx, appID, sid, speech.StructuredOutputConfig{
                    UpdateMs: p.cfg.StructuredCfg.UpdateMs,
                    Schema:            p.cfg.StructuredCfg.Schema,
                    ParsingGuide:      p.cfg.StructuredCfg.ParsingGuide,
                }); err != nil {
                    logger.Warnf("⚠️ [S_PIPELINE] failed to store/get schema: %v", err)
                } else {
                    // Link the schema to the session
                    if p.deps.StructuredOutputSchemasRepo != nil {
                        if err := p.deps.StructuredOutputSchemasRepo.LinkSchemaToSession(ctx, sid, schemaID); err != nil {
                            logger.Errorf("❌ [S_PIPELINE] failed to link structured schema to session: %v", err)
                        } else {
                            logger.ServiceDebugf("S_PIPELINE", "linked structured schema to session %s", sid.String())
                        }
                    }
                    p.schemaLinked = true
                    p.schemaID = schemaID
                }
            } else {
                logger.Warnf("⚠️ [S_PIPELINE] invalid session/app id; cannot link schema")
				return
            }
        }

		// 1) Lazy connect to upstream to avoid spinning STT until audio arrives
		_, toSTT, ok := LazyConnect(upstream)
		if !ok {
			logger.Errorf("❌ [S_PIPELINE] upstream closed before any audio")
			return
		}

		// 2) Start STT
		sttOut, err := p.deps.STT.Stream(ctx, toSTT)
		if err != nil {
			logger.Errorf("❌ [S_PIPELINE] stt stream failed: %v", err)
			return
		}

		// 3) Consume STT results
		for tr := range sttOut {
			// Emit interim transcripts immediately
			if !tr.IsFinal {
				p.outTr <- tr
				continue
			}

			// Ignore empty finals
			if tr.Text == "" {
				continue
			}

			// 3.1 Final path: normalize -> redact -> LLM processing -> client emit -> accumulate
			logger.ServiceDebugf("S_PIPELINE", `🔍 final tr hit: %s`, tr.Text)
			
			// Normalize
			nctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			norm, nerr := p.deps.Normalizer.Normalize(nctx, tr.Text)
			cancel() // free up immediately
			
			if nerr != nil {
				logger.Warnf("⚠️ [S_PIPELINE] normalization error, using raw text: %v", nerr)
				norm = tr.Text
			}

			logger.ServiceDebugf("S_PIPELINE", "🔍 normalized text: %s", norm)

			// Redact for Mode B (pre-llm): honor DisablePHI. We still must mask PCI when detected.
			masked := norm
			if p.deps.RedactionService != nil {
				if redactedText, rerr := RedactTranscriptForLLM(p.cfg.DBSessionID, norm, p.deps, p.sessionRedactionBuffer, p.cfg.DisablePHI); rerr != nil {
					logger.Errorf("❌ [S_PIPELINE] phi redaction failed: %v", rerr)
				} else {
					masked = redactedText
				}
			} else {
				// PHI redaction disabled or unavailable -> still append a plain segment so accumulation works
				p.sessionRedactionBuffer.appendPlainSegment(p.cfg.DBSessionID, norm)
				if p.cfg.DisablePHI {
					logger.ServiceDebugf("S_PIPELINE", "PHI redaction disabled; sending unredacted text to LLM (PCI masking handled separately)")
				}
			}

			logger.ServiceDebugf("S_PIPELINE", "🔍 redacted text: %s", masked)

			// Emit to Client, normalized UI event (do not send masked to client)
			disp := speech.Transcript{
				Text: norm, // normalized for UI (not redacted)
				IsFinal: true,
				Words: tr.Words,
				Turns: tr.Turns,
			}

			p.outTr <- disp

			// Build Phrases, from timing of final chunk
			// NOTE: helper returns multiple phrases per final, split by speaker boundaries
			var phs []speech.Phrase
			var pls []speech.PhraseLight
			if len(tr.Turns) > 0 {
				// Diarization-aware phrase grouping by turns
				normByTurn := map[string]string{}
				maskedByTurn := map[string]string{}
				for _, t := range tr.Turns { normByTurn[t.ID] = norm; maskedByTurn[t.ID] = masked }
				phs, pls = pkg_speech.BuildPhrasesBySpeakerGroups(tr.Words, tr.Turns, normByTurn, maskedByTurn)
			} else {
				phs, pls = pkg_speech.BuildPhrasesFromFinalWords(tr.Words, norm, masked)
			}

			logger.ServiceDebugf("S_PIPELINE", "🔍 phrases: %v", phs)

			// Aggregate for storage + LLM
			p.finalTranscripts = append(p.finalTranscripts, tr)
			p.completePhrases = append(p.completePhrases, phs...)
			p.completePhrasesLight = append(p.completePhrasesLight, pls...)

			// Build diarized paragraphs text for LLM (latest chunk)
			diarizedParas := prompts.BuildDiarizedParagraphs(pls)
			
			// Downstream, use masked for LLM
			tr.Text = masked


			// Only call LLM on final chunks and if configured
			if !tr.IsFinal || p.deps.LLM == nil || p.cfg.StructuredCfg == nil {
				continue
			}

			// Check if we should call LLM based on parsing strategy
			if !p.shouldCallLLM(tr.IsFinal) {
				continue
			}

			// Throttle - only apply throttling for "auto" and "update-ms" strategies
			// "after-silence" and "end-of-session" strategies are handled externally
			if p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy == "auto" || p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy == "update-ms" {
				if gap := time.Duration(p.cfg.StructuredCfg.UpdateMs) * time.Millisecond; gap > 0 && time.Since(p.lastLLM) < gap {
					logger.ServiceDebugf("S_PIPELINE", "throttling structured output: gap=%dms, time_since_last=%dms", gap.Milliseconds(), time.Since(p.lastLLM).Milliseconds())
					continue
				}
			}

			p.lastLLM = time.Now()
			logger.ServiceDebugf("S_PIPELINE", "making structured output request: update_frequency=%dms", p.cfg.StructuredCfg.UpdateMs)

			// Prepare structured LLM config
			schemaBytes, _ := json.Marshal(p.cfg.StructuredCfg.Schema)
			
			sCfg := &speech.StructuredConfig{
				Schema:       schemaBytes, 
				ParsingGuide: p.cfg.StructuredCfg.ParsingGuide, // Use the original parsing guide for caching
				UpdateMS:     p.cfg.StructuredCfg.UpdateMs,
			}

			logger.ServiceDebugf("S_PIPELINE", "System prompt (structured) guide_len=%d", len(sCfg.ParsingGuide))

			// Create masked previous structured output for LLM (filter against current schema and avoid sending PHI)
			if p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode == "apply" {
				logger.Infof("S_PIPELINE - Applying previous structured output inclusion policy: %v", p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode)
				filteredPrev := p.filterPrevStructuredBySchema(p.prevStructured)
				maskedPrevStructured := filteredPrev
				if p.deps.RedactionService != nil && len(filteredPrev) > 0 {
					maskedPrevStructured = p.sessionRedactionBuffer.maskStructuredOutput(p.cfg.DBSessionID, filteredPrev)
					maskedPrevStructuredMarshalled, _ := json.Marshal(maskedPrevStructured)
					logger.ServiceDebugf("S_PIPELINE", "🔍 masked previous structured output for LLM: %v", string(maskedPrevStructuredMarshalled))
				}
			} else if p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode == "ignore" {
				logger.Infof("S_PIPELINE - previous structured should not be applied according to inclusion policy: %v", p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode)
				logger.ServiceDebugf("S_PIPELINE", "Prev output inclusion disabled; not sending previous structured output in prompt")
			}
			
			// Build minimal user prompt: move instructions to system; only send transcript
			// Use full accumulated masked transcript to provide context
			baseContext := p.GetTranscriptSinceConfigEpoch()
			if baseContext == "" { baseContext = tr.Text }
			// Compose diarized latest + context BEFORE windowing, preserving speaker headers
			combined := baseContext
			if len(diarizedParas) > 0 {
				diarizedBlock := prompts.RenderDiarizedParagraphs(diarizedParas)
				if p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode == "window" && p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize > 0 {
					maxTok := p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize
					diarToks := len(strings.Fields(diarizedBlock))
					if diarToks >= maxTok {
						diarizedBlock = prompts.WindowDiarizedParagraphsTokens(diarizedParas, maxTok)
						combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock
					} else {
						tailNeeded := maxTok - diarToks
						combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock + "\n\nContext (previous masked transcript):\n" + windowLastTokens(baseContext, tailNeeded)
					}
				} else {
					combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock + "\n\nContext (previous masked transcript):\n" + baseContext
				}
			} else {
				// No diarized latest -> optionally window context
				if p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode == "window" && p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize > 0 {
					combined = windowLastTokens(baseContext, p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize)
				} else {
					combined = baseContext
				}
			}
			var prompt string = combined

			logger.Infof("S_PIPELINE - Dynamic prompt chars=%d tokens=%d", len(prompt), len(strings.Fields(prompt)))
			logger.Infof("S_PIPELINE - Dynamic prompt: %s", prompt)


			// Try cached structured LLM first, fallback to regular structured LLM
			var obj map[string]any
			var u *speech.LLMUsage
			var err error

			// Force schema-constrained path by preferring the regular structured generation
			if structuredLLM, ok := p.deps.LLM.(speech.StructuredLLM); ok {
				logger.ServiceDebugf("S_PIPELINE", "Using regular structured generation: prompt_len=%d guide_len=%d update_ms=%d", len(prompt), len(p.cfg.StructuredCfg.ParsingGuide), p.cfg.StructuredCfg.UpdateMs)
				obj, u, err = structuredLLM.GenerateStructured(ctx, speech.Prompt(prompt), tr, sCfg)
				logger.ServiceDebugf("S_PIPELINE", "🔍 structured output: %v", obj)
			} else {
				// Track LLM error for metrics and analytics
				provider := "gemini"
				model := "gemini-2.0-flash"
				p.usageAccumulator.AddLLMError(provider, model, "structured_not_supported", "LLM does not support structured generation")
				logger.Warnf("⚠️ [S_PIPELINE] llm does not support structured generation (impl=%T)", p.deps.LLM)
				continue
			}
			
			// Add retry logic for Gemini 500 errors
			if err != nil && strings.Contains(err.Error(), "500") {
				logger.Warnf("⚠️ [S_PIPELINE] gemini 2.0 flash 500 error with structured output: %v", err)
				// Could implement exponential backoff here
				continue
			}
			if err != nil {
				// Track LLM error for metrics and analytics
				provider := "gemini"
				model := "gemini-2.0-flash"
				p.usageAccumulator.AddLLMError(provider, model, "structured_generation_failed", err.Error())
				logger.Errorf("❌ [S_PIPELINE] failed to generate structured output: %v", err)
				continue
			}

			logger.ServiceDebugf("S_PIPELINE", "🔍 structured output: %v", obj)

			// Post-process the LLM response to extract actual JSON if nested
			processedObj := p.extractStructuredJSON(obj)
			if processedObj == nil {
				logger.Warnf("⚠️ [S_PIPELINE] failed to extract structured JSON from LLM response")
				continue
			}

			// Record LLM usage (handle cached vs non-cached)
			if u != nil {
				provider := "gemini"
				model := "gemini-2.0-flash"
				if u.Cached && u.SavedPromptTokens > 0 {
					p.usageAccumulator.AddLLMWithSavings(u.Prompt, u.Completion, u.SavedPromptTokens, provider, model)
				} else {
					p.usageAccumulator.AddLLM(u.Prompt, u.Completion, provider, model)
				}
			}

			// Compute shallow delta vs previous
			delta := computeShallowDelta(p.prevStructured, processedObj)
			if len(delta) == 0 {
				continue
			}
			deltaMarshalled, _ := json.Marshal(delta)
			logger.ServiceDebugf("S_PIPELINE", "🔍 structured output delta: %v", string(deltaMarshalled))

			p.revision++
			
			// Store redacted structured output for metrics and database storage
			redactedUpdate := speech.StructuredOutputUpdate{Rev: int(p.revision), Delta: delta}
			p.sessionRedactedStructured = append(p.sessionRedactedStructured, redactedUpdate)
			p.latestRedactedStructured = []speech.StructuredOutputUpdate{redactedUpdate} // Store latest batch for metrics
			
			// Reconstruct structured output for client (replace placeholders with original values)
			reconstructedDelta := p.sessionRedactionBuffer.reconstructStructuredOutput(p.cfg.DBSessionID, delta)
			reconstructedUpdate := speech.StructuredOutputUpdate{Rev: int(p.revision), Delta: reconstructedDelta}

			reconstructedMarshalled, _ := json.Marshal(reconstructedUpdate)
			logger.ServiceDebugf("S_PIPELINE", "🔍 sending structured output update: rev=%d, delta_keys=%v, final_keys=%v (reconstructed for client): %v", p.revision, getMapKeys(reconstructedDelta), getMapKeys(p.prevStructured), string(reconstructedMarshalled))
		
			p.outStructured <- reconstructedUpdate

			// Update previous snapshot (use redacted for internal tracking)
			p.prevStructured = shallowMerge(p.prevStructured, processedObj)
		}
	}()
	return p.outTr, p.outStructured, nil
}

// computeShallowDelta returns a map of keys whose values changed or were added
func computeShallowDelta(prev, next map[string]any) map[string]any {
	d := map[string]any{}
	for k, v := range next {
		if pv, ok := prev[k]; !ok || !jsonDeepEqual(pv, v) {
			d[k] = v
		}
	}
	return d
}

func jsonDeepEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

// filterPrevStructuredBySchema returns a shallow copy of prev containing only keys
// that are declared in the current structured schema (depth-1 properties).
func (p *StructuredPipeline) filterPrevStructuredBySchema(prev map[string]any) map[string]any {
	if len(prev) == 0 || p.cfg.StructuredCfg == nil {
		return nil
	}
	props := p.cfg.StructuredCfg.Schema.Properties
	if len(props) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(props))
	for k := range props {
		allowed[k] = struct{}{}
	}
	out := make(map[string]any)
	for k, v := range prev {
		if _, ok := allowed[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// extractStructuredJSON handles LLM responses that might be nested in a "final" -> "text" structure
func (p *StructuredPipeline) extractStructuredJSON(obj map[string]any) map[string]any {
	if obj == nil {
		return nil
	}

	logger.ServiceDebugf("S_PIPELINE", "processing LLM response structure: %v", obj)

	// Check if the response has the nested structure: {"final": {"text": "..."}}
	if final, ok := obj["final"].(map[string]any); ok {
		if text, ok := final["text"].(string); ok {
			// Try to parse the text as JSON
			var parsed map[string]any
			if err := json.Unmarshal([]byte(text), &parsed); err == nil {
				logger.ServiceDebugf("S_PIPELINE", "extracted nested JSON from final.text: %v", parsed)
				return parsed
			} else {
				logger.Warnf("⚠️ [S_PIPELINE] failed to parse final.text as JSON: %v", err)
				logger.ServiceDebugf("S_PIPELINE", "Raw final.text content: %q", text)
			}
		}
	}	

	// Check if the response has a direct "text" field
	if text, ok := obj["text"].(string); ok {
		// Try to parse the text as JSON
		var parsed map[string]any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			logger.ServiceDebugf("S_PIPELINE", "extracted JSON from direct text field: %v", parsed)
			return parsed
		} else {
			logger.Warnf("⚠️ [S_PIPELINE] failed to parse direct text as JSON: %v", err)
		}
	}

	// If the object looks like it already contains the schema fields, return as-is
	// Check for common schema field names to determine if this is already the structured data
	schemaFieldCount := 0
	for key := range obj {
		// Skip metadata fields that are not part of the schema
		if key != "final" && key != "text" && key != "rev" && key != "revision" {
			schemaFieldCount++
		}
	}
	
	if schemaFieldCount > 0 {
		logger.ServiceDebugf("S_PIPELINE", "using object as-is (contains %d schema fields): %v", schemaFieldCount, obj)
		return obj
	}

	// If we get here, the object doesn't contain expected schema fields
	logger.Warnf("⚠️ [S_PIPELINE] llm response doesn't contain expected schema structure: %v", obj)
	return nil
}

// shallowMerge copies all keys from src into dst (1-level deep) and returns dst.
// If dst is nil, a new map is created. Values in src overwrite those in dst.
func shallowMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	if src == nil {
		return dst
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// TODO: just dynamic prompt builders to handle diarization. If we have diarization, group by speaker (use turns util).

// shouldCallLLM determines if we should call the LLM based on the parsing strategy
func (p *StructuredPipeline) shouldCallLLM(isFinal bool) bool {
	if p.cfg.StructuredCfg == nil {
		return false
	}

	switch p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy {
	case "auto":
		// Auto strategy: call on final transcripts, respecting UpdateMS throttling
		return isFinal
	case "update-ms":
		// Update-ms strategy: call on final transcripts, respecting UpdateMS throttling
		return isFinal
	case "manual":
		// Manual strategy: only call when explicitly triggered
		return false
	case "after-silence":
		// after-silence strategy: only call when silence is detected
		// This will be handled by the silence service integration
		return false // We'll handle this in the silence service callback
	case "end-of-session":
		// End-of-session strategy: don't call during streaming, only at session end
		return false
	default:
		// Default to auto behavior
		return isFinal
	}
}

// ForceLLMCall forces an LLM call regardless of parsing strategy (used for silence and end-of-session)
func (p *StructuredPipeline) ForceLLMCall(ctx context.Context, transcriptForLLM string) error {
	logger.ServiceDebugf("S_PIPELINE", "forceLLMCall called with transcript length: %d, strategy: %s", len(transcriptForLLM), p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy)
	logger.Infof("S_PIPELINE - forceLLMCall called with transcript length: %d, strategy: %s", len(transcriptForLLM), p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy)
	if p.deps.LLM == nil || p.cfg.StructuredCfg == nil {
		logger.Errorf("🔇 [S_PIPELINE] forceLLMCall failed - LLM or structured config not available")
		return fmt.Errorf("LLM or structured config not available")
	}

	// For "after-silence" strategy, prevent multiple calls during the same silence period
	if p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy == "after-silence" && p.inSilence {
		logger.Debugf("🔇 [S_PIPELINE] skipping LLM call - already processed this silence period")
		return nil
	}

	// Apply transcript windowing policy if configured
	pol := p.cfg.StructuredCfg.ParsingConfig.TranscriptInclusionPolicy
	if pol.TranscriptMode == "window" && pol.WindowTokenSize > 0 {
		before := len(transcriptForLLM)
		transcriptForLLM = windowLastTokens(transcriptForLLM, pol.WindowTokenSize)
		logger.ServiceDebugf("S_PIPELINE", "ForceLLMCall: applied transcript windowing policy size=%d chars_before=%d chars_after=%d", pol.WindowTokenSize, before, len(transcriptForLLM))
	}

	// Create masked previous structured output for LLM (filtered to current schema)
	var maskedPrevStructured map[string]any
	if p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode == "apply" {
		filteredPrev := p.filterPrevStructuredBySchema(p.prevStructured)
		maskedPrevStructured = filteredPrev
		if p.deps.RedactionService != nil && len(filteredPrev) > 0 {
			maskedPrevStructured = p.sessionRedactionBuffer.maskStructuredOutput(p.cfg.DBSessionID, filteredPrev)
		}
	} else if p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode == "ignore" {
		logger.Infof("S_PIPELINE - previous structured should not be applied according to inclusion policy: %v", p.cfg.StructuredCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode)
		maskedPrevStructured = nil
		logger.ServiceDebugf("S_PIPELINE", "ForceLLMCall: prev output inclusion disabled; not sending previous structured output")
	}

	maskedPrevStructuredMarshalled, _ := json.Marshal(maskedPrevStructured)
	logger.ServiceDebugf("S_PIPELINE", "forceLLMCall called with masked prev structured: %v", string(maskedPrevStructuredMarshalled))

	// Build minimal user prompt: system carries instructions; only send transcript
	var prompt string = transcriptForLLM
	

	// Create a dummy transcript for the LLM call
	dummyTr := speech.Transcript{
		Text:    transcriptForLLM,
		IsFinal: true,
	}

	// Prepare structured LLM config
	schemaBytes, _ := json.Marshal(p.cfg.StructuredCfg.Schema)
	sCfg := &speech.StructuredConfig{
		Schema:       schemaBytes,
		ParsingGuide: p.cfg.StructuredCfg.ParsingGuide,
		UpdateMS:     p.cfg.StructuredCfg.UpdateMs,
	}

	// Prefer schema-constrained path
	var obj map[string]any
	var u *speech.LLMUsage
	var err error
	if structuredLLM, ok := p.deps.LLM.(speech.StructuredLLM); ok {
		obj, u, err = structuredLLM.GenerateStructured(ctx, speech.Prompt(prompt), dummyTr, sCfg)
	} else if cachedStructuredLLM, ok := p.deps.LLM.(interface {
		GenerateStructuredWithOptimalStrategy(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error)
	}); ok {
		obj, u, err = cachedStructuredLLM.GenerateStructuredWithOptimalStrategy(ctx, speech.Prompt(prompt), dummyTr, sCfg)
	} else {
		return fmt.Errorf("llm does not support structured generation")
	}

	if err != nil {
		return fmt.Errorf("failed to generate structured output: %w", err)
	}

	// Process the results
	processedObj := p.extractStructuredJSON(obj)
	if processedObj == nil {
		return fmt.Errorf("failed to extract structured JSON from llm response")
	}

	// Compute shallow delta vs previous
	delta := computeShallowDelta(p.prevStructured, processedObj)
	if len(delta) == 0 {
		return nil // No changes
	}

	p.revision++

	// Store redacted structured output for metrics
	redactedUpdate := speech.StructuredOutputUpdate{Rev: int(p.revision), Delta: delta}
	p.sessionRedactedStructured = append(p.sessionRedactedStructured, redactedUpdate)
	p.latestRedactedStructured = []speech.StructuredOutputUpdate{redactedUpdate}

	// Reconstruct structured output for client
	reconstructedDelta := p.sessionRedactionBuffer.reconstructStructuredOutput(p.cfg.DBSessionID, delta)
	reconstructedUpdate := speech.StructuredOutputUpdate{Rev: int(p.revision), Delta: reconstructedDelta}

	logger.ServiceDebugf("S_PIPELINE", "forceLLMCall called with reconstructed delta: %v", reconstructedDelta)
	reconstructedMarshalled, _ := json.Marshal(reconstructedUpdate)
	logger.ServiceDebugf("S_PIPELINE", "forceLLMCall called with reconstructed update: %v", string(reconstructedMarshalled))

	// Send to output channel
	select {
	case p.outStructured <- reconstructedUpdate:
	default:
		logger.Warnf("⚠️ [S_PIPELINE] Structured output channel full, dropping forced LLM call results")
	}

	// Record usage
	if u != nil {
		provider := "gemini"
		model := "gemini-2.0-flash"
		if u.Cached && u.SavedPromptTokens > 0 {
			p.usageAccumulator.AddLLMWithSavings(u.Prompt, u.Completion, u.SavedPromptTokens, provider, model)
		} else {
			p.usageAccumulator.AddLLM(u.Prompt, u.Completion, provider, model)
		}
	}

	// Merge into previous snapshot so session-end persistence has full state
	p.prevStructured = shallowMerge(p.prevStructured, processedObj)
	
	// For "after-silence" strategy, mark that we've processed this silence period
	if p.cfg.StructuredCfg.ParsingConfig.ParsingStrategy == "after-silence" {
		p.inSilence = true
		logger.Debugf("🔇 [S_PIPELINE] Marked silence period as processed - will skip future calls until audio resumes")
	}

	return nil
}

// GetTranscriptSinceConfigEpoch returns the masked transcript from the last config-update epoch
func (p *StructuredPipeline) GetTranscriptSinceConfigEpoch() string {
	return p.sessionRedactionBuffer.getMaskedTranscriptFromSegment(p.cfg.DBSessionID, p.epochStartSegmentIdx)
}

// UpdateStructuredConfig updates structured config at runtime and optionally preserves context
func (p *StructuredPipeline) UpdateStructuredConfig(newCfg *speech.StructuredOutputConfig, preserveContext bool) {
	p.cfg.StructuredCfg = newCfg
	if preserveContext {
		// keep epoch start at 0 to include all history
		p.epochStartSegmentIdx = 0
		// prune previous structured to keys allowed by new schema
		p.prevStructured = p.filterPrevStructuredBySchema(p.prevStructured)
	} else {
		// move epoch to current end so new schema sees only future segments
		p.epochStartSegmentIdx = p.sessionRedactionBuffer.getSegmentCount(p.cfg.DBSessionID)
		// clear prev structured so deltas/merges start fresh for new schema
		p.prevStructured = map[string]any{}
	}
	logger.ServiceDebugf("S_PIPELINE", "Updated structured config (preserve=%v) epochStart=%d", preserveContext, p.epochStartSegmentIdx)
}

// parseUUID is a small helper to convert string to pg UUID with zero fallback
// func parseUUID(s string) (id pgtype.UUID) { _ = id.Scan(s); return }