package solver

import (
	"strings"
	"testing"
)

// TestFailureHint covers the three contracts of FailureHint: the
// guardrail signature gains the actionable hint, everything else is
// returned byte-identical, and empty stays empty (failure_reason is
// written verbatim into the queue row).
func TestFailureHint(t *testing.T) {
	const rawGuardrail = "solver: pi agent exited 1: No endpoints available matching your guardrail restrictions and data policy."

	t.Run("guardrail text gains hint with URL", func(t *testing.T) {
		got := FailureHint(rawGuardrail)
		if !strings.Contains(got, "openrouter.ai/settings/privacy") {
			t.Errorf("hint missing privacy URL: %q", got)
		}
		// The original provider text must survive verbatim as a prefix.
		if !strings.HasPrefix(got, rawGuardrail) {
			t.Errorf("original error text was rewritten: %q", got)
		}
		if got == rawGuardrail {
			t.Error("guardrail text returned unchanged, want hint appended")
		}
		if len(strings.Split(strings.TrimRight(got, "\n"), "\n")) != 2 {
			t.Errorf("want exactly one appended hint line, got %q", got)
		}
	})

	t.Run("unrelated text is byte-identical", func(t *testing.T) {
		const other = "solve: bwrap: sandbox exited 137 (timeout after 300s)"
		if got := FailureHint(other); got != other {
			t.Errorf("FailureHint(unrelated) = %q, want unchanged %q", got, other)
		}
	})

	t.Run("empty stays empty", func(t *testing.T) {
		if got := FailureHint(""); got != "" {
			t.Errorf("FailureHint(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("already-hinted text is not stacked", func(t *testing.T) {
		once := FailureHint(rawGuardrail)
		if twice := FailureHint(once); twice != once {
			t.Errorf("hint stacked twice:\nonce:  %q\ntwice: %q", once, twice)
		}
	})

	t.Run("near-miss wording is untouched", func(t *testing.T) {
		// Different provider failure that merely mentions guardrails.
		const nearMiss = "rate limit exceeded for provider with guardrails enabled"
		if got := FailureHint(nearMiss); got != nearMiss {
			t.Errorf("FailureHint(near-miss) = %q, want unchanged %q", got, nearMiss)
		}
	})
}
