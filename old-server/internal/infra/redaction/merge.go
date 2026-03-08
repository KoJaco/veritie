package redaction

import (
	"fmt"
	"sort"
	"strings"

	redactor "schma.ai/internal/domain/redaction"
)

var _ redactor.RedactionOrchestrator = (*Orchestrator)(nil)

type Orchestrator struct {
	vault redactor.Vault
}

func NewOrchestrator(vault redactor.Vault) *Orchestrator {
	return &Orchestrator{
		vault: vault,
	}
}

func (o *Orchestrator) Apply(text string, spanSets ...[]redactor.Span) (redactor.RedactionResult, error) {
	// 1. Merge all spans with precedence (PCI > PHI > PII)
	allSpans := o.mergeSpansWithPrecedence(spanSets...)
	
	// 2. Since normalization happens pre-redaction, spans are already in the correct coordinate system
	// No need for coordinate conversion
	
	// 3. Sort spans by start position
	sort.Slice(allSpans, func(i, j int) bool {
		return allSpans[i].Start < allSpans[j].Start
	})
	
	// 4. Create placeholders and store in vault
	placeholders := make([]redactor.Placeholder, 0, len(allSpans))
	ordinalCounters := make(map[redactor.Kind]int)
	
	for _, span := range allSpans {
		spanValue := text[span.Start:span.End]
		
		// Create placeholder
		ordinalCounters[span.Kind]++
		placeholder := redactor.Placeholder{
			Kind:    span.Kind,
			Ordinal: ordinalCounters[span.Kind],
			Text:    fmt.Sprintf("[%s:%s#%d]", strings.ToUpper(string(span.Kind)), span.RuleID, ordinalCounters[span.Kind]),
		}
		
		// Store in vault
		if _, err := o.vault.Put(span.Kind, spanValue); err != nil {
			return redactor.RedactionResult{}, fmt.Errorf("failed to store in vault: %w", err)
		}
		
		placeholders = append(placeholders, placeholder)
	}
	
	// 5. Apply redaction to text
	redactedText := o.applyRedaction(text, allSpans, placeholders)
	
	return redactor.RedactionResult{
		Spans:        allSpans,
		Placeholders: placeholders,
		RedactedRaw:  redactedText,
	}, nil
}

func (o *Orchestrator) mergeSpansWithPrecedence(spanSets ...[]redactor.Span) []redactor.Span {
	// Priority order: PCI > PHI > PII
	priorityOrder := []redactor.Kind{redactor.KindPCI, redactor.KindPHI, redactor.KindPII}
	
	var allSpans []redactor.Span
	
	// Process spans in priority order
	for _, kind := range priorityOrder {
		for _, spanSet := range spanSets {
			for _, span := range spanSet {
				if span.Kind == kind {
					allSpans = append(allSpans, span)
				}
			}
		}
	}
	
	// Remove overlapping spans (higher priority wins)
	return o.removeOverlaps(allSpans)
}

func (o *Orchestrator) removeOverlaps(spans []redactor.Span) []redactor.Span {
	if len(spans) == 0 {
		return spans
	}
	
	// Sort by start position
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Start < spans[j].Start
	})
	
	var result []redactor.Span
	result = append(result, spans[0])
	
	for i := 1; i < len(spans); i++ {
		current := spans[i]
		last := &result[len(result)-1]
		
		// Check for overlap
		if current.Start < last.End {
			// Overlap detected - keep the one with higher priority
			if o.getPriority(current.Kind) > o.getPriority(last.Kind) {
				// Replace the last span with current
				*last = current
			}
			// If same priority, keep the one with higher confidence
			if o.getPriority(current.Kind) == o.getPriority(last.Kind) && current.Confidence > last.Confidence {
				*last = current
			}
		} else {
			result = append(result, current)
		}
	}
	
	return result
}

func (o *Orchestrator) getPriority(kind redactor.Kind) int {
	switch kind {
	case redactor.KindPCI:
		return 3
	case redactor.KindPHI:
		return 2
	case redactor.KindPII:
		return 1
	default:
		return 0
	}
}

func (o *Orchestrator) applyRedaction(text string, spans []redactor.Span, placeholders []redactor.Placeholder) string {
	if len(spans) == 0 {
		return text
	}
	
	var result strings.Builder
	lastEnd := 0
	
	for i, span := range spans {
		// Add text before the span
		if span.Start > lastEnd {
			result.WriteString(text[lastEnd:span.Start])
		}
		
		// Add placeholder
		result.WriteString(placeholders[i].Text)
		
		lastEnd = span.End
	}
	
	// Add remaining text
	if lastEnd < len(text) {
		result.WriteString(text[lastEnd:])
	}
	
	return result.String()
}