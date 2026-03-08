package redaction

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	redactor "schma.ai/internal/domain/redaction"
)

var _ redactor.PersistenceGuard = (*PersistenceGuard)(nil)

type PersistenceGuard struct {
	policy redactor.EffectivePolicy
}

func NewPersistenceGuard(policy redactor.EffectivePolicy) *PersistenceGuard {
	return &PersistenceGuard{
		policy: policy,
	}
}

func (g *PersistenceGuard) AllowStorePHI() bool {
	return g.policy.StorePHI
}

func (g *PersistenceGuard) AllowStoreRawTranscripts() bool {
	return g.policy.StoreRawTranscripts
}

func (g *PersistenceGuard) FilterBeforeSave(kind redactor.Kind, data any) (any, error) {
	// Check if we should store this type of data
	switch kind {
	case redactor.KindPHI:
		if !g.policy.StorePHI {
			return nil, fmt.Errorf("PHI storage not allowed by policy")
		}
	case redactor.KindPII:
		if g.policy.RedactPIIStrict {
			return nil, fmt.Errorf("PII storage not allowed by strict policy")
		}
	case redactor.KindPCI:
		return nil, fmt.Errorf("PCI storage never allowed")
	}
	
	// Deep copy and filter the data
	filtered, err := g.deepFilter(data)
	if err != nil {
		return nil, fmt.Errorf("failed to filter data: %w", err)
	}
	
	return filtered, nil
}

func (g *PersistenceGuard) deepFilter(data any) (any, error) {
	if data == nil {
		return nil, nil
	}
	
	v := reflect.ValueOf(data)
	
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil, nil
		}
		filtered, err := g.deepFilter(v.Elem().Interface())
		if err != nil {
			return nil, err
		}
		return filtered, nil
		
	case reflect.Struct:
		result := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldType := v.Type().Field(i)
			
			// Check for sensitive field tags
			if g.isSensitiveField(fieldType) {
				// Replace with placeholder
				result.Field(i).SetString("[REDACTED]")
			} else {
				// Recursively filter
				filtered, err := g.deepFilter(field.Interface())
				if err != nil {
					return nil, err
				}
				if filtered != nil {
					result.Field(i).Set(reflect.ValueOf(filtered))
				}
			}
		}
		return result.Interface(), nil
		
	case reflect.Map:
		result := reflect.MakeMap(v.Type())
		for _, key := range v.MapKeys() {
			value := v.MapIndex(key)
			
			// Check if key indicates sensitive data
			if g.isSensitiveKey(key.String()) {
				result.SetMapIndex(key, reflect.ValueOf("[REDACTED]"))
			} else {
				filtered, err := g.deepFilter(value.Interface())
				if err != nil {
					return nil, err
				}
				if filtered != nil {
					result.SetMapIndex(key, reflect.ValueOf(filtered))
				}
			}
		}
		return result.Interface(), nil
		
	case reflect.Slice, reflect.Array:
		result := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			filtered, err := g.deepFilter(v.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			if filtered != nil {
				result.Index(i).Set(reflect.ValueOf(filtered))
			}
		}
		return result.Interface(), nil
		
	case reflect.String:
		// Check if string contains sensitive patterns
		if g.containsSensitivePatterns(v.String()) {
			return "[REDACTED]", nil
		}
		return v.Interface(), nil
		
	default:
		return v.Interface(), nil
	}
}

func (g *PersistenceGuard) isSensitiveField(field reflect.StructField) bool {
	// Check for sensitive field tags
	tag := field.Tag.Get("redaction")
	return tag == "sensitive" || tag == "phi" || tag == "pii" || tag == "pci"
}

func (g *PersistenceGuard) isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"email", "phone", "address", "ssn", "credit_card", "password",
		"name", "dob", "mrn", "medicare", "account", "id",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}

func (g *PersistenceGuard) containsSensitivePatterns(text string) bool {
	// Simple pattern matching for sensitive data
	patterns := []string{
		`\b\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\b`, // Credit card
		`\b\d{3}-\d{2}-\d{4}\b`,             // SSN
		`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`, // Email
		`\b\d{10}\b`,                        // Phone numbers
	}
	
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}
	return false
}