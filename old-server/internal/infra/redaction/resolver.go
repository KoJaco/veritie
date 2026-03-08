package redaction

import (
	redactor "schma.ai/internal/domain/redaction"
)

var _ redactor.PolicyResolver = (*PolicyResolver)(nil)

type PolicyResolver struct {
	defaultPolicy redactor.EffectivePolicy
	sessionPolicies map[string]redactor.EffectivePolicy
}

func NewPolicyResolver(defaultPolicy redactor.EffectivePolicy) *PolicyResolver {
	return &PolicyResolver{
		defaultPolicy: defaultPolicy,
		sessionPolicies: make(map[string]redactor.EffectivePolicy),
	}
}

func (r *PolicyResolver) Resolve(sessionID string) (redactor.EffectivePolicy, error) {
	// Check for session-specific policy
	if policy, exists := r.sessionPolicies[sessionID]; exists {
		return policy, nil
	}
	
	// Return default policy
	return r.defaultPolicy, nil
}

// SetSessionPolicy allows setting a custom policy for a specific session
func (r *PolicyResolver) SetSessionPolicy(sessionID string, policy redactor.EffectivePolicy) {
	r.sessionPolicies[sessionID] = policy
}

// ClearSessionPolicy removes a session-specific policy
func (r *PolicyResolver) ClearSessionPolicy(sessionID string) {
	delete(r.sessionPolicies, sessionID)
}

// CreateDefaultPolicy creates a default policy based on mode
func CreateDefaultPolicy(mode string) redactor.EffectivePolicy {
	switch mode {
	case "vertex_hipaa":
		return redactor.EffectivePolicy{
			RedactPHI:           true,
			RedactPIIStrict:     true,
			RedactPCI:           true,
			StorePHI:            false,
			StoreRawTranscripts: false,
			Locale:              "AU",
			PriorityOrder:       []redactor.Kind{redactor.KindPCI, redactor.KindPHI, redactor.KindPII},
		}
	case "public":
		return redactor.EffectivePolicy{
			RedactPHI:           true,
			RedactPIIStrict:     false,
			RedactPCI:           true,
			StorePHI:            false,
			StoreRawTranscripts: false,
			Locale:              "AU",
			PriorityOrder:       []redactor.Kind{redactor.KindPCI, redactor.KindPHI, redactor.KindPII},
		}
	default:
		// Conservative default
		return redactor.EffectivePolicy{
			RedactPHI:           true,
			RedactPIIStrict:     true,
			RedactPCI:           true,
			StorePHI:            false,
			StoreRawTranscripts: false,
			Locale:              "AU",
			PriorityOrder:       []redactor.Kind{redactor.KindPCI, redactor.KindPHI, redactor.KindPII},
		}
	}
}


// - Loads policy from Config
// - Mode (B default), redactPHI(true), redactPII(strict?), redactPCI(true), locale, priority order
