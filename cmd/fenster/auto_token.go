package main

import (
	"crypto/rand"
	"encoding/hex"
)

func autoToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "fenster-fallback-token"
	}
	return hex.EncodeToString(b)
}
