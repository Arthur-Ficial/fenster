// Package validate enforces every apfel rejection rule on incoming chat
// completion requests. Returning a non-nil error means the request must be
// rejected with the sentinel's HTTP status and Envelope.
package validate

import (
	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// Request runs every rejection rule. Order matches apfel for parity with
// pytest's "first invalid field wins" assertions.
func Request(r *wire.ChatCompletionRequest) error {
	if r == nil || len(r.Messages) == 0 {
		return cerr.ErrEmptyMessages
	}
	for _, m := range r.Messages {
		if m.Content != nil && m.Content.HasImage() {
			return cerr.ErrImageContent
		}
	}
	if r.Logprobs != nil && *r.Logprobs {
		return cerr.ErrLogprobs
	}
	if r.N != nil && *r.N != 1 {
		return cerr.ErrN
	}
	if r.PresencePenalty != nil {
		return cerr.ErrPresencePenalty
	}
	if r.FrequencyPenalty != nil {
		return cerr.ErrFrequencyPenalty
	}
	if r.Stop != nil {
		return cerr.ErrStop
	}
	if r.MaxTokens != nil && *r.MaxTokens <= 0 {
		return cerr.ErrMaxTokensInvalid
	}
	if r.Temperature != nil && *r.Temperature < 0 {
		return cerr.ErrTemperatureInvalid
	}
	return nil
}
