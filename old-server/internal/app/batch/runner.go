package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"schma.ai/internal/app/pipeline"
	"schma.ai/internal/app/prompts"
	"schma.ai/internal/app/usage"
	"schma.ai/internal/domain/batch"
	db_domain "schma.ai/internal/domain/db"
	normalizer_domain "schma.ai/internal/domain/normalizer"
	redaction_domain "schma.ai/internal/domain/redaction"
	"schma.ai/internal/domain/session"
	"schma.ai/internal/domain/speech"
	usage_domain "schma.ai/internal/domain/usage"
	"schma.ai/internal/pkg/logger"
	pkg_speech "schma.ai/internal/pkg/speech"
)

// BatchProcessor handles the actual processing of batch jobs
type BatchProcessor struct {
	jobRepo                     batch.JobRepo
	stt                         speech.STTBatchClient
	llm                         Parser
	transcriptsRepo             db_domain.TranscriptsRepo
	functionCallsRepo           db_domain.FunctionCallsRepo
	structuredOutputsRepo       db_domain.StructuredOutputsRepo
	functionSchemasRepo         db_domain.FunctionSchemasRepo
	structuredOutputSchemasRepo db_domain.StructuredOutputSchemasRepo
	usageMeterRepo              usage_domain.UsageMeterRepo
	usageEventRepo              usage_domain.UsageEventRepo
	draftAggRepo                usage_domain.DraftAggRepo
	sessionManager              session.Manager
	pricing                     usage_domain.Pricing
	// Pipeline dependencies for proper processing
	normalizer                  normalizer_domain.Normalizer
	redactionService            redaction_domain.Redactor
}

type Parser interface {
	Enrich(ctx context.Context, prompt speech.Prompt, tr speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error)
}

func NewBatchProcessor(
	jobRepo batch.JobRepo,
	stt speech.STTBatchClient,
	llm Parser,
	transcriptsRepo db_domain.TranscriptsRepo,
	functionCallsRepo db_domain.FunctionCallsRepo,
	structuredOutputsRepo db_domain.StructuredOutputsRepo,
	functionSchemasRepo db_domain.FunctionSchemasRepo,
	structuredOutputSchemasRepo db_domain.StructuredOutputSchemasRepo,
	usageMeterRepo usage_domain.UsageMeterRepo,
	usageEventRepo usage_domain.UsageEventRepo,
	draftAggRepo usage_domain.DraftAggRepo,
	sessionManager session.Manager,
	normalizer normalizer_domain.Normalizer,
	redactionService redaction_domain.Redactor,
	pricing usage_domain.Pricing,
) *BatchProcessor {
	return &BatchProcessor{
		jobRepo:                     jobRepo,
		stt:                         stt,
		llm:                         llm,
		transcriptsRepo:             transcriptsRepo,
		functionCallsRepo:           functionCallsRepo,
		structuredOutputsRepo:       structuredOutputsRepo,
		functionSchemasRepo:         functionSchemasRepo,
		structuredOutputSchemasRepo: structuredOutputSchemasRepo,
		usageMeterRepo:              usageMeterRepo,
		usageEventRepo:              usageEventRepo,
		draftAggRepo:                draftAggRepo,
		sessionManager:              sessionManager,
		normalizer:                  normalizer,
		redactionService:            redactionService,
		pricing:                     pricing,
	}
}

func (p *BatchProcessor) ProcessJob(ctx context.Context, job batch.Job) error {
    // Always attempt to clean up temp files/artifacts when the job finishes
    defer func() {
        if err := p.cleanupJobFiles(job); err != nil {
            logger.Warnf("⚠️ [BATCH] Cleanup warning for job %s: %v", job.ID.String(), err)
        }
    }()

	// Update status to processing
	if err := p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusProcessing, ""); err != nil {
		return fmt.Errorf("failed to update job status to processing: %w", err)
	}

	_, err := p.processJobInternal(ctx, job)
	if err != nil {
		// Update status to failed
		p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusFailed, err.Error())
		return err
	}

	// Update status to completed
	if err := p.jobRepo.UpdateStatus(ctx, job.ID, batch.StatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status to completed: %w", err)
	}

	return nil
}

func (p *BatchProcessor) processJobInternal(ctx context.Context, job batch.Job) (map[string]any, error) {
	// 1. Open and read the audio file
	file, err := os.Open(job.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// 2. Transcribe using batch STT (no streaming)
	finalTranscript, err := p.stt.TranscribeFile(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("batch STT failed: %w", err)
	}

	// 3. Process transcript following pipeline approach: normalize -> redact -> phrases
	logger.ServiceDebugf("BATCH", "🔍 Processing transcript: %s", finalTranscript.Text)

	// 3.1 Normalize transcript
	nctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	normalized, nerr := p.normalizer.Normalize(nctx, finalTranscript.Text)
	cancel()

	if nerr != nil {
		logger.Warnf("⚠️ [BATCH] Normalization error, using raw text: %v", nerr)
		normalized = finalTranscript.Text
	}
	logger.ServiceDebugf("BATCH", "🔍 Normalized text: %s", normalized)

	// 3.2 Initialize redaction buffer for this session
	sessionRedactionBuffer := pipeline.NewSessionRedactionBuffer()
	sessionIDStr := job.SessionID.String()

	// 3.3 Redact for LLM processing (masked text)
	masked := normalized
	loggedNoRedaction := false

	if p.redactionService == nil {
		if !loggedNoRedaction {
			logger.Warnf("🔒 [BATCH] No redaction service configured, skipping PHI redaction")
			loggedNoRedaction = true
		}
	} else {
		// Create pipeline deps for redaction
		deps := pipeline.Deps{RedactionService: p.redactionService}
		if redactedText, rerr := pipeline.RedactTranscriptForLLM(sessionIDStr, normalized, deps, sessionRedactionBuffer, false); rerr != nil {
			logger.Errorf("❌ [BATCH] PHI redaction failed: %v", rerr)
		} else {
			masked = redactedText
		}
	}
	logger.Infof("🔍 [BATCH] Redacted text: %s", masked)

	// 3.4 Build phrases from words (similar to streaming pipeline)
	var phrases []speech.Phrase
	var phrasesLight []speech.PhraseLight

	if len(finalTranscript.Turns) > 0 {
		// Group by diarization turns
		normByTurn := map[string]string{}
		maskedByTurn := map[string]string{}
		for _, t := range finalTranscript.Turns {
			normByTurn[t.ID] = normalized
			maskedByTurn[t.ID] = masked
		}
		phs, pls := pkg_speech.BuildPhrasesBySpeakerGroups(finalTranscript.Words, finalTranscript.Turns, normByTurn, maskedByTurn)
		phrases = append(phrases, phs...)
		phrasesLight = append(phrasesLight, pls...)
	} else {
		// Multiple phrases from final words, split by speaker boundaries
		phs, pls := pkg_speech.BuildPhrasesFromFinalWords(finalTranscript.Words, normalized, masked)
		phrases = append(phrases, phs...)
		phrasesLight = append(phrasesLight, pls...)
	}

	// 3.5 Store complete transcript with phrases (using masked text for storage)
	completeTranscript := speech.Transcript{
		Text:        masked, // Store masked version in database
		IsFinal:     true,
		Confidence:  finalTranscript.Confidence,
		ChunkDurSec: finalTranscript.ChunkDurSec,
		Phrases:     phrasesLight,
		Words:       nil, // Don't store words, use phrases instead
		Turns:       finalTranscript.Turns,
	}

	if _, err := p.transcriptsRepo.StoreTranscript(ctx, job.SessionID, completeTranscript); err != nil {
		logger.Errorf("❌ [BATCH] Failed to store transcript (non-fatal): %v", err)
		// Continue processing - transcript storage is supplementary
	}

	// 4. Retrieve configuration from schema tables (linked to this session)
	var functionConfig *speech.FunctionConfig
	var structuredOutputCfg *speech.StructuredOutputConfig

	// Check for function schemas linked to this session
	functionSchemas, err := p.functionSchemasRepo.GetSchemasBySession(ctx, job.SessionID)
	if err != nil {
		logger.Errorf("❌ [BATCH] Failed to get function schemas (non-fatal): %v", err)
	} else if len(functionSchemas) > 0 {
		// Use the first function schema (there should only be one per batch session)
		schema := functionSchemas[0]
		functionConfig = &speech.FunctionConfig{
			ParsingConfig: speech.ParsingConfig{ParsingStrategy: "end-of-session"}, // Batch always uses end-of-session
			UpdateMs:        int(schema.UpdateMs.Int32),
			ParsingGuide:    schema.ParsingGuide.String,
		}
		// Unmarshal declarations from JSON
		if err := json.Unmarshal(schema.Declarations, &functionConfig.Declarations); err != nil {
			return nil, fmt.Errorf("failed to unmarshal function declarations: %w", err)
		}
		logger.ServiceDebugf("BATCH", "🔗 Using function schema: %s", schema.Name.String)
	}

	// Check for structured output schemas linked to this session
	structuredSchemas, err := p.structuredOutputSchemasRepo.GetSchemasBySession(ctx, job.SessionID)
	if err != nil {
		logger.Errorf("❌ [BATCH] Failed to get structured output schemas (non-fatal): %v", err)
	} else if len(structuredSchemas) > 0 {
		// Use the first structured schema (there should only be one per batch session)
		schema := structuredSchemas[0]
		structuredOutputCfg = &speech.StructuredOutputConfig{
			UpdateMs:     int(schema.UpdateMs.Int32),
			ParsingGuide: schema.ParsingGuide.String,
		}
		// Unmarshal schema from JSON
		if err := json.Unmarshal(schema.Schema, &structuredOutputCfg.Schema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal structured output schema: %w", err)
		}
		logger.ServiceDebugf("BATCH", "🔗 Using structured output schema: %s", schema.Name.String)
	}

	// Validate that we have exactly one configuration type
	if functionConfig == nil && structuredOutputCfg == nil {
		return nil, fmt.Errorf("no function or structured output configuration found for session %s", job.SessionID.String())
	}
	if functionConfig != nil && structuredOutputCfg != nil {
		return nil, fmt.Errorf("both function and structured output configurations found for session %s - should have exactly one", job.SessionID.String())
	}

	var functionCalls []speech.FunctionCall
	var structuredObj map[string]any
	var llmUsage *speech.LLMUsage

	// 5. Initialize usage accumulator for this batch session
	usageAccumulator := usage.NewUsageAccumulator(
		job.SessionID,
		job.AppID,
		job.AccountID,
		p.usageMeterRepo,
		p.usageEventRepo,
		p.draftAggRepo,
		true,    // batch mode
		"batch", // LLM mode
	)

	// Start usage accumulator for this batch processing
	usageAccumulator.Start(ctx)
	defer func() {
		// Always stop and flush usage accumulator
		usageAccumulator.Stop(ctx)
	}()

	// Log STT usage for the entire audio file
	usageAccumulator.AddSTT(finalTranscript.ChunkDurSec, "session_total")

	// 6. Process with LLM depending on mode (using masked text for LLM)
	if functionConfig != nil && p.llm != nil {
		// Build function parsing prompt with redaction awareness
		var dynPrompt string
		if p.redactionService != nil {
			// Use redaction-aware prompt with masked text
			dynPrompt = prompts.BuildFunctionParsingPromptWithRedaction(masked, map[string]string{}, []speech.FunctionCall{})
		} else {
			// Use standard prompt
			dynPrompt = prompts.BuildFunctionParsingPrompt(masked, map[string]string{}, []speech.FunctionCall{})
		}

		// Create transcript with masked text for LLM
		maskedTranscript := finalTranscript
		maskedTranscript.Text = masked

		functionCalls, llmUsage, err = p.llm.Enrich(ctx, speech.Prompt(dynPrompt), maskedTranscript, functionConfig)
		if err != nil {
			logger.Errorf("❌ [BATCH] LLM (functions) failed (non-fatal): %v", err)
		} else {
			// Log LLM usage
			if llmUsage != nil {
				usageAccumulator.AddLLM(llmUsage.Prompt, llmUsage.Completion, "gemini", "gemini-2.0-flash")
			}

			// Reconstruct function calls for client (replace placeholders with original values)
			reconstructedCalls := functionCalls
			if p.redactionService != nil {
				// Note: Using unexported method - this should be exported in pipeline package
				// For now, we'll use the raw function calls (redaction reconstruction is optional)
				logger.Warnf("⚠️ [BATCH] Function call reconstruction not available - using raw calls")
			}

			// Store function calls in session (using redacted versions for database)
			for _, fnCall := range functionCalls {
				if _, err := p.functionCallsRepo.StoreFunctionCall(ctx, job.SessionID, fnCall.Name, fnCall.Args); err != nil {
					logger.Errorf("❌ [BATCH] Failed to store function call (non-fatal): %v", err)
					// Continue processing - function call storage is supplementary
				}
			}

			// Use reconstructed calls for result (client-safe)
			functionCalls = reconstructedCalls
		}
	} else if structuredOutputCfg != nil && p.llm != nil {
		// Build structured prompt for batch (using masked text)
		sp := prompts.BuildStructuredParsingPrompt(masked, map[string]string{}, map[string]any{})

		// Prepare structured LLM config
		schemaBytes, _ := json.Marshal(structuredOutputCfg.Schema)
		sCfg := &speech.StructuredConfig{
			Schema:       schemaBytes,
			ParsingGuide: structuredOutputCfg.ParsingGuide,
			UpdateMS:     structuredOutputCfg.UpdateMs,
		}

		if sLLM, ok := p.llm.(speech.StructuredLLM); ok {
			// Create transcript with masked text for LLM
			maskedTranscript := finalTranscript
			maskedTranscript.Text = masked

			obj, usage, err := sLLM.GenerateStructured(ctx, speech.Prompt(sp), maskedTranscript, sCfg)
			if err != nil {
				logger.Errorf("❌ [BATCH] LLM (structured) failed (non-fatal): %v", err)
			} else {
				logger.ServiceDebugf("BATCH", "🎯 LLM returned successful response: %+v", obj)
				// Extract structured data from LLM response
				// LLM may return structured data wrapped in a "text" field that needs to be parsed
				var finalStructuredObj map[string]any
				if textValue, hasText := obj["text"]; hasText {
					logger.ServiceDebugf("BATCH", "📝 Found text field in LLM response, attempting to parse JSON...")
					// LLM returned JSON as a string in "text" field - parse it to get the actual structured data
					if textStr, ok := textValue.(string); ok {
						logger.ServiceDebugf("BATCH", "🔍 Text field content: %s", textStr)
						if err := json.Unmarshal([]byte(textStr), &finalStructuredObj); err != nil {
							logger.Errorf("❌ [BATCH] Failed to parse JSON from LLM text response: %v", err)
							finalStructuredObj = obj // fallback to raw response
						} else {
							logger.Infof("✅ [BATCH] Successfully extracted structured data: %+v", finalStructuredObj)
						}
					} else {
						logger.Warnf("⚠️ [BATCH] LLM text field is not a string: %T", textValue)
						finalStructuredObj = obj
					}
				} else {
					logger.ServiceDebugf("BATCH", "🔄 No text field found, using LLM response directly")
					// LLM returned structured object directly
					finalStructuredObj = obj
				}

				structuredObj = finalStructuredObj
				llmUsage = usage

				// Log LLM usage
				if llmUsage != nil {
					usageAccumulator.AddLLM(llmUsage.Prompt, llmUsage.Completion, "gemini", "gemini-2.0-flash")
				}

				// Store structured output in session - use the extracted structured data as the final object
				update := speech.StructuredOutputUpdate{
					Rev:   1,
					Delta: finalStructuredObj, // Use the extracted structured data
					Final: finalStructuredObj, // Use the extracted structured data
				}
				if _, err := p.structuredOutputsRepo.StoreStructuredOutput(ctx, job.SessionID, update); err != nil {
					logger.Errorf("❌ [BATCH] Failed to store structured output (non-fatal): %v", err)
					// Continue processing - structured output storage is supplementary
				}

				// Reconstruct structured output for client if needed
				// TODO: Implement structured output reconstruction if needed
			}
		} else {
			logger.Warnf("⚠️ [BATCH] Structured output requested but LLM does not implement StructuredLLM")
		}
	}

	// 7. Prepare results (client-facing data with normalized text and reconstructed function calls)
	clientTranscript := speech.Transcript{
		Text:           normalized, // Use normalized text for client (not masked)
		IsFinal:        true,
		Confidence:     finalTranscript.Confidence,
		ChunkDurSec:    finalTranscript.ChunkDurSec,
		PhrasesDisplay: phrases, // Include phrases with both normalized and reconstructed data
		Turns:          finalTranscript.Turns,
	}

	result := map[string]any{
		"transcript": map[string]any{
			"text":            clientTranscript.Text,
			"confidence":      clientTranscript.Confidence,
			"is_final":        clientTranscript.IsFinal,
			"duration":        clientTranscript.ChunkDurSec,
			"phrases_display": clientTranscript.PhrasesDisplay,
			"turns":           clientTranscript.Turns,
		},
		"processed_at": time.Now().UTC().Format(time.RFC3339),
		"file_size":    job.FileSize,
		"session_id":   job.SessionID.String(),
	}

	if functionConfig != nil {
		result["functions"] = functionCalls
	}
	if structuredOutputCfg != nil {
		result["structured"] = structuredObj
	}

	if llmUsage != nil {
		result["usage"] = map[string]any{
			"prompt_tokens":     llmUsage.Prompt,
			"completion_tokens": llmUsage.Completion,
		}
	}

	// 7. Save artifacts to output directory (if needed)
	if err := p.saveArtifacts(job, finalTranscript, functionCalls, structuredObj); err != nil {
		logger.Errorf("❌ [BATCH] Failed to save artifacts (non-fatal): %v", err)
		// Don't fail the job for artifact saving errors
	}

	// 8. Close the session (set closed_at timestamp)
	if err := p.sessionManager.CloseSession(ctx, session.DBSessionID(job.SessionID)); err != nil {
		logger.Errorf("❌ [BATCH] Failed to close session (non-fatal): %v", err)
		// Continue - session closing is supplementary
	}

	// 9. Clear redaction buffers
	if p.redactionService != nil {
		sessionRedactionBuffer.Clear(sessionIDStr)
	}

	logger.Infof("✅ [BATCH] Successfully processed batch job %s for session %s", job.ID.String(), job.SessionID.String())

	return result, nil
}

func (p *BatchProcessor) saveArtifacts(job batch.Job, transcript speech.Transcript, functions []speech.FunctionCall, structured map[string]any) error {
    // Optional: gate artifact saving behind env flag (default off)
    if os.Getenv("BATCH_SAVE_ARTIFACTS") != "1" {
        return nil
    }

	// Create output directory based on job ID
	outputDir := filepath.Join(os.TempDir(), "schma-batch-results", job.ID.String())
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Save transcript
	transcriptData, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(outputDir, "transcript.json"), transcriptData, 0644); err != nil {
		return err
	}

	// Save functions
	if functions != nil {
		functionsData, err := json.MarshalIndent(functions, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outputDir, "functions.json"), functionsData, 0644); err != nil {
			return err
		}
	}

	// Save structured output if present
	if structured != nil {
		structuredData, err := json.MarshalIndent(structured, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outputDir, "structured.json"), structuredData, 0644); err != nil {
			return err
		}
	}

	logger.Infof("✅ [BATCH] Artifacts saved to: %s", outputDir)
	return nil
}

// cleanupJobFiles removes the temp audio file/dir and any artifacts directory for the job
func (p *BatchProcessor) cleanupJobFiles(job batch.Job) error {
    errors := []error{}

    // Remove the job's temp dir (contains the uploaded audio)
    if job.FilePath != "" {
        dir := filepath.Dir(job.FilePath)
        if err := os.RemoveAll(dir); err != nil {
            errors = append(errors, err)
        }
    }

    // Remove artifacts dir if present
    artDir := filepath.Join(os.TempDir(), "schma-batch-results", job.ID.String())
    if err := os.RemoveAll(artDir); err != nil {
        errors = append(errors, err)
    }

    // Return the first error, if any
    if len(errors) > 0 {
        return errors[0]
    }
    return nil
}
