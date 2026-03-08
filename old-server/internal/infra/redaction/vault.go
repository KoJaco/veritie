package redaction

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	redactor "schma.ai/internal/domain/redaction"
)

var _ redactor.Vault = (*Vault)(nil)

type Vault struct {
	mu       sync.RWMutex
	store    map[string]string // placeholder -> encrypted value
	key      []byte            // AES key for encryption
	ordinals map[redactor.Kind]int // per-kind ordinal counter
}

func NewVault(key []byte) *Vault {
	if len(key) != 32 {
		panic("vault key must be 32 bytes for AES-256")
	}
	
	return &Vault{
		store:    make(map[string]string),
		key:      key,
		ordinals: make(map[redactor.Kind]int),
	}
}

func (v *Vault) Put(kind redactor.Kind, rawValue string) (ph redactor.Placeholder, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	
	// Increment ordinal for this kind
	v.ordinals[kind]++
	ordinal := v.ordinals[kind]
	
	// Encrypt the raw value
	encrypted, err := v.encrypt(rawValue)
	if err != nil {
		return redactor.Placeholder{}, fmt.Errorf("failed to encrypt value: %w", err)
	}
	
	// Create placeholder
	placeholder := fmt.Sprintf("[%s#%d]", strings.ToUpper(string(kind)), ordinal)
	
	// Store encrypted value
	v.store[placeholder] = encrypted
	
	return redactor.Placeholder{
		Kind:    kind,
		Ordinal: ordinal,
		Text:    placeholder,
	}, nil
}

func (v *Vault) Get(ph redactor.Placeholder) (rawValue string, ok bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	
	encrypted, exists := v.store[ph.Text]
	if !exists {
		return "", false
	}
	
	// Decrypt the value
	decrypted, err := v.decrypt(encrypted)
	if err != nil {
		return "", false
	}
	
	return decrypted, true
}

func (v *Vault) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	
	// Clear all stored data
	v.store = make(map[string]string)
	v.ordinals = make(map[redactor.Kind]int)
}

func (v *Vault) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	
	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// Encode as base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (v *Vault) decrypt(encrypted string) (string, error) {
	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	
	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}