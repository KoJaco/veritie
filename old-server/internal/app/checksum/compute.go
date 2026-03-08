package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"schma.ai/internal/domain/speech"
)

// ComputeSchemaChecksum is stored in `checksum` column when inserting
func ComputeSchemaChecksum(params any) (string, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// Reuses existing JSON marshaling and SHA256 logic for consistency
func ComputeFunctionsContextChecksum(tools []speech.FunctionDefinition) (string, error) {
    
    // Reuse existing logic from ComputeSchemaChecksum
    b, err := json.Marshal(tools)
    if err != nil {
        return "", err
    }
    
    h := sha256.Sum256(b)
    return hex.EncodeToString(h[:]), nil
}

// ComputeStructuredContextChecksum computes a checksum for structured output caching
// Uses the same pattern as ComputeContextChecksum but for schema + parsing guide
func ComputeStructuredContextChecksum(schemaJSON []byte) (string, error) {
    // Create combined structure for consistent hashing
 
    
    // Reuse existing logic from ComputeSchemaChecksum
    b, err := json.Marshal(schemaJSON)
    if err != nil {
        return "", err
    }
    
    h := sha256.Sum256(b)
    return hex.EncodeToString(h[:]), nil
}
