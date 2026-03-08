package redaction

import (
	"context"
	"fmt"
	"testing"

	redactor "schma.ai/internal/domain/redaction"
	"schma.ai/internal/infra/redaction"

	onnx "github.com/yalue/onnxruntime_go"
)

// stubNormalizer is a test implementation that just returns the original text
type stubNormalizer struct{}

func (n *stubNormalizer) Normalize(ctx context.Context, text string) (normalized string, err error) {
	// For testing, just return the original text with a simple alignment
	return text, nil
}

func (n *stubNormalizer) NormalizeBatch(ctx context.Context, texts []string) (normalized []string, err error) {
	// For testing, just return the original text with a simple alignment
	return texts, nil
}

func (n *stubNormalizer) Healthy(ctx context.Context) bool {
	return true
}

type stubAlignment struct {
	originalText string
}

func (a *stubAlignment) ToRawSpan(normStart, normEnd int) (rawStart, rawEnd int, ok bool) {
	// Simple 1:1 mapping for testing
	if normStart >= 0 && normEnd <= len(a.originalText) && normStart <= normEnd {
		return normStart, normEnd, true
	}
	return 0, 0, false
}

func (a *stubAlignment) ToNormSpan(rawStart, rawEnd int) (normStart, normEnd int, ok bool) {
	// Simple 1:1 mapping for testing
	if rawStart >= 0 && rawEnd <= len(a.originalText) && rawStart <= rawEnd {
		return rawStart, rawEnd, true
	}
	return 0, 0, false
}

// TestPHIRedaction tests the PHI redaction functionality
func TestPHIRedaction(t *testing.T) {
	// Skip if not in test environment (requires models)
	if testing.Short() {
		t.Skip("Skipping PHI redaction test in short mode")
	}

	// 1. Set the shared library path
	runtimePath := "/home/kori/dev/business/memonic/server/runtime/onnxruntime-linux-x64-1.17.0/lib/libonnxruntime.so.1.17.0"
	onnx.SetSharedLibraryPath(runtimePath)

	// 2. Initialize the ONNX environment
	err := onnx.InitializeEnvironment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize ONNX environment: %v", err))
	}

	// 3. Create a session for the BGE-small model (using dynamic session, should I switch to NewAdvnacedDynamicSession)
	_, err = onnx.NewDynamicSession[int64, float32](
		"/home/kori/dev/business/memonic/server/models/phi_roberta_onnx_int8/model_quantized.onnx",
		[]string{"input_ids", "attention_mask"},
		[]string{"sentence_embedding"},
	)

	if err != nil {
		panic(fmt.Sprintf("Failed to create ONNX session: %v", err))
	}


	// todo: create onnx dynamic session

	// Create test config
	config := &RedactionConfig{
		Redaction: RedactionPolicyConfig{
			Mode:              ModeA_HIPAA,
			RedactPHI:         true,
			RedactPIIStrict:   true,
			RedactPCI:         true,
			StorePHI:          false,
			StoreRawTranscripts: false,
			Locale:            "AU",
		},
		Models: ModelPaths{
			PHI_ONNX: "/home/kori/dev/business/memonic/server/models/phi_roberta_onnx_int8",
			PII_ONNX: "/home/kori/dev/business/memonic/server/models/pii_int8", // TODO: implement PII model
		},
	}

	// Create service
	service, err := NewService(config, &stubNormalizer{})
	if err != nil {
		t.Skipf("Skipping test - failed to create redaction service: %v", err)
	}
	defer service.Close()

	t.Run("Transcript Redaction", func(t *testing.T) {
		testTranscriptRedaction(t, service)
	})

	t.Run("Function Args Redaction", func(t *testing.T) {
		testFunctionArgsRedaction(t, service)
	})

	t.Run("Structured Output Redaction", func(t *testing.T) {
		testStructuredOutputRedaction(t, service)
	})
}

func testTranscriptRedaction(t *testing.T, service *Service) {
	testText := "Michael Nguyen (DOB: 2001-07-14, age 23) presented to Royal North Shore Hospital, Sydney NSW. Contact: 0401 234 567; email michael.nguyen@example.com. MRN 12345678."
	
	result, err := service.RedactTranscript("test-session", testText)
	if err != nil {
		t.Fatalf("Failed to redact transcript: %v", err)
	}

	t.Logf("✅ Transcript redaction successful: found %d spans", len(result.Spans))
	
	// Test different placeholder formats
	verboseMasked := redaction.ApplyMaskingWithFormat(testText, result.Spans, redaction.PlaceholderVerbose)
	compactMasked := redaction.ApplyMaskingWithFormat(testText, result.Spans, redaction.PlaceholderCompact)
	minimalMasked := redaction.ApplyMaskingWithFormat(testText, result.Spans, redaction.PlaceholderMinimal)
	
	t.Logf("🔒 Verbose masked: %s", verboseMasked)
	t.Logf("🔒 Compact masked: %s", compactMasked)
	t.Logf("🔒 Minimal masked: %s", minimalMasked)

	// Basic assertions
	if len(result.Spans) == 0 {
		t.Error("Expected to find PHI spans in test text")
	}

	// Check that we found expected PHI types
	foundTypes := make(map[string]bool)
	for _, span := range result.Spans {
		foundTypes[string(span.Kind)] = true
		t.Logf("Found span: %s [%d:%d] (confidence: %.2f, rule: %s)", 
			span.Kind, span.Start, span.End, span.Confidence, span.RuleID)
	}

	expectedTypes := []string{"phi"}
	for _, expected := range expectedTypes {
		if !foundTypes[expected] {
			t.Errorf("Expected to find %s spans", expected)
		}
	}
}

func testFunctionArgsRedaction(t *testing.T, service *Service) {
	testArgs := map[string]interface{}{
		"patient_name": "Sarah Johnson",
		"dob": "1985-03-22",
		"phone": "+61 2 9123 4567",
		"email": "sarah.johnson@example.com",
		"mrn": "12345678",
	}
	
	// Test the infra redactor directly for detailed span testing
	if phiRedactor, ok := service.phiRedactor.(*redaction.PHIRedactor); ok {
		spans, err := phiRedactor.RedactFunctionArgs(testArgs)
		if err != nil {
			t.Fatalf("Failed to get spans from PHI redactor: %v", err)
		}

		t.Logf("✅ PHI redactor found %d spans", len(spans))
		
		// Show the flattened text and different placeholder formats
		argsText := redaction.FlattenMap(testArgs)
		t.Logf("🔒 Flattened function args text: %s", argsText)
		
		verboseMasked := redaction.ApplyMaskingWithFormat(argsText, spans, redaction.PlaceholderVerbose)
		minimalMasked := redaction.ApplyMaskingWithFormat(argsText, spans, redaction.PlaceholderMinimal)
		
		t.Logf("🔒 Verbose function args: %s", verboseMasked)
		t.Logf("🔒 Minimal function args: %s", minimalMasked)

		// Basic assertions
		if len(spans) == 0 {
			t.Error("Expected to find PHI spans in function args")
		}

		// Log individual spans
		for i, span := range spans {
			t.Logf("Function args span %d: %s [%d:%d] (confidence: %.2f, rule: %s)", 
				i+1, span.Kind, span.Start, span.End, span.Confidence, span.RuleID)
		}
	} else {
		t.Skip("PHI redactor not available for testing")
	}
	
	// Test the service method with a policy that allows storage
	// Create a temporary policy that allows storage for testing
	service.SetSessionPolicy("test-session", redactor.EffectivePolicy{
		RedactPHI: true,
		RedactPIIStrict: false, // Allow PII storage for testing
		RedactPCI: true,
		StorePHI: true, // Allow PHI storage for testing
		StoreRawTranscripts: true,
		Locale: "AU",
		PriorityOrder: []redactor.Kind{"pci", "phi", "pii"},
	})
	
	result, err := service.RedactFunctionArgs("test-session", testArgs)
	if err != nil {
		t.Logf("Service method failed (expected due to policy): %v", err)
	} else {
		t.Logf("✅ Service method successful: filtered result has %d keys", len(result))
	}
}

func testStructuredOutputRedaction(t *testing.T, service *Service) {
	testOutput := map[string]interface{}{
		"patient_details": "name: Dr. Emily Chen, contact: +61 4 1234 5678, email: emily.chen@hospital.com",
		"diagnosis": "Hypertension",
		"medication": "Lisinopril 10mg",
	}
	
	// Test the infra redactor directly for detailed span testing
	if phiRedactor, ok := service.phiRedactor.(*redaction.PHIRedactor); ok {
		spans, err := phiRedactor.RedactStructuredOutput(testOutput)
		if err != nil {
			t.Fatalf("Failed to get spans from PHI redactor: %v", err)
		}

		t.Logf("✅ PHI redactor found %d spans", len(spans))
		
		// Show different placeholder formats
		outputText := redaction.FlattenMap(testOutput)
		verboseMasked := redaction.ApplyMaskingWithFormat(outputText, spans, redaction.PlaceholderVerbose)
		minimalMasked := redaction.ApplyMaskingWithFormat(outputText, spans, redaction.PlaceholderMinimal)
		
		t.Logf("🔒 Verbose structured output: %s", verboseMasked)
		t.Logf("🔒 Minimal structured output: %s", minimalMasked)

		// Basic assertions
		if len(spans) == 0 {
			t.Error("Expected to find PHI spans in structured output")
		}

		// Log individual spans
		for i, span := range spans {
			t.Logf("Structured output span %d: %s [%d:%d] (confidence: %.2f, rule: %s)", 
				i+1, span.Kind, span.Start, span.End, span.Confidence, span.RuleID)
		}
	} else {
		t.Skip("PHI redactor not available for testing")
	}
	
	// Test the service method with a policy that allows storage
	service.SetSessionPolicy("test-session", redactor.EffectivePolicy{
		RedactPHI: true,
		RedactPIIStrict: false,
		RedactPCI: true,
		StorePHI: true, // Allow PHI storage for testing
		StoreRawTranscripts: true,
		Locale: "AU",
		PriorityOrder: []redactor.Kind{"pci", "phi", "pii"},
	})
	
	result, err := service.RedactStructuredOutput("test-session", testOutput)
	if err != nil {
		t.Logf("Service method failed (expected due to policy): %v", err)
	} else {
		t.Logf("✅ Service method successful: filtered result has %d keys", len(result))
	}
}

// TestPlaceholderFormats tests the different placeholder formatting options
func TestPlaceholderFormats(t *testing.T) {
	// Create test spans
	testSpans := []redactor.Span{
		{Start: 0, End: 5, Kind: "phi", Confidence: 0.95, RuleID: "model:PATIENT"},
		{Start: 10, End: 15, Kind: "phi", Confidence: 0.98, RuleID: "rule:DATE"},
		{Start: 20, End: 25, Kind: "phi", Confidence: 0.92, RuleID: "rule:EMAIL"},
	}

	testText := "John Doe was born on 1990-01-01 and can be reached at john@example.com"

	t.Run("Verbose Format", func(t *testing.T) {
		result := redaction.ApplyMaskingWithFormat(testText, testSpans, redaction.PlaceholderVerbose)
		t.Logf("Verbose: %s", result)
		
		// Should contain full format
		if !contains(result, "[PHI:model:PATIENT#1]") {
			t.Error("Verbose format should contain full placeholder")
		}
	})

	t.Run("Compact Format", func(t *testing.T) {
		result := redaction.ApplyMaskingWithFormat(testText, testSpans, redaction.PlaceholderCompact)
		t.Logf("Compact: %s", result)
		
		// Should contain rule ID only
		if !contains(result, "[model:PATIENT#1]") {
			t.Error("Compact format should contain rule ID only")
		}
	})

	t.Run("Minimal Format", func(t *testing.T) {
		result := redaction.ApplyMaskingWithFormat(testText, testSpans, redaction.PlaceholderMinimal)
		t.Logf("Minimal: %s", result)
		
		// Should contain just the type
		if !contains(result, "[PATIENT#1]") {
			t.Error("Minimal format should contain just the type")
		}
		if !contains(result, "[DATE#2]") {
			t.Error("Minimal format should contain just the type")
		}
	})
}

// TestFlattenMap tests the map flattening functionality
func TestFlattenMap(t *testing.T) {
	t.Run("Simple Map", func(t *testing.T) {
		testMap := map[string]interface{}{
			"name": "John Doe",
			"age":  "30",
		}
		
		result := redaction.FlattenMap(testMap)
		expected := "name John Doe age 30"
		
		if result != expected {
			t.Errorf("Expected '%s', got '%s'", expected, result)
		}
		t.Logf("Flattened: %s", result)
	})

	t.Run("Nested Map", func(t *testing.T) {
		testMap := map[string]interface{}{
			"patient": map[string]interface{}{
				"name": "Jane Smith",
				"contact": "+61 2 9123 4567",
			},
			"diagnosis": "Hypertension",
		}
		
		result := redaction.FlattenMap(testMap)
		t.Logf("Nested flattened: %s", result)
		
		// Should contain all elements
		if !contains(result, "patient") || !contains(result, "name") || !contains(result, "Jane Smith") {
			t.Error("Nested map should be flattened correctly")
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}
