// auth/key.go
package auth

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// LoadSigningKey returns the raw bytes for HS256 HMAC.
// Accepts base64, base64url, hex, or raw string (last resort).
func LoadSigningKeyFromEnv(envName string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return nil, fmt.Errorf("%s not set", envName)
	}

	// Try standard base64
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) > 0 {
		return b, nil
	}
	// Try URL-safe base64
	if b, err := base64.RawURLEncoding.DecodeString(raw); err == nil && len(b) > 0 {
		return b, nil
	}
	// Try hex (openssl rand -hex 32)
	if b, err := hex.DecodeString(raw); err == nil && len(b) > 0 {
		return b, nil
	}
	// Fallback: use raw bytes of the string (NOT recommended, but works)
	return []byte(raw), nil
}
