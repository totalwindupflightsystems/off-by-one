package solver

import "strings"

// guardrailSignature is the stable part of the error OpenRouter returns
// when a key's privacy/data-policy guardrails leave no eligible provider
// for the requested model:
//
//	No endpoints available matching your guardrail restrictions and data policy.
//
// We match the prefix only — the trailing wording is provider-side and
// varies — and the rest of the raw text stays untouched in the stored
// failure reason.
const guardrailSignature = "No endpoints available matching your guardrail restrictions"

// guardrailHint is the actionable line appended to a guardrail failure.
// It names the setting that caused the block and the page that exposes
// it, so a submitter reading failure_reason knows what to change instead
// of seeing only the provider's raw text (DF-OFF-BY-ONE-5). The URL is
// deliberately the account-level privacy page: the block is a
// key/provider-allowlist decision, not a model or prompt problem.
const guardrailHint = "hint: provider routing was blocked by this key's privacy/data-policy guardrails — " +
	"allow an eligible provider for the model at https://openrouter.ai/settings/privacy " +
	"(or retry with a key that has no data-policy guardrail)"

// FailureHint turns a raw solver error into a failure reason that is
// useful to the submitter. Unknown errors are returned byte-identical;
// only text carrying a known signature gains an appended hint line, so
// the original error is never hidden or rewritten.
//
// Empty input returns empty: callers persist the result directly into
// queue_entries.failure_reason, and an empty reason must stay empty.
func FailureHint(errText string) string {
	if errText == "" {
		return ""
	}
	if !strings.Contains(errText, guardrailSignature) {
		return errText
	}
	// Idempotent: never stack the hint on a reason that already carries
	// it (the stored text is re-read by operators and re-processed by
	// retry paths).
	if strings.Contains(errText, "openrouter.ai/settings/privacy") {
		return errText
	}
	return errText + "\n" + guardrailHint
}
