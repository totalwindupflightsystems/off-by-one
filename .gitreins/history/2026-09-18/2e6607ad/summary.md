# Verdict: OB-GAP-076

**Task:** P3 docs: live docs teach the bare go build ./cmd/off-by-one path that the freshness guard rejects
**Evaluated:** 2026-09-18T18:08:08.402921
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.724s
- ✗ **tier2**
  - INCOMPLETE
  ✗ PASS: every live (non-historical) doc/skill reference that tells a reader how to build the binary uses make build, and a repo-wide grep for 'go build ./cmd/off-by-one' across docs/ skills/ README.md returns only dated dogfood logs (or nothing); the guard's messages are quoted where a doc explains how to verify a build.: First two clauses PASS: `grep -rn 'go build ./cmd/off-by-one' docs/ skills/ README.md` returns exactly one hit — docs/dogfood/2026-08-10-integration.md:83 ('Built with `go build ./cmd/off-by-one` (clean, ~30s)'), a dated dogfood log (historical, allowed). All live build instructions now use `make build`: docs/integration.md:26, docs/landing-spec.md:678, docs/dogfood/diagnostics.md:92, skills/off-by-one-usage/SKILL.md:35 and :152, README.md:289 (commit c684c4c). Remaining `go build` hits are `go build ./...` (README.md:313/341, AGENTS.md:35, CONTRIBUTING.md:6) — repo-wide build-all, not the binary path. THIRD CLAUSE FAILS: `grep -rn 'check-binary-fresh|no version stamp|dirty tree' docs/ skills/ README.md` returns only README.md:305, which merely names the target ('run `make build` and verify with `make check-binary-fresh`') and quotes no guard message. No doc anywhere quotes the guard's actual messages — Makefile:50 'ERROR: ./$(BINARY) was built from a dirty tree (stamp: $$stamp) ... run 'make build'' or Makefile:76 'ERROR: ./$(BINARY) carries no version stamp ($$stamp) — it was not built by 'make build'; run 'make build''. docs/dogfood/diagnostics.md:92 is the live doc that explains how to verify a build locally ('Test locally: `make build && ./off-by-one --skip-sandbox ...`') and it quotes no guard message, so the criterion's explicit requirement is unmet.
Live docs were correctly switched to `make build` and the only remaining bare `go build ./cmd/off-by-one` is a dated dogfood log, but no doc quotes the freshness guard's messages where it explains how to verify a build, so the criterion is not fully met.

## Summary

Judge Result: OB-GAP-076

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.724s

Stage tier2: FAIL
  INCOMPLETE
  ✗ PASS: every live (non-historical) doc/skill reference that tells a reader how to build the binary uses make build, and a repo-wide grep for 'go build ./cmd/off-by-one' across docs/ skills/ README.md returns only dated dogfood logs (or nothing); the guard's messages are quoted where a doc explains how to verify a build.: First two clauses PASS: `grep -rn 'go build ./cmd/off-by-one' docs/ skills/ README.md` returns exactly one hit — docs/dogfood/2026-08-10-integration.md:83 ('Built with `go build ./cmd/off-by-one` (clean, ~30s)'), a dated dogfood log (historical, allowed). All live build instructions now use `make build`: docs/integration.md:26, docs/landing-spec.md:678, docs/dogfood/diagnostics.md:92, skills/off-by-one-usage/SKILL.md:35 and :152, README.md:289 (commit c684c4c). Remaining `go build` hits are `go build ./...` (README.md:313/341, AGENTS.md:35, CONTRIBUTING.md:6) — repo-wide build-all, not the binary path. THIRD CLAUSE FAILS: `grep -rn 'check-binary-fresh|no version stamp|dirty tree' docs/ skills/ README.md` returns only README.md:305, which merely names the target ('run `make build` and verify with `make check-binary-fresh`') and quotes no guard message. No doc anywhere quotes the guard's actual messages — Makefile:50 'ERROR: ./$(BINARY) was built from a dirty tree (stamp: $$stamp) ... run 'make build'' or Makefile:76 'ERROR: ./$(BINARY) carries no version stamp ($$stamp) — it was not built by 'make build'; run 'make build''. docs/dogfood/diagnostics.md:92 is the live doc that explains how to verify a build locally ('Test locally: `make build && ./off-by-one --skip-sandbox ...`') and it quotes no guard message, so the criterion's explicit requirement is unmet.
Live docs were correctly switched to `make build` and the only remaining bare `go build ./cmd/off-by-one` is a dated dogfood log, but no doc quotes the freshness guard's messages where it explains how to verify a build, so the criterion is not fully met.

Overall: FAIL ✗
