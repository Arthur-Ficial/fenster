package tokens

import "testing"

func TestEstimate_EmptyIsZero(t *testing.T) {
	if got := Estimate(""); got != 0 {
		t.Fatalf("empty should be 0, got %d", got)
	}
}

func TestEstimate_LongerStringMoreTokens(t *testing.T) {
	short := Estimate("hi")
	long := Estimate("the quick brown fox jumps over the lazy dog repeatedly")
	if !(long > short) {
		t.Fatalf("longer must produce more tokens (short=%d long=%d)", short, long)
	}
}

func TestEstimate_AlwaysAtLeastOneForNonEmpty(t *testing.T) {
	if Estimate("a") < 1 {
		t.Fatalf("non-empty should be ≥1")
	}
}

func TestUsage_TotalIsSum(t *testing.T) {
	u := Usage{Prompt: 7, Completion: 3}
	if u.Total() != 10 {
		t.Fatalf("Total should sum, got %d", u.Total())
	}
}

func TestEstimateMessages_GrowsMonotonicWithPrefix(t *testing.T) {
	// apfel asserts: with_history.usage.prompt_tokens > without_history.usage.prompt_tokens
	a := EstimateMessages([]string{"hello"})
	b := EstimateMessages([]string{"hello", "and a longer follow up that is meaningful"})
	if !(b > a) {
		t.Fatalf("prefix-extended messages must produce ≥ tokens (a=%d b=%d)", a, b)
	}
}
