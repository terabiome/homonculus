package k3s

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateToken generates a secure random K3s cluster token.
// K3s tokens can be any string, but we generate a cryptographically secure random token.
func GenerateToken() (string, error) {
	// Generate 32 bytes (256 bits) of random data
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode to base64 URL-safe format (no padding)
	// This format is compatible with K3s and safe for URLs/configs
	token := base64.RawURLEncoding.EncodeToString(b)

	return token, nil
}

