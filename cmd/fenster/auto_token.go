package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// autoToken returns a hex-encoded random token (legacy).
func autoToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "fenster-fallback-token"
	}
	return hex.EncodeToString(b)
}

// autoTokenUUID returns a UUID-v4-shaped token. apfel's --token-auto
// emits this shape; security_test's regex `[0-9A-Fa-f-]{36}` expects 36
// hex+dash chars (8-4-4-4-12).
func autoTokenUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	// Set version (4) and variant (10).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
