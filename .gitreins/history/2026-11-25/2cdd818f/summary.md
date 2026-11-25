# Verdict: DOC-14

**Task:** P5 — README --version example 'off-by-one 0287082' matches no build mode (source build prints 0.3.0-dev; release ldflags inject the tag)
**Evaluated:** 2026-11-25T06:22:12.480664
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
- ✗ **tier2**
  - INCOMPLETE
  ✗ Coverage FAILED — ungraded: Falsifiable: README.md's `--version` row shows the real from-source form (`off-by-one 0.3.0-dev`, matching `go build ./cmd/off-by-one && ./off-by-one --version`) and names the release form (tag injected via -ldflags, e.g. `off-by-one v0.3.0`); the literal 0287082 no longer occurs anywhere in README.md.

GATE ARM (card-manager 2026-11-25T01:47Z): Run `grep -c 0287082 README.md` from the repo root; expect 0. Re-verified at origin/master 839a1f24 this pass: the count is still 1 (the literal sits in the --version example row), so the finding is open and this row is dispatchable as written.
Evaluator error: LLM call failed: LLM request failed after 3 attempts (provider=openai model=glm-5.3-flash url=https://api.z.ai/api/coding/paas/v4/chat/completions key=<GITREINS_LLM_API_KEY>) — last error: 429 Client Error: Too Many Requests for url: https://api.z.ai/api/coding/paas/v4/chat/completions
- ✓ **fix_verification**
  - fix_verification: 0 finding(s) verified

## Summary

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check

Stage tier2: FAIL
  INCOMPLETE
  ✗ Coverage FAILED — ungraded: Falsifiable: README.md's `--version` row shows the real from-source form (`off-by-one 0.3.0-dev`, matching `go build ./cmd/off-by-one && ./off-by-one --version`) and names the release form (tag injected via -ldflags, e.g. `off-by-one v0.3.0`); the literal 0287082 no longer occurs anywhere in README.md.

GATE ARM (card-manager 2026-11-25T01:47Z): Run `grep -c 0287082 README.md` from the repo root; expect 0. Re-verified at origin/master 839a1f24 this pass: the count is still 1 (the literal sits in the --version example row), so the finding is open and this row is dispatchable as written.
Evaluator error: LLM call failed: LLM request failed after 3 attempts (provider=openai model=glm-5.3-flash url=https://api.z.ai/api/coding/paas/v4/chat/completions key=<GITREINS_LLM_API_KEY>) — last error: 429 Client Error: Too Many Requests for url: https://api.z.ai/api/coding/paas/v4/chat/completions

Stage fix_verification: PASS
  fix_verification: 0 finding(s) verified

Overall: FAIL ✗
