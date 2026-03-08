package pipeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	redactor "schma.ai/internal/domain/redaction"
	"schma.ai/internal/domain/speech"
	infra_redaction "schma.ai/internal/infra/redaction"
	"schma.ai/internal/pkg/logger"
)

// TranscriptSegment represents a single redacted transcript segment
type TranscriptSegment struct {
	OriginalText string
	MaskedText   string
	Spans        []redactor.Span
	Placeholders []redactor.Placeholder
	Timestamp    time.Time
}

// RedactionBuffer stores redaction information for a session
type RedactionBuffer struct {
	SessionID              string
	TranscriptSegments     []TranscriptSegment
	NextPlaceholderOrdinal int
}

// SessionRedactionBuffer manages redaction buffers for multiple sessions
type SessionRedactionBuffer struct {
	buffers map[string]*RedactionBuffer
}

// NewSessionRedactionBuffer creates a new session redaction buffer manager
func NewSessionRedactionBuffer() *SessionRedactionBuffer {
	return &SessionRedactionBuffer{
		buffers: make(map[string]*RedactionBuffer),
	}
}

// Store stores a redaction buffer for a session
func (rb *SessionRedactionBuffer) Store(sessionID string, buffer *RedactionBuffer) {
	rb.buffers[sessionID] = buffer
	logger.ServiceDebugf("REDACTION", "Stored redaction buffer for session %s with %d segments", 
		sessionID, len(buffer.TranscriptSegments))
}

// Get retrieves a redaction buffer for a session
func (rb *SessionRedactionBuffer) Get(sessionID string) (*RedactionBuffer, bool) {
	buffer, exists := rb.buffers[sessionID]
	return buffer, exists
}

// Clear removes a redaction buffer for a session
func (rb *SessionRedactionBuffer) Clear(sessionID string) {
	delete(rb.buffers, sessionID)
	logger.ServiceDebugf("REDACTION", "Cleared redaction buffer for session %s", sessionID)
}

// ClearAll removes all redaction buffers
func (rb *SessionRedactionBuffer) ClearAll() {
	rb.buffers = make(map[string]*RedactionBuffer)
	logger.ServiceDebugf("REDACTION", "Cleared all redaction buffers")
}

// reconstructText reconstructs text by replacing placeholders with original values
func (rb *SessionRedactionBuffer) reconstructText(sessionID, text string) string {
	buffer, exists := rb.Get(sessionID)
	if !exists {
		logger.ServiceDebugf("REDACTION", "No redaction buffer found for session %s, returning original text", sessionID)
		return text
	}

	result := text
	replacedCount := 0

	// For each placeholder in the text, find it in the appropriate transcript segment
	for _, segment := range buffer.TranscriptSegments {
		for _, placeholder := range segment.Placeholders {
			if strings.Contains(result, placeholder.Text) {
				originalValue := rb.extractOriginalValue(segment, placeholder)
				if originalValue != "" {
					result = strings.ReplaceAll(result, placeholder.Text, originalValue)
					replacedCount++
					logger.ServiceDebugf("REDACTION", "Replaced %s with %s", placeholder.Text, originalValue)
				}
			}
		}
	}

	logger.ServiceDebugf("REDACTION", "Reconstructed text for session %s, replaced %d placeholders", sessionID, replacedCount)
	return result
}

// extractOriginalValue extracts the original value from a transcript segment based on placeholder
func (rb *SessionRedactionBuffer) extractOriginalValue(segment TranscriptSegment, placeholder redactor.Placeholder) string {
	// Find the span that corresponds to this placeholder
	for i, span := range segment.Spans {
		if i < len(segment.Placeholders) && segment.Placeholders[i].Text == placeholder.Text {
			logger.ServiceDebugf("REDACTION", "Extracting for %s: span[%d] = %+v, text length = %d", 
				placeholder.Text, i, span, len(segment.OriginalText))
			
			if span.Start >= 0 && span.End <= len(segment.OriginalText) {
				extracted := segment.OriginalText[span.Start:span.End]
				logger.ServiceDebugf("REDACTION", "Extracted '%s' from positions %d:%d", extracted, span.Start, span.End)
				return extracted
			} else {
				logger.ServiceDebugf("REDACTION", "Invalid span coordinates: start=%d, end=%d, text_length=%d", 
					span.Start, span.End, len(segment.OriginalText))
			}
		}
	}
	return ""
}

// reconstructArgs reconstructs function call arguments by replacing placeholders
func (rb *SessionRedactionBuffer) reconstructArgs(sessionID string, args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}

	// Convert args to JSON string, reconstruct placeholders, convert back
	argsJSON, err := json.Marshal(args)
	if err != nil {
		logger.Errorf("❌ [REDACTION] Failed to marshal args for reconstruction: %v", err)
		return args
	}

	reconstructedJSON := rb.reconstructText(sessionID, string(argsJSON))

	var reconstructedArgs map[string]interface{}
	if err := json.Unmarshal([]byte(reconstructedJSON), &reconstructedArgs); err != nil {
		logger.Errorf("❌ [REDACTION] Failed to unmarshal reconstructed args: %v", err)
		return args
	}

	return reconstructedArgs
}

// reconstructStructuredOutput reconstructs structured output by replacing placeholders
func (rb *SessionRedactionBuffer) reconstructStructuredOutput(sessionID string, output map[string]interface{}) map[string]interface{} {
	if output == nil {
		return nil
	}

	// Convert output to JSON string, reconstruct placeholders, convert back
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logger.Errorf("❌ [REDACTION] Failed to marshal output for reconstruction: %v", err)
		return output
	}

	reconstructedJSON := rb.reconstructText(sessionID, string(outputJSON))

	var reconstructedOutput map[string]interface{}
	if err := json.Unmarshal([]byte(reconstructedJSON), &reconstructedOutput); err != nil {
		logger.Errorf("❌ [REDACTION] Failed to unmarshal reconstructed output: %v", err)
		return output
	}

	return reconstructedOutput
}

// maskFunctionCalls masks function calls by replacing original values with placeholders
func (rb *SessionRedactionBuffer) maskFunctionCalls(sessionID string, calls []speech.FunctionCall) []speech.FunctionCall {
	if calls == nil {
		return nil
	}

	logger.ServiceDebugf("REDACTION", "Masking %d function calls for session %s", len(calls), sessionID)
	
	masked := make([]speech.FunctionCall, len(calls))
	for i, call := range calls {
		masked[i] = call
		masked[i].Args = rb.maskArgs(sessionID, call.Args)
		logger.ServiceDebugf("REDACTION", "Masked function call %s: %v", call.Name, masked[i].Args)
	}

	return masked
}

// maskArgs masks function arguments by replacing original values with placeholders
func (rb *SessionRedactionBuffer) maskArgs(sessionID string, args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}

	// Convert args to JSON string, mask placeholders, convert back
	argsJSON, err := json.Marshal(args)
	if err != nil {
		logger.Errorf("❌ [REDACTION] Failed to marshal args for masking: %v", err)
		return args
	}

	maskedJSON := rb.maskText(sessionID, string(argsJSON))

	var maskedArgs map[string]interface{}
	if err := json.Unmarshal([]byte(maskedJSON), &maskedArgs); err != nil {
		logger.Errorf("❌ [REDACTION] Failed to unmarshal masked args: %v", err)
		return args
	}

	return maskedArgs
}

// maskText masks text by replacing original values with placeholders
func (rb *SessionRedactionBuffer) maskText(sessionID, text string) string {
	buffer, exists := rb.Get(sessionID)
	if !exists {
		logger.ServiceDebugf("REDACTION", "No redaction buffer found for session %s, returning original text", sessionID)
		return text
	}

	result := text
	maskedCount := 0

	// For each transcript segment, replace original values with placeholders
	for _, segment := range buffer.TranscriptSegments {
		for i := range segment.Spans {
			if i < len(segment.Placeholders) {
				placeholder := segment.Placeholders[i]
				originalValue := rb.extractOriginalValue(segment, placeholder)
				if originalValue != "" && strings.Contains(result, originalValue) {
					result = strings.ReplaceAll(result, originalValue, placeholder.Text)
					logger.ServiceDebugf("REDACTION", "Masked %s with %s", originalValue, placeholder.Text)
					maskedCount++
				}
			}
		}
	}

	if maskedCount > 0 {
		logger.ServiceDebugf("REDACTION", "Masked %d values in text for session %s", maskedCount, sessionID)
	}
	return result
}

// maskStructuredOutput masks structured output by replacing original values with placeholders
func (rb *SessionRedactionBuffer) maskStructuredOutput(sessionID string, output map[string]interface{}) map[string]interface{} {
	if output == nil {
		return nil
	}

	logger.ServiceDebugf("REDACTION", "Masking structured output for session %s: %v", sessionID, output)

	// Convert output to JSON string, mask placeholders, convert back
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logger.Errorf("❌ [REDACTION] Failed to marshal output for masking: %v", err)
		return output
	}

	maskedJSON := rb.maskText(sessionID, string(outputJSON))

	var maskedOutput map[string]interface{}
	if err := json.Unmarshal([]byte(maskedJSON), &maskedOutput); err != nil {
		logger.Errorf("❌ [REDACTION] Failed to unmarshal masked output: %v", err)
		return output
	}

	logger.ServiceDebugf("REDACTION", "Masked structured output: %v", maskedOutput)
	return maskedOutput
}

// reconstructFunctionCalls reconstructs function calls by replacing placeholders in arguments
func (rb *SessionRedactionBuffer) reconstructFunctionCalls(sessionID string, calls []speech.FunctionCall) []speech.FunctionCall {
	if calls == nil {
		return nil
	}

	reconstructed := make([]speech.FunctionCall, len(calls))
	for i, call := range calls {
		reconstructed[i] = call
		reconstructed[i].Args = rb.reconstructArgs(sessionID, call.Args)
	}

	return reconstructed
}


// RedactTranscriptForLLM redacts a transcript before sending to LLM and stores the buffer.
// NOTE: This version always appends a segment to the session buffer (even if no spans are found),
// so getAccumulatedMaskedTranscript() is a complete, ordered view of the session.
// When disablePHI is true, only PCI is masked; when false and a redaction service is available,
// PHI spans from the service are included in addition to PCI.
func RedactTranscriptForLLM(sessionID, text string, deps Deps, redactionBuffer *SessionRedactionBuffer, disablePHI bool) (string, error) {
	// Build spans set
	spans := make([]redactor.Span, 0, 8)
	// Optional PHI via configured redaction service
	if !disablePHI && deps.RedactionService != nil {
		if phiSpans, err := deps.RedactionService.RedactTranscript(text); err == nil {
			spans = append(spans, phiSpans...)
		} else {
			logger.Errorf("❌ [REDACTION] failed to PHI-redact transcript for session %s: %v", sessionID, err)
		}
	}
	// Always add PCI spans
	pciSpans := infra_redaction.DetectPCI(text)
	if len(pciSpans) > 0 {
		spans = append(spans, pciSpans...)
	}

	// Get existing buffer or create new one
	existingBuffer, exists := redactionBuffer.Get(sessionID)
	var buffer *RedactionBuffer
	if exists {
		buffer = existingBuffer
		logger.ServiceDebugf("REDACTION", "Using existing buffer for session %s with %d segments",
			sessionID, len(buffer.TranscriptSegments))
	} else {
		buffer = &RedactionBuffer{
			SessionID:              sessionID,
			TranscriptSegments:     []TranscriptSegment{},
			NextPlaceholderOrdinal: 1,
		}
		logger.ServiceDebugf("REDACTION", "Creating new buffer for session %s", sessionID)
	}

	// Create placeholders and masked output (always produce a segment)
	maskedOut := text
	placeholders := make([]redactor.Placeholder, 0, len(spans))

	if len(spans) > 0 {
		placeholders = make([]redactor.Placeholder, len(spans))
		for i, span := range spans {
			ordinal := buffer.NextPlaceholderOrdinal + i
			placeholders[i] = redactor.Placeholder{
				Kind:    span.Kind,
				Ordinal: ordinal,
				Text:    formatPlaceholder(span, ordinal, "minimal"), // minimal format for LLM
			}
		}
		maskedOut = applyMaskingWithPlaceholders(text, spans, placeholders)
		buffer.NextPlaceholderOrdinal += len(spans)
	} else {
		logger.ServiceDebugf("REDACTION", "No sensitive data detected in transcript for session %s; storing unmodified segment", sessionID)
	}

	// Append transcript segment (even if zero spans)
	segment := TranscriptSegment{
		OriginalText: text,
		MaskedText:   maskedOut,
		Spans:        spans,
		Placeholders: placeholders,
		Timestamp:    time.Now(),
	}
	buffer.TranscriptSegments = append(buffer.TranscriptSegments, segment)

	// Store updated buffer
	redactionBuffer.Store(sessionID, buffer)

	logger.ServiceDebugf("REDACTION", "session %s: appended segment (%d spans, %d placeholders, total segments: %d)",
		sessionID, len(spans), len(placeholders), len(buffer.TranscriptSegments))

	// Debug: log the segment contents
	logger.ServiceDebugf("REDACTION", "Segment for session %s:", sessionID)
	logger.ServiceDebugf("REDACTION", "  OriginalText: '%s'", segment.OriginalText)
	logger.ServiceDebugf("REDACTION", "  MaskedText:   '%s'", segment.MaskedText)
	for i, span := range segment.Spans {
		if i < len(segment.Placeholders) {
			logger.ServiceDebugf("REDACTION", "  Span[%d]: %+v -> %s", i, span, segment.Placeholders[i].Text)
		}
	}

	return maskedOut, nil
}


// formatPlaceholder creates a placeholder string (simplified version)
func formatPlaceholder(span redactor.Span, ordinal int, format string) string {
	switch format {
	case "minimal":
		// Extract just the type from rule ID
		ruleType := extractRuleType(span.RuleID)
		return fmt.Sprintf("[%s#%d]", ruleType, ordinal)
	default:
		return fmt.Sprintf("[%s:%s#%d]", strings.ToUpper(string(span.Kind)), span.RuleID, ordinal)
	}
}

// extractRuleType extracts the main type from a rule ID
func extractRuleType(ruleID string) string {
	if idx := strings.LastIndex(ruleID, ":"); idx > 0 {
		return strings.ToUpper(ruleID[idx+1:])
	}
	return strings.ToUpper(ruleID)
}

// applyMaskingWithPlaceholders applies redaction spans to text using provided placeholders
func applyMaskingWithPlaceholders(text string, spans []redactor.Span, placeholders []redactor.Placeholder) string {
	if len(spans) == 0 {
		return text
	}

	// Sort spans by start position
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Start < spans[j].Start
	})

	var result strings.Builder
	lastEnd := 0

	for i, span := range spans {
		// Add text before the span
		if span.Start > lastEnd {
			result.WriteString(text[lastEnd:span.Start])
		}

		// Add placeholder
		if i < len(placeholders) {
			result.WriteString(placeholders[i].Text)
		} else {
			// Fallback if placeholders don't match
			result.WriteString(formatPlaceholder(span, i+1, "minimal"))
		}

		lastEnd = span.End
	}

	// Add remaining text
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}

	return result.String()
}

// getAccumulatedMaskedTranscript returns the full accumulated masked transcript for LLM context
func (rb *SessionRedactionBuffer) getAccumulatedMaskedTranscript(sessionID string) string {
	buffer, exists := rb.Get(sessionID)
	if !exists || len(buffer.TranscriptSegments) == 0 {
		return ""
	}

	var result strings.Builder
	for i, segment := range buffer.TranscriptSegments {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(segment.MaskedText)
	}

	accumulated := result.String()
	logger.ServiceDebugf("REDACTION", "Accumulated masked transcript for session %s: '%s'", sessionID, accumulated)
	return accumulated
}

// getSegmentCount returns the number of stored transcript segments for a session
func (rb *SessionRedactionBuffer) getSegmentCount(sessionID string) int {
    buffer, exists := rb.Get(sessionID)
    if !exists {
        return 0
    }
    return len(buffer.TranscriptSegments)
}

// getMaskedTranscriptFromSegment returns the concatenated masked transcript from a
// starting segment index (0-based) to the end. If startIdx is out of range, it
// clamps it to valid bounds.
func (rb *SessionRedactionBuffer) getMaskedTranscriptFromSegment(sessionID string, startIdx int) string {
    buffer, exists := rb.Get(sessionID)
    if !exists || len(buffer.TranscriptSegments) == 0 {
        return ""
    }
    if startIdx < 0 { startIdx = 0 }
    if startIdx >= len(buffer.TranscriptSegments) { return "" }
    var result strings.Builder
    for i := startIdx; i < len(buffer.TranscriptSegments); i++ {
        if i > startIdx { result.WriteString(" ") }
        result.WriteString(buffer.TranscriptSegments[i].MaskedText)
    }
    out := result.String()
    logger.ServiceDebugf("REDACTION", "Segmented masked transcript for session %s from idx %d: '%s'", sessionID, startIdx, out)
    return out
}

// appendPlainSegment appends a non-redacted transcript segment to the buffer so
// that transcript accumulation works even when PHI redaction is disabled or the
// redaction service is unavailable. MaskedText equals OriginalText.
func (rb *SessionRedactionBuffer) appendPlainSegment(sessionID, text string) {
    existingBuffer, exists := rb.Get(sessionID)
    var buffer *RedactionBuffer
    if exists {
        buffer = existingBuffer
    } else {
        buffer = &RedactionBuffer{
            SessionID:              sessionID,
            TranscriptSegments:     []TranscriptSegment{},
            NextPlaceholderOrdinal: 1,
        }
    }

    segment := TranscriptSegment{
        OriginalText: text,
        MaskedText:   text,
        Spans:        nil,
        Placeholders: nil,
        Timestamp:    time.Now(),
    }
    buffer.TranscriptSegments = append(buffer.TranscriptSegments, segment)
    rb.Store(sessionID, buffer)
    logger.ServiceDebugf("REDACTION", "session %s: appended plain segment (total segments: %d)", sessionID, len(buffer.TranscriptSegments))
}

// ShouldRedactBeforeLLM determines if redaction should be applied before LLM processing
func ShouldRedactBeforeLLM(deps Deps) bool {
	return deps.RedactionService != nil
}