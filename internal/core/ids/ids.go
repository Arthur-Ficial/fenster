// Package ids generates the id-shaped identifiers OpenAI clients expect:
// "chatcmpl-<random>" for completions, "call_<random>" for tool calls.
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

// ChatCompletion returns a freshly-generated chat completion id.
func ChatCompletion() string { return "chatcmpl-" + randomHex(12) }

// ToolCall returns a freshly-generated tool call id.
func ToolCall() string { return "call_" + randomHex(8) }

func randomHex(n int) string {
	// n is the number of output hex chars we want, so we need n/2 bytes.
	bytes := (n + 1) / 2
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		// rand.Read on Linux/macOS only fails if the kernel CSPRNG is
		// unavailable, which is unrecoverable. Don't pretend otherwise.
		panic(err)
	}
	return hex.EncodeToString(b)[:n]
}
