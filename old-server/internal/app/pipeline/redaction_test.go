package pipeline

import (
	"testing"
	"time"

	redactor "schma.ai/internal/domain/redaction"
)

func TestRedactionBuffer(t *testing.T) {
	// Create a new session redaction buffer
	rb := NewSessionRedactionBuffer()

	// Test data
	sessionID := "test-session-123"
	originalText := "Patient John Doe (DOB: 1990-01-01) has phone +61 2 9123 4567"
	
	// Create test spans with correct coordinates
	// "Patient John Doe (DOB: 1990-01-01) has phone +61 2 9123 4567"
	//  012345678901234567890123456789012345678901234567890123456789
	spans := []redactor.Span{
		{Start: 8, End: 16, Kind: "phi", Confidence: 0.95, RuleID: "model:PATIENT"},   // "John Doe"
		{Start: 23, End: 33, Kind: "phi", Confidence: 0.98, RuleID: "rule:DATE"},       // "1990-01-01"
		{Start: 45, End: 60, Kind: "phi", Confidence: 0.92, RuleID: "rule:PHONE"},      // "+61 2 9123 4567"
	}
	
	// Create test placeholders
	placeholders := []redactor.Placeholder{
		{Kind: "phi", Ordinal: 1, Text: "[PATIENT#1]"},
		{Kind: "phi", Ordinal: 2, Text: "[DATE#2]"},
		{Kind: "phi", Ordinal: 3, Text: "[PHONE#3]"},
	}

	// Create transcript segment and store redaction buffer (segment-based)
	maskedText := "Patient [PATIENT#1] (DOB: [DATE#2]) has phone [PHONE#3]"
	segment := TranscriptSegment{
		OriginalText: originalText,
		MaskedText:   maskedText,
		Spans:        spans,
		Placeholders: placeholders,
		Timestamp:    time.Now(),
	}
	buffer := &RedactionBuffer{
		SessionID:              sessionID,
		TranscriptSegments:     []TranscriptSegment{segment},
		NextPlaceholderOrdinal: 4,
	}
	
	rb.Store(sessionID, buffer)

	// Test retrieval
	retrievedBuffer, exists := rb.Get(sessionID)
	if !exists {
		t.Fatal("Failed to retrieve redaction buffer")
	}
	
	if retrievedBuffer.SessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, retrievedBuffer.SessionID)
	}
	
	if len(retrievedBuffer.TranscriptSegments) != 1 {
		t.Fatalf("Expected 1 transcript segment, got %d", len(retrievedBuffer.TranscriptSegments))
	}
	gotSeg := retrievedBuffer.TranscriptSegments[0]
	if gotSeg.OriginalText != originalText {
		t.Errorf("Expected segment original text %q, got %q", originalText, gotSeg.OriginalText)
	}
	if gotSeg.MaskedText != maskedText {
		t.Errorf("Expected segment masked text %q, got %q", maskedText, gotSeg.MaskedText)
	}
	if len(gotSeg.Spans) != len(spans) {
		t.Errorf("Expected %d spans, got %d", len(spans), len(gotSeg.Spans))
	}
	if len(gotSeg.Placeholders) != len(placeholders) {
		t.Errorf("Expected %d placeholders, got %d", len(placeholders), len(gotSeg.Placeholders))
	}

	// Test text reconstruction
	redactedText := "Patient [PATIENT#1] (DOB: [DATE#2]) has phone [PHONE#3]"
	reconstructedText := rb.reconstructText(sessionID, redactedText)
	
	expectedReconstructed := "Patient John Doe (DOB: 1990-01-01) has phone +61 2 9123 4567"
	if reconstructedText != expectedReconstructed {
		t.Errorf("Expected reconstructed text %s, got %s", expectedReconstructed, reconstructedText)
	}

	// Test function call reconstruction
	testArgs := map[string]interface{}{
		"patient_name": "[PATIENT#1]",
		"dob": "[DATE#2]",
		"phone": "[PHONE#3]",
	}
	
	reconstructedArgs := rb.reconstructArgs(sessionID, testArgs)
	
	expectedArgs := map[string]interface{}{
		"patient_name": "John Doe",
		"dob": "1990-01-01",
		"phone": "+61 2 9123 4567",
	}
	
	// Compare reconstructed args
	for key, expectedValue := range expectedArgs {
		if actualValue, exists := reconstructedArgs[key]; !exists {
			t.Errorf("Missing key %s in reconstructed args", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected %s=%s, got %s=%v", key, expectedValue, key, actualValue)
		}
	}

	// Test clearing
	rb.Clear(sessionID)
	_, exists = rb.Get(sessionID)
	if exists {
		t.Error("Redaction buffer should not exist after clearing")
	}

	t.Logf("✅ All redaction buffer tests passed")
}

func TestRedactionBufferWithNoBuffer(t *testing.T) {
	rb := NewSessionRedactionBuffer()
	
	// Test reconstruction with no buffer
	text := "Some text with [PATIENT#1]"
	reconstructed := rb.reconstructText("non-existent-session", text)
	
	// Should return original text when no buffer exists
	if reconstructed != text {
		t.Errorf("Expected original text %s, got %s", text, reconstructed)
	}
	
	// Test args reconstruction with no buffer
	args := map[string]interface{}{
		"name": "[PATIENT#1]",
	}
	reconstructedArgs := rb.reconstructArgs("non-existent-session", args)
	
	// Should return original args when no buffer exists
	if reconstructedArgs["name"] != "[PATIENT#1]" {
		t.Errorf("Expected original args, got %v", reconstructedArgs)
	}
	
	t.Logf("✅ No buffer tests passed")
}
