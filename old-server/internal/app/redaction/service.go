package redaction

import (
	"crypto/rand"
	"fmt"
	"sort"
	"sync"

	"schma.ai/internal/domain/normalizer"
	redactor "schma.ai/internal/domain/redaction"
	redaction_infra "schma.ai/internal/infra/redaction"
)

type Service struct {
	config     *RedactionConfig
	normalizer normalizer.Normalizer
	phiRedactor redactor.Redactor
	piiRedactor redactor.Redactor
	pciRedactor redactor.Redactor
	orchestrator redactor.RedactionOrchestrator
	policyResolver redactor.PolicyResolver
	persistenceGuard redactor.PersistenceGuard
	vault redactor.Vault
	
	mu sync.RWMutex
	sessionVaults map[string]redactor.Vault
}

func NewService(config *RedactionConfig, normalizer normalizer.Normalizer) (*Service, error) {
	// Generate encryption key for vault
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate vault key: %w", err)
	}
	
	// Create vault
	vault := redaction_infra.NewVault(key)
	
	// Create PHI redactor
	phiRedactor, err := redaction_infra.NewPHIRedactor(
		config.Models.PHI_ONNX + "/model_quantized.onnx",
		config.Models.PHI_ONNX + "/config.json", // Assuming config is in same dir
		"/tmp/tok.sock", // Tokenizer socket path
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create PHI redactor: %w", err)
	}
	
	// Create default policy
	defaultPolicy := redaction_infra.CreateDefaultPolicy(string(config.Redaction.Mode))
	
	// Create orchestrator
	orchestrator := redaction_infra.NewOrchestrator(vault)
	
	// Create policy resolver
	policyResolver := redaction_infra.NewPolicyResolver(defaultPolicy)
	
	// Create persistence guard
	persistenceGuard := redaction_infra.NewPersistenceGuard(defaultPolicy)
	
	return &Service{
		config: config,
		normalizer: normalizer,
		phiRedactor: phiRedactor,
		piiRedactor: &redaction_infra.PIIRedactor{}, // TODO: implement
		pciRedactor: &redaction_infra.PCIRedactor{}, // TODO: implement
		orchestrator: orchestrator,
		policyResolver: policyResolver,
		persistenceGuard: persistenceGuard,
		vault: vault,
		sessionVaults: make(map[string]redactor.Vault),
	}, nil
}

// RedactTranscript performs complete redaction on a transcript
func (s *Service) RedactTranscript(sessionID, text string) (*redactor.RedactionResult, error) {
	// Get policy for this session
	policy, err := s.policyResolver.Resolve(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policy: %w", err)
	}
	
	// Normalization should occur within the pipeline, not within redaction.
	
	// Get session vault (for future use)
	_ = s.getSessionVault(sessionID)
	
	// Collect spans from all redactors
	var spanSets [][]redactor.Span
	
	// PHI redaction
	if policy.RedactPHI {
		phiSpans, err := s.phiRedactor.RedactTranscript(text)
		if err != nil {
			return nil, fmt.Errorf("PHI redaction failed: %w", err)
		}
		spanSets = append(spanSets, phiSpans)
	}
	
	// PII redaction
	if policy.RedactPIIStrict {
		piiSpans, err := s.piiRedactor.RedactTranscript(text)
		if err != nil {
			return nil, fmt.Errorf("PII redaction failed: %w", err)
		}
		spanSets = append(spanSets, piiSpans)
	}
	
	// PCI redaction
	if policy.RedactPCI {
		pciSpans, err := s.pciRedactor.RedactTranscript(text)
		if err != nil {
			return nil, fmt.Errorf("PCI redaction failed: %w", err)
		}
		spanSets = append(spanSets, pciSpans)
	}
	
	// Apply orchestration
	result, err := s.orchestrator.Apply(text, spanSets...)
	if err != nil {
		return nil, fmt.Errorf("orchestration failed: %w", err)
	}
	
	return &result, nil
}

// RedactTrascript implements the Redactor interface for the service
// This method performs redaction and returns spans for the pipeline
func (s *Service) RedactTrascript(text string) ([]redactor.Span, error) {
	// For pipeline usage, we need to perform redaction without session context
	// We'll use a default policy and return spans directly
	
	// Collect spans from all redactors with default policy (enable all)
	var spanSets [][]redactor.Span
	
	// PHI redaction
	phiSpans, err := s.phiRedactor.RedactTranscript(text)
	if err != nil {
		return nil, fmt.Errorf("PHI redaction failed: %w", err)
	}
	spanSets = append(spanSets, phiSpans)
	
	// PII redaction
	piiSpans, err := s.piiRedactor.RedactTranscript(text)
	if err != nil {
		return nil, fmt.Errorf("PII redaction failed: %w", err)
	}
	spanSets = append(spanSets, piiSpans)
	
	// PCI redaction
	pciSpans, err := s.pciRedactor.RedactTranscript(text)
	if err != nil {
		return nil, fmt.Errorf("PCI redaction failed: %w", err)
	}
	spanSets = append(spanSets, pciSpans)
	
	// Merge spans with precedence (PCI > PHI > PII)
	allSpans := s.mergeSpansWithPrecedence(spanSets...)
	
	return allSpans, nil
}

// mergeSpansWithPrecedence merges spans with precedence (PCI > PHI > PII)
func (s *Service) mergeSpansWithPrecedence(spanSets ...[]redactor.Span) []redactor.Span {
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
	return s.removeOverlaps(allSpans)
}

// removeOverlaps removes overlapping spans (higher priority wins)
func (s *Service) removeOverlaps(spans []redactor.Span) []redactor.Span {
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
			if s.getPriority(current.Kind) > s.getPriority(last.Kind) {
				// Replace the last span with current
				*last = current
			}
			// If same priority, keep the one with higher confidence
			if s.getPriority(current.Kind) == s.getPriority(last.Kind) && current.Confidence > last.Confidence {
				*last = current
			}
		} else {
			result = append(result, current)
		}
	}
	
	return result
}

// getPriority returns the priority of a kind (PCI > PHI > PII)
func (s *Service) getPriority(kind redactor.Kind) int {
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

// RedactFunctionArgs redacts sensitive data from function arguments
func (s *Service) RedactFunctionArgs(sessionID string, args map[string]interface{}) (map[string]interface{}, error) {
	// Get policy for this session
	_, err := s.policyResolver.Resolve(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policy: %w", err)
	}
	
	// Filter arguments based on policy
	filtered, err := s.persistenceGuard.FilterBeforeSave(redactor.KindPII, args)
	if err != nil {
		return nil, fmt.Errorf("failed to filter arguments: %w", err)
	}
	
	return filtered.(map[string]interface{}), nil
}

// RedactStructuredOutput redacts sensitive data from structured output
func (s *Service) RedactStructuredOutput(sessionID string, output map[string]interface{}) (map[string]interface{}, error) {
	// Get policy for this session
	_, err := s.policyResolver.Resolve(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policy: %w", err)
	}
	
	// Filter output based on policy
	filtered, err := s.persistenceGuard.FilterBeforeSave(redactor.KindPHI, output)
	if err != nil {
		return nil, fmt.Errorf("failed to filter output: %w", err)
	}
	
	return filtered.(map[string]interface{}), nil
}

// GetPlaceholder retrieves the original value for a placeholder
func (s *Service) GetPlaceholder(sessionID string, placeholder redactor.Placeholder) (string, bool) {
	sessionVault := s.getSessionVault(sessionID)
	return sessionVault.Get(placeholder)
}

// ClearSession clears all session-specific data
func (s *Service) ClearSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if vault, exists := s.sessionVaults[sessionID]; exists {
		vault.Clear()
		delete(s.sessionVaults, sessionID)
	}
}

// SetSessionPolicy sets a custom policy for a session
func (s *Service) SetSessionPolicy(sessionID string, policy redactor.EffectivePolicy) {
	if resolver, ok := s.policyResolver.(*redaction_infra.PolicyResolver); ok {
		resolver.SetSessionPolicy(sessionID, policy)
	}
}

func (s *Service) getSessionVault(sessionID string) redactor.Vault {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if vault, exists := s.sessionVaults[sessionID]; exists {
		return vault
	}
	
	// Create new session vault
	key := make([]byte, 32)
	rand.Read(key) // Ignore error for simplicity
	vault := redaction_infra.NewVault(key)
	s.sessionVaults[sessionID] = vault
	
	return vault
}

// Close cleans up resources
func (s *Service) Close() error {
	// Close PHI redactor
	if closer, ok := s.phiRedactor.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("failed to close PHI redactor: %w", err)
		}
	}
	
	// Clear all session vaults
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, vault := range s.sessionVaults {
		vault.Clear()
	}
	s.sessionVaults = make(map[string]redactor.Vault)
	
	return nil
}
