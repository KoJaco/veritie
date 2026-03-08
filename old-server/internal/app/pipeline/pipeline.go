package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/app/draft"
	"schma.ai/internal/app/prompts"
	"schma.ai/internal/app/spelling"
	"schma.ai/internal/app/usage"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/fnutil"
	"schma.ai/internal/pkg/logger"
	pkg_speech "schma.ai/internal/pkg/speech"
)

// Real-time flow: Audio -> STT -> Parser -> LLM
// TODO: move to domain, define interface and then implement here...
type Pipeline struct {
	cfg              ConfigFunctions
	deps             Deps
	usageAccumulator *usage.UsageAccumulator

	outTr     chan speech.Transcript
	outFns    chan []speech.FunctionCall
	outDrafts chan speech.FunctionCall

	// Internal
	knownDraft    map[string]float64 // name -> best-similarity so far
	spellingCache map[string]string  // chunk -> spelled names
	bufferedTr    string             // buffered transcript text to build out spelling cache on each new stt res

	// Redaction
	sessionRedactionBuffer *SessionRedactionBuffer

	// Session-end aggregation (store once at session completion)
	finalTranscripts     []speech.Transcript               // accumulate final transcripts for session
	// completeTranscriptMasked   string                            // concatenated masked+normalized text 
	completePhrases 	 []speech.Phrase // accumulate all phrases from final transcripts
	completePhrasesLight []speech.PhraseLight // accumulate all phrases from final transcripts
	// completeWords        []speech.Word                     // accumulate all words from final transcripts
	// completeTurns        []speech.Turn                     // accumulate all turns from final transcripts
	sessionFunctionCalls map[string]map[string]interface{} // function_name -> latest_redacted_args (for storage)
	sessionRedactedCalls []speech.FunctionCall             // all redacted calls for metrics (from LLM)
	latestRedactedCalls  []speech.FunctionCall             // latest batch of redacted calls for metrics

    	// Previous LLM calls used to build dynamic prompt context
    prevCalls []speech.FunctionCall

    // LLM context epoch: starting transcript segment index for current config
    epochStartSegmentIdx int
	
	// Silence state tracking for "after-silence" strategy
	inSilence bool
}



// Accessors
func (p *Pipeline) UsageAccumulator() *usage.UsageAccumulator { return p.usageAccumulator }
func (p *Pipeline) CostUSD() float64 {
	_, cost := p.usageAccumulator.GetCurrentTotals()
	return cost.TotalCost
}

// GetLatestRedactedFunctionCalls returns the latest batch of redacted function calls for metrics
func (p *Pipeline) GetLatestRedactedFunctionCalls() []speech.FunctionCall {
	return p.latestRedactedCalls
}

// ClearLatestRedactedFunctionCalls clears the latest batch after metrics tracking
func (p *Pipeline) ClearLatestRedactedFunctionCalls() {
	p.latestRedactedCalls = make([]speech.FunctionCall, 0)
}

// GetParsingStrategy returns the current parsing strategy
func (p *Pipeline) GetParsingStrategy() string {
	if p.cfg.FuncCfg != nil {
		return p.cfg.FuncCfg.ParsingConfig.ParsingStrategy
	}
	return ""
}

// GetAccumulatedTranscript returns the accumulated masked transcript for LLM
func (p *Pipeline) GetAccumulatedTranscript() string {
	return p.sessionRedactionBuffer.getAccumulatedMaskedTranscript(p.cfg.DBSessionID)
}

// GetTranscriptSinceConfigEpoch returns the masked transcript from the last config-update epoch
func (p *Pipeline) GetTranscriptSinceConfigEpoch() string {
	return p.sessionRedactionBuffer.getMaskedTranscriptFromSegment(p.cfg.DBSessionID, p.epochStartSegmentIdx)
}

// SetSilenceState updates the silence state for "after-silence" strategy
func (p *Pipeline) SetSilenceState(inSilence bool) {
	p.inSilence = inSilence
}

// IsInSilence returns the current silence state
func (p *Pipeline) IsInSilence() bool {
	return p.inSilence
}


// New constructs a Pipeline and returns it *before any goroutines start... handler decides when to run
func New(cfg ConfigFunctions, deps Deps) (*Pipeline, error) {
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

	// Create usage accumulator for this session (with batch mode enabled)
    usageAccumulator := usage.NewUsageAccumulator(
		dbSessionID,
		appID,
		accountID,
		deps.UsageMeterRepo,
		deps.UsageEventRepo,
		deps.DraftAggRepo,
		true, // Enable batch mode - write all data at session end
		"functions", // LLM mode
	)

    if usageAccumulator == nil {
        logger.Warnf("⚠️ [PIPELINE] usage accumulator is nil (dbSessionID=%s)", cfg.DBSessionID)
    }

	return &Pipeline{
		cfg:              cfg,
		deps:             deps,
		outTr:            make(chan speech.Transcript, 128),
		outFns:           make(chan []speech.FunctionCall, 64),
		outDrafts:        make(chan speech.FunctionCall, 64),
		knownDraft:       make(map[string]float64),
		usageAccumulator: usageAccumulator,
		spellingCache:    make(map[string]string),
		bufferedTr:       "",
		// Initialize redaction buffer
		sessionRedactionBuffer: NewSessionRedactionBuffer(),
		// Initialize aggregation fields
		finalTranscripts:     make([]speech.Transcript, 0),
		// completeTranscriptMasked:   "", // TODO: remove after testing, masked available from sessionRedactionBuffer.getAccumulatedMaskedTranscript
		completePhrases:      make([]speech.Phrase, 0),
		completePhrasesLight: make([]speech.PhraseLight, 0),
		// completeWords:        make([]speech.Word, 0),
		// completeTurns:        make([]speech.Turn, 0),
		sessionFunctionCalls: make(map[string]map[string]interface{}),
		sessionRedactedCalls: make([]speech.FunctionCall, 0),
		latestRedactedCalls:  make([]speech.FunctionCall, 0),
		epochStartSegmentIdx: 0,
	}, nil
}

// Run begins stt stream, pushes transcripts into p.out

func (p *Pipeline) Run(
	ctx context.Context,
	upstream <-chan speech.AudioChunk,
) (<-chan speech.Transcript, <-chan []speech.FunctionCall, <-chan speech.FunctionCall, error) {
	go func() {

		defer close(p.outTr)
		defer close(p.outFns)
		defer close(p.outDrafts)

        // Initialize previous calls context
        p.prevCalls = []speech.FunctionCall{}

		// Lazy connect - wait for first chunk *note, first is already in toSTT buffer (no audio lost)
		_, toSTT, ok := LazyConnect(upstream)
		if !ok {
			logger.Errorf("❌ [PIPELINE] Upstream closed before any audio")
			return
		}

		// 1. Start STT
		sttStartTime := time.Now()
		sttOut, err := p.deps.STT.Stream(ctx, toSTT)
		if err != nil {
			return
		}
		logger.ServiceDebugf("PIPELINE", "TIMING: STT stream started at %s",
			sttStartTime.Format("15:04:05.000"))

		lastLLM := time.Time{}

		loggedNoRedaction := false

		// 2. Consume STT results
		for tr := range sttOut {
		
			// 2.1 Pre-emit, gather spelling cache and buffer transcript
			p.bufferedTr += " " + tr.Text

			const maxBuffer = 5000
			if len(p.bufferedTr) > maxBuffer {
				p.bufferedTr = p.bufferedTr[len(p.bufferedTr)-maxBuffer:]
			}

			// populate spelling cache before publishing tr
			names := spelling.ExtractSpelledNames(p.bufferedTr)
			for lower, proper := range names {
				p.spellingCache[lower] = proper
				logger.ServiceDebugf("PIPELINE", "Spelling cache: %s -> %s", lower, proper)
			}

			// Draft Detection, detect for interim results
			// TODO: decide if this should detect on final path too
			if !tr.IsFinal && p.cfg.DraftIndex != nil {
				if d := p.cfg.DraftIndex.Detect(tr.Text); d != nil {
					best, seen := p.knownDraft[d.Name]
					// Send only if new or strictly better

					if !seen || d.SimilarityScore > best {
						p.knownDraft[d.Name] = d.SimilarityScore
						logger.ServiceDebugf("PIPELINE", "Sending draft: %s", d.Name)

						p.outDrafts <- *d
					} else {
						logger.ServiceDebugf("PIPELINE", "Draft skipped name=%s new=%.2f best=%.2f",
							d.Name, d.SimilarityScore, best)
					}
				}
			}


			// 2.2 Interim & Exclusion path, Emit once per result:

			if !tr.IsFinal {
				// INTERIM -> raw to the client
				p.outTr <- tr
				continue
			}

			// Ignore empty finals
			if tr.Text == "" {
				continue
			}

			// 2.3 Final path, normalize -> redact -> phrases -> client emit for final -> accumulate
			logger.ServiceDebugf("PIPELINE", "final tr hit: %s", tr.Text)
			
			// Normalize
			nctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			norm, nerr := p.deps.Normalizer.Normalize(nctx, tr.Text)
			cancel() // free up immediately

			logger.ServiceDebugf("PIPELINE", "normalized: %v", norm)
			
			if nerr != nil {
				logger.Warnf("⚠️ [PIPELINE] normalization error, using raw text: %v", nerr)
				norm = tr.Text
			}

			logger.ServiceDebugf("PIPELINE", "normalized text: %s", norm)

			// Run Draft Detection on Norm
			if p.cfg.DraftIndex != nil {
				if d := p.cfg.DraftIndex.Detect(norm); d != nil {
					best, seen := p.knownDraft[d.Name]
					// Send only if new or strictly better

					if !seen || d.SimilarityScore > best {
						p.knownDraft[d.Name] = d.SimilarityScore
						logger.ServiceDebugf("PIPELINE", "sending draft: %s", d.Name)

						p.outDrafts <- *d
					} else {
						logger.ServiceDebugf("PIPELINE", "draft skipped name=%s new=%.2f best=%.2f",
							d.Name, d.SimilarityScore, best)
					}
				}
			}

			// Redact for Mode B (pre-llm), redacted -> llm, logs, storage, unredacted -> client SDK, draft detection
			masked := norm
		
			if p.deps.RedactionService == nil {
				if !loggedNoRedaction {
					logger.ServiceDebugf("PIPELINE", "no redaction service configured, skipping PHI redaction")
					loggedNoRedaction = true
				}
			} else {
				if redactedText, rerr := RedactTranscriptForLLM(p.cfg.DBSessionID, norm, p.deps, p.sessionRedactionBuffer, p.cfg.DisablePHI); rerr != nil {
					logger.Errorf("❌ [PIPELINE] phi redaction failed: %v", rerr)
				} else {
					masked = redactedText
				}
			}

			logger.ServiceDebugf("PIPELINE", "redacted text: %s", masked)
		
			// Build Phrase(s)
			var diarizedParas []prompts.DiarizedParagraph
			if len(tr.Turns) > 0 {
				// Group by diarization turns; assign full text to each turn by default
				normByTurn := map[string]string{}
				maskedByTurn := map[string]string{}
				for _, t := range tr.Turns { normByTurn[t.ID] = norm; maskedByTurn[t.ID] = masked }
				phs, pls := pkg_speech.BuildPhrasesBySpeakerGroups(tr.Words, tr.Turns, normByTurn, maskedByTurn)
				// Emit a display transcript aggregating the latest phrases for UI
				p.outTr <- speech.Transcript{ Text: norm, IsFinal: true, PhrasesDisplay: phs, Turns: tr.Turns }
				// Accumulate
				p.completePhrases = append(p.completePhrases, phs...)
				p.completePhrasesLight = append(p.completePhrasesLight, pls...)
				// Build diarized paragraphs for LLM (latest chunk)
				diarizedParas = prompts.BuildDiarizedParagraphs(pls)
			} else {
				// Multiple phrases from final words, split by speaker boundaries
				phs, pls := pkg_speech.BuildPhrasesFromFinalWords(tr.Words, norm, masked)
				logger.ServiceDebugf("PIPELINE", "phrases: %v", phs)
				// Emit to Client, normalized UI event (do not send masked to client)
				disp := speech.Transcript{
					Text: norm, // normalized for UI
					IsFinal: true,
					PhrasesDisplay: phs,
					Turns: []speech.Turn{}, // empty turns since we're using phrases
				}
				p.outTr <- disp
				// Accumulate
				p.completePhrases = append(p.completePhrases, phs...)
				p.completePhrasesLight = append(p.completePhrasesLight, pls...)
				// Build diarized paragraphs for LLM (latest chunk)
				diarizedParas = prompts.BuildDiarizedParagraphs(pls)
			}
			
			// buffer for LLM, 
			transcriptForLLM := p.GetTranscriptSinceConfigEpoch()
			if transcriptForLLM == "" {
				transcriptForLLM = masked 
			}

			// Downstream, use masked for LLM
			tr.Text = masked

			
			// 2.4. Slots
			// 3.3 Call LLM only for final utterances and if we actually have an adapter
			if p.deps.LLM != nil && p.cfg.FuncCfg != nil {

				// Check if we should call LLM based on parsing strategy
				if !p.shouldCallLLM(tr.IsFinal) {
					continue
				}

				// ╭─ throttle (simple window) ──────────────────────────────╮
				// Only apply throttling for "auto" and "update-ms" strategies
				// "after-silence" and "end-of-session" strategies are handled externally
				if p.cfg.FuncCfg.ParsingConfig.ParsingStrategy == "auto" || p.cfg.FuncCfg.ParsingConfig.ParsingStrategy == "update-ms" {
					if gap := time.Duration(p.cfg.FuncCfg.UpdateMs) * time.Millisecond; gap > 0 && time.Since(lastLLM) < gap {
						continue
					}
				}
				lastLLM = time.Now()
				// ╰─────────────────────────────────────────────────────────╯

				// TODO: smarter debounce strategy (token diff, cost caps, etc.)

				// Create masked previous function calls for LLM (filter against current schema and avoid sending PHI)
				var maskedPrevCalls []speech.FunctionCall
				if p.cfg.FuncCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode == "apply" {
					filteredPrev := p.filterPrevCallsByCurrentSchema(p.prevCalls)
					maskedPrevCalls = filteredPrev
					if p.deps.RedactionService != nil && len(filteredPrev) > 0 {
						maskedPrevCalls = p.sessionRedactionBuffer.maskFunctionCalls(p.cfg.DBSessionID, filteredPrev)
						logger.ServiceDebugf("PIPELINE", "using masked previous function calls for LLM: %d calls (filtered)", len(maskedPrevCalls))
					}
				} else if p.cfg.FuncCfg.ParsingConfig.PrevOutputInclusionPolicy.PrevOutputMode == "ignore" {
					logger.ServiceDebugf("PIPELINE", "Prev output inclusion disabled; not sending previous function calls in prompt")
				}

				
				// Build dynamic prompt with redaction awareness
				// TODO: if we're in diarization mode we need to build the prompt based on speaker turns.
				var dynPrompt string				
				if p.deps.RedactionService != nil {
					// Build combined transcript BEFORE windowing: diarized latest (headers preserved) + context
					combined := transcriptForLLM
					if len(diarizedParas) > 0 {
						diarizedBlock := prompts.RenderDiarizedParagraphs(diarizedParas)
						if p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode == "window" && p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize > 0 {
							maxTok := p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize
							diarToks := len(strings.Fields(diarizedBlock))
							if diarToks >= maxTok {
								// Keep only last maxTok tokens of diarized, preserving header on first included piece
								diarizedBlock = prompts.WindowDiarizedParagraphsTokens(diarizedParas, maxTok)
								combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock
							} else {
								tailNeeded := maxTok - diarToks
								combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock + "\n\nContext (previous masked transcript):\n" + windowLastTokens(transcriptForLLM, tailNeeded)
							}
						} else {
							combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock + "\n\nContext (previous masked transcript):\n" + transcriptForLLM
						}
					} else {
						// No diarization for latest -> window context only if configured
						if p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode == "window" && p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize > 0 {
							combined = windowLastTokens(transcriptForLLM, p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize)
						} else {
							combined = transcriptForLLM
						}
					}
					dynPrompt = prompts.BuildFunctionParsingPromptWithRedaction(combined, p.spellingCache, maskedPrevCalls)
					logger.ServiceDebugf("PIPELINE", "using redaction-aware prompt for function parsing with composed transcript")
				} else {
					// Use standard prompt with current transcript (unredacted path)
					baseContext := tr.Text
					combined := baseContext
					if len(diarizedParas) > 0 {
						diarizedBlock := prompts.RenderDiarizedParagraphs(diarizedParas)
						if p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode == "window" && p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize > 0 {
							maxTok := p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize
							diarToks := len(strings.Fields(diarizedBlock))
							if diarToks >= maxTok {
								diarizedBlock = prompts.WindowDiarizedParagraphsTokens(diarizedParas, maxTok)
								combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock
							} else {
								tailNeeded := maxTok - diarToks
								combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock + "\n\nContext (previous transcript):\n" + windowLastTokens(baseContext, tailNeeded)
							}
						} else {
							combined = "Per-speaker paragraphs (latest):\n" + diarizedBlock + "\n\nContext (previous transcript):\n" + baseContext
						}
					} else {
						if p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.TranscriptMode == "window" && p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize > 0 {
							combined = windowLastTokens(baseContext, p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy.WindowTokenSize)
						} else {
							combined = baseContext
						}
					}
					dynPrompt = prompts.BuildFunctionParsingPrompt(combined, p.spellingCache, maskedPrevCalls)
				}

				// ─── Call adapter ────────────────────────────────────
                calls, u, err := p.deps.LLM.Enrich(ctx, speech.Prompt(dynPrompt), tr, p.cfg.FuncCfg)

                if err != nil {
                    // Track LLM error for metrics and analytics
                    provider := "gemini"        // TODO: get from config
                    model := "gemini-2.5-flash" // TODO: get from config
                    p.usageAccumulator.AddLLMError(provider, model, "enrich_failed", err.Error())
                    logger.Errorf("❌ [PIPELINE] llm enrich failed: %v", err)
                    continue
                }

                if u != nil {
                    // Record LLM usage
                    provider := "gemini"        // TODO: get from config
                    model := "gemini-2.0-flash" // TODO: get from config
                    if u.Cached && u.SavedPromptTokens > 0 {
                        p.usageAccumulator.AddLLMWithSavings(u.Prompt, u.Completion, u.SavedPromptTokens, provider, model)
                    } else {
                        p.usageAccumulator.AddLLM(u.Prompt, u.Completion, provider, model)
                    }
                }

                if len(calls) > 0 {
                    // Strictly validate against current schema; drop invalid or incomplete calls
                    if p.cfg.FuncCfg != nil && len(p.cfg.FuncCfg.Declarations) > 0 {
                        calls = validateAndFilterCallsStrict(calls, p.cfg.FuncCfg.Declarations)
                    }
                    calls = fnutil.InheritIDs(p.prevCalls, calls)
                    calls = fnutil.EnsureIDs(calls)
                    merged := fnutil.MergeUpdate(p.prevCalls, calls)

					logger.ServiceDebugf("PIPELINE", "Debug functions: prev=%v new=%v merged=%v", p.prevCalls, calls, merged)
					
					// Log the raw LLM response to see if it's using placeholders
					for i, call := range calls {
						logger.ServiceDebugf("PIPELINE", "raw LLM function call %d: %s with args: %v", i, call.Name, call.Args)
					}

                    if !reflect.DeepEqual(merged, p.prevCalls) {
						// Store redacted calls for metrics and database storage
						p.sessionRedactedCalls = append(p.sessionRedactedCalls, merged...)
						p.latestRedactedCalls = merged // Store latest batch for metrics
						
						// Aggregate function calls for database storage (using redacted calls)
						for _, call := range merged {
							p.sessionFunctionCalls[call.Name] = call.Args
						}

						// Reconstruct function calls for client (replace placeholders with original values)
						reconstructedCalls := p.sessionRedactionBuffer.reconstructFunctionCalls(p.cfg.DBSessionID, merged)
						
						logger.ServiceDebugf("PIPELINE", "Sending functions: %d (reconstructed for client)", len(reconstructedCalls))
                        p.outFns <- reconstructedCalls
                        p.prevCalls = merged // remember
						b, _ := json.Marshal(merged)
						p.cfg.PrevFunctionsJSON = string(b)
					}
				}

			}
		}

	}()

	return p.outTr, p.outFns, p.outDrafts, nil
}

// StoreCompleteTranscript stores the aggregated complete transcript at session end
func (p *Pipeline) StoreCompleteTranscript(ctx context.Context) {
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
			logger.ServiceDebugf("PIPELINE", "Failed to store complete transcript: %v", err)
		} else {
			logger.ServiceDebugf("PIPELINE", "Stored complete transcript (%d phrases)", 
				len(p.completePhrases),)
		}
	}
}

// StoreAggregatedFunctionCalls stores the aggregated function calls at session end
func (p *Pipeline) StoreAggregatedFunctionCalls(ctx context.Context) {
	if len(p.sessionFunctionCalls) == 0 || p.deps.FunctionCallsRepo == nil {
		return
	}

	var dbSessionID pgtype.UUID
	if dbSessionID.Scan(p.cfg.DBSessionID) == nil {
		for functionName, args := range p.sessionFunctionCalls {
			if _, err := p.deps.FunctionCallsRepo.StoreFunctionCall(ctx, dbSessionID, functionName, args); err != nil {
				logger.ServiceDebugf("PIPELINE", "Failed to store aggregated function call %s: %v", functionName, err)
			} else {
				logger.ServiceDebugf("PIPELINE", "Stored aggregated function call: %s", functionName)
			}
		}
	}
}

// StoreSessionData stores all aggregated session data at session end
func (p *Pipeline) StoreSessionData(ctx context.Context) {
	p.StoreCompleteTranscript(ctx)
	p.StoreAggregatedFunctionCalls(ctx)
	
	// Clear redaction buffers for this session
	if p.sessionRedactionBuffer != nil {
		p.sessionRedactionBuffer.Clear(p.cfg.DBSessionID)
		logger.ServiceDebugf("PIPELINE", "Cleared redaction buffers for session")
	}
}

// windowLastTokens returns the last N whitespace-delimited tokens from text
func windowLastTokens(text string, n int) string {
    if n <= 0 || text == "" {
        return text
    }
    parts := strings.Fields(text)
    if len(parts) <= n {
        return text
    }
    return strings.Join(parts[len(parts)-n:], " ")
}

// filterCallsBySchema removes any function calls missing required arguments per the declared schema
func ensureRequiredArgsWithPlaceholders(calls []speech.FunctionCall, decls []speech.FunctionDefinition) []speech.FunctionCall {
    if len(calls) == 0 || len(decls) == 0 {
        return calls
    }
    // Build lookup of required params per function
    req := map[string][]string{}
    for _, d := range decls {
        required := make([]string, 0)
        for _, p := range d.Parameters {
            if p.Required {
                required = append(required, p.Name)
            }
        }
        req[d.Name] = required
    }
    // Ensure placeholders for missing required args
    for i := range calls {
        c := &calls[i]
        required, ok := req[c.Name]
        if !ok {
            // Unknown function; skip modification
            continue
        }
        if c.Args == nil {
            c.Args = make(map[string]interface{})
        }
        for _, pname := range required {
            if _, exists := c.Args[pname]; !exists {
                c.Args[pname] = "[missing]"
            }
        }
    }
    return calls
}

// validateAndFilterCallsStrict drops any call whose name is not in the schema,
// or which contains any arg not declared in the schema, or is missing any
// required arg, or whose enum/malformed types clearly violate simple constraints.
// We apply light type checks (string/number/bool/object/array) best-effort.
func validateAndFilterCallsStrict(calls []speech.FunctionCall, decls []speech.FunctionDefinition) []speech.FunctionCall {
    if len(calls) == 0 || len(decls) == 0 {
        return nil
    }
    // Build spec maps
    type paramSpec struct{
        typ string
        required bool
        enums map[string]struct{}
    }
    spec := make(map[string]map[string]paramSpec, len(decls))
    requiredByFn := make(map[string]map[string]struct{}, len(decls))
    for _, d := range decls {
        perFn := make(map[string]paramSpec, len(d.Parameters))
        reqs := make(map[string]struct{})
        for _, pdef := range d.Parameters {
            enums := make(map[string]struct{}, len(pdef.Enum))
            for _, e := range pdef.Enum { enums[e] = struct{}{} }
            perFn[pdef.Name] = paramSpec{typ: strings.ToLower(pdef.Type), required: pdef.Required, enums: enums}
            if pdef.Required { reqs[pdef.Name] = struct{}{} }
        }
        spec[d.Name] = perFn
        requiredByFn[d.Name] = reqs
    }

    out := make([]speech.FunctionCall, 0, len(calls))
    for _, c := range calls {
        params, ok := spec[c.Name]
        if !ok {
            continue // function not in schema
        }
        // check args exist and only declared keys
        if c.Args == nil { continue }

        // verify: no unknown keys
        unknown := false
        for k := range c.Args {
            if _, ok := params[k]; !ok { unknown = true; break }
        }
        if unknown { continue }

        // verify: all required keys present and non-empty (best-effort)
        reqs := requiredByFn[c.Name]
        missing := false
        for rk := range reqs {
            if _, ok := c.Args[rk]; !ok { missing = true; break }
            if v := c.Args[rk]; v == nil || v == "" { missing = true; break }
        }
        if missing { continue }

        // verify: basic type and enum checks
        typeBad := false
        for k, v := range c.Args {
            ps := params[k]
            // enum check
            if len(ps.enums) > 0 {
                if s, ok := v.(string); ok {
                    if _, ok := ps.enums[s]; !ok { typeBad = true; break }
                } else {
                    typeBad = true; break
                }
            }
            // type check
            switch ps.typ {
            case "string":
                if _, ok := v.(string); !ok { typeBad = true }
            case "number", "float", "double":
                switch v.(type) {
                case float64, float32, int, int32, int64, uint, uint32, uint64:
                default:
                    typeBad = true
                }
            case "integer":
                switch v.(type) {
                case int, int32, int64, uint, uint32, uint64:
                default:
                    typeBad = true
                }
            case "boolean":
                if _, ok := v.(bool); !ok { typeBad = true }
            case "object":
                if _, ok := v.(map[string]any); !ok { typeBad = true }
            case "array":
                if _, ok := v.([]any); !ok { typeBad = true }
            default:
                // unknown type → let it pass only if it's a string or simple value
                if v == nil { typeBad = true }
            }
            if typeBad { break }
        }
        if typeBad { continue }

        out = append(out, c)
    }
    return out
}

// filterPrevCallsByCurrentSchema returns only those previous calls whose function names
// are present in the current function declarations. It also prunes argument keys to
// those declared in the current schema to avoid leaking fields from previous schemas.
func (p *Pipeline) filterPrevCallsByCurrentSchema(prev []speech.FunctionCall) []speech.FunctionCall {
    if len(prev) == 0 || p.cfg.FuncCfg == nil || len(p.cfg.FuncCfg.Declarations) == 0 {
        return nil
    }
    // Build allow-lists
    allowedFunctions := make(map[string]struct{}, len(p.cfg.FuncCfg.Declarations))
    allowedArgsByFn := make(map[string]map[string]struct{}, len(p.cfg.FuncCfg.Declarations))
    for _, d := range p.cfg.FuncCfg.Declarations {
        allowedFunctions[d.Name] = struct{}{}
        if len(d.Parameters) > 0 {
            m := make(map[string]struct{}, len(d.Parameters))
            for _, prm := range d.Parameters {
                m[prm.Name] = struct{}{}
            }
            allowedArgsByFn[d.Name] = m
        }
    }
    // Filter and prune
    out := make([]speech.FunctionCall, 0, len(prev))
    for _, c := range prev {
        if _, ok := allowedFunctions[c.Name]; !ok {
            continue
        }
        // Copy and prune args to allowed set (if known)
        var prunedArgs map[string]interface{}
        if c.Args != nil {
            if allowed, ok := allowedArgsByFn[c.Name]; ok && len(allowed) > 0 {
                prunedArgs = make(map[string]interface{}, len(c.Args))
                for k, v := range c.Args {
                    if _, keep := allowed[k]; keep {
                        prunedArgs[k] = v
                    }
                }
            } else {
                // No parameter info; keep as-is
                prunedArgs = c.Args
            }
        }
        out = append(out, speech.FunctionCall{
            ID:   c.ID,
            Name: c.Name,
            Args: prunedArgs,
        })
    }
    return out
}

// UpdateFunctionConfig dynamically updates the pipeline's function configuration
func (p *Pipeline) UpdateFunctionConfig(prompt speech.Prompt, funcCfg *speech.FunctionConfig) {
	p.cfg.Prompt = prompt
	p.cfg.FuncCfg = funcCfg
	// Move LLM context epoch to current end of transcript so new schema sees only future segments
	p.epochStartSegmentIdx = p.sessionRedactionBuffer.getSegmentCount(p.cfg.DBSessionID)
	logger.ServiceDebugf("PIPELINE", "Function config updated dynamically")
}

// UpdateDraftIndex dynamically updates the pipeline's draft detection index
func (p *Pipeline) UpdateDraftIndex(draftIndex *draft.Index) {
	p.cfg.DraftIndex = draftIndex
	// Reset known draft scores when index changes
	p.knownDraft = make(map[string]float64)
	logger.ServiceDebugf("PIPELINE", "Draft index updated dynamically")
}

// ClearPrevFunctions resets the rolling previous-calls context used to build dynamic prompts
func (p *Pipeline) ClearPrevFunctions() {
    p.prevCalls = nil
    p.cfg.PrevFunctionsJSON = ""
    logger.ServiceDebugf("PIPELINE", "Cleared previous functions after schema change")
}

// shouldCallLLM determines if we should call the LLM based on the parsing strategy
func (p *Pipeline) shouldCallLLM(isFinal bool) bool {
	if p.cfg.FuncCfg == nil {
		return false
	}

	switch p.cfg.FuncCfg.ParsingConfig.ParsingStrategy {
	case "auto":
		// Auto strategy: call on final transcripts, respecting UpdateMS throttling
		return isFinal
	case "update-ms":
		// Update-ms strategy: call on final transcripts, respecting UpdateMS throttling
		return isFinal
	case "manual":
		// Manual strategy: never auto-call; only on explicit trigger
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
func (p *Pipeline) ForceLLMCall(ctx context.Context, transcriptForLLM string) error {
	logger.Debugf("🔇 [PIPELINE] ForceLLMCall called with transcript length: %d, strategy: %s", len(transcriptForLLM), p.cfg.FuncCfg.ParsingConfig.ParsingStrategy)
	
	if p.deps.LLM == nil || p.cfg.FuncCfg == nil {
		logger.Errorf("🔇 [PIPELINE] ForceLLMCall failed - LLM or function config not available")
		return fmt.Errorf("LLM or function config not available")
	}

	// For "after-silence" strategy, prevent multiple calls during the same silence period
	if p.cfg.FuncCfg.ParsingConfig.ParsingStrategy == "after-silence" && p.inSilence {
		logger.Debugf("🔇 [PIPELINE] Skipping LLM call - already processed this silence period")
		return nil
	}

	// Create masked previous function calls for LLM (filtered against current schema)
	filteredPrev := p.filterPrevCallsByCurrentSchema(p.prevCalls)
	maskedPrevCalls := filteredPrev
	if p.deps.RedactionService != nil && len(filteredPrev) > 0 {
		maskedPrevCalls = p.sessionRedactionBuffer.maskFunctionCalls(p.cfg.DBSessionID, filteredPrev)
	}

	// Apply transcript windowing policy to manual call (parity with streaming path)
	pol := p.cfg.FuncCfg.ParsingConfig.TranscriptInclusionPolicy
	if pol.TranscriptMode == "window" && pol.WindowTokenSize > 0 {
		before := len(transcriptForLLM)
		transcriptForLLM = windowLastTokens(transcriptForLLM, pol.WindowTokenSize)
		logger.ServiceDebugf("PIPELINE", "ForceLLMCall: applied transcript windowing size=%d chars_before=%d chars_after=%d", pol.WindowTokenSize, before, len(transcriptForLLM))
	}

	// Build dynamic prompt
	var dynPrompt string
	if p.deps.RedactionService != nil {
		dynPrompt = prompts.BuildFunctionParsingPromptWithRedaction(transcriptForLLM, p.spellingCache, maskedPrevCalls)
	} else {
		dynPrompt = prompts.BuildFunctionParsingPrompt(transcriptForLLM, p.spellingCache, maskedPrevCalls)
	}

	// Create a dummy transcript for the LLM call
	dummyTr := speech.Transcript{
		Text:    transcriptForLLM,
		IsFinal: true,
	}

	// Call LLM
	calls, u, err := p.deps.LLM.Enrich(ctx, speech.Prompt(dynPrompt), dummyTr, p.cfg.FuncCfg)
	if err != nil {
		return fmt.Errorf("LLM enrich failed: %w", err)
	}

	// Process the results (similar to the main pipeline logic)
	if len(calls) > 0 {
		if p.cfg.FuncCfg != nil && len(p.cfg.FuncCfg.Declarations) > 0 {
			calls = validateAndFilterCallsStrict(calls, p.cfg.FuncCfg.Declarations)
		}
		calls = fnutil.InheritIDs(p.prevCalls, calls)
		calls = fnutil.EnsureIDs(calls)
		merged := fnutil.MergeUpdate(p.prevCalls, calls)

		if !reflect.DeepEqual(merged, p.prevCalls) {
			// Store redacted calls for metrics and database storage
			p.sessionRedactedCalls = append(p.sessionRedactedCalls, merged...)
			p.latestRedactedCalls = merged

			// Aggregate function calls for database storage (using redacted calls)
			for _, call := range merged {
				p.sessionFunctionCalls[call.Name] = call.Args
			}

			// Reconstruct function calls for client (replace placeholders with original values)
			reconstructedCalls := p.sessionRedactionBuffer.reconstructFunctionCalls(p.cfg.DBSessionID, merged)
			
			logger.ServiceDebugf("PIPELINE", "Sending functions from ForceLLMCall: %d (reconstructed for client)", len(reconstructedCalls))
			
			// Send to output channel
			select {
			case p.outFns <- reconstructedCalls:
			default:
				logger.Warnf("⚠️ [PIPELINE] Function output channel full, dropping forced LLM call results")
			}

			// Remember merged calls for subsequent prompts and merging
			p.prevCalls = merged
			b, _ := json.Marshal(merged)
			p.cfg.PrevFunctionsJSON = string(b)
		}
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

	// For "after-silence" strategy, mark that we've processed this silence period
	if p.cfg.FuncCfg.ParsingConfig.ParsingStrategy == "after-silence" {
		p.inSilence = true
		logger.Debugf("🔇 [PIPELINE] Marked silence period as processed - will skip future calls until audio resumes")
	}

	return nil
}
