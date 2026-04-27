// Package tokens estimates token counts. Real engines (FoundationModels in
// apfel, Gemini Nano via the Prompt API in fenster) report exact counts; this
// package is the fallback when the engine doesn't expose a counter, and the
// host-side estimator we use to populate the Usage block honestly.
//
// The heuristic is conservative: ceil(len(s) / 4) for ASCII-ish text. Apfel's
// pytest suite checks invariants (total = prompt+completion; longer prefix =>
// more tokens), not exact values.
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
