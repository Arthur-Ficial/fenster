// Package tokens estimates token counts. The Gemini Nano Prompt API
// reports exact counts via session.measureInputUsage() and chunk metadata;
// this package is the host-side fallback for cases where the API doesn't
// expose a counter, and is used to populate the Usage block honestly.
//
// The heuristic is conservative: ceil(len(s) / 4) for ASCII-ish text. The
// pytest suite checks invariants (total = prompt+completion; longer prefix
// => more tokens), not exact values.
package tokens

import "unicode/utf8"

// Estimate returns a token-count estimate for s. 0 for empty; ceil(len/4)
// otherwise (with a minimum of 1).
func Estimate(s string) int {
	if s == "" {
		return 0
	}
	n := utf8.RuneCountInString(s)
	t := (n + 3) / 4
	if t < 1 {
		t = 1
	}
	return t
}

// EstimateMessages sums Estimate over the slice and returns the total.
func EstimateMessages(parts []string) int {
	total := 0
	for _, p := range parts {
		total += Estimate(p)
	}
	return total
}

// Usage is the host-side aggregator before we marshal wire.Usage.
type Usage struct {
	Prompt     int
	Completion int
}

// Total returns the sum.
func (u Usage) Total() int { return u.Prompt + u.Completion }
