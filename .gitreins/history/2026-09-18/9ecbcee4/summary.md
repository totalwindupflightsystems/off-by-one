# Verdict: OB-GAP-070

**Task:** Freshness guard: dirty-tree build false PASS + README Quick Start path contradicts the guard
**Evaluated:** 2026-09-18T13:30:19.991066
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Makefile check-binary-fresh must exit non-zero with the literal message 'built from a dirty tree' when ./off-by-one reports a -dirty version stamp (grep -n 'built from a dirty tree' Makefile matches).: grep -n 'built from a dirty tree' Makefile matches at Makefile:50 (echo "ERROR: ./$(BINARY) was built from a dirty tree (stamp: $$stamp) ...") inside the `*-dirty)` case that runs `exit 1`. Runtime proof: replaced ./off-by-one with a stub printing 'off-by-one version 0.1.0-dev-dirty'; `make check-binary-fresh` output 'ERROR: ./off-by-one was built from a dirty tree (stamp: 0.1.0-dev-dirty) — the working tree had uncommitted changes when it was compiled; commit or stash them, then run 'make build'' and exited non-zero (make: *** [Makefile:43: check-binary-fresh] Error 1, EXIT=2).
  ✓ README.md Quick Start must build via 'make build' so the guard passes on a clean clone (grep -n 'go build ./cmd/off-by-one' README.md returns nothing).: `grep -n 'go build ./cmd/off-by-one' README.md` returns nothing (exit 1). README.md:289 (Quick Start section starting at :277) uses `make build`. The only `go build` hits are README.md:313 `go build ./...` and README.md:341 table entry, both in the separate 'Build, Test, Lint' section, not the Quick Start. Clean-clone runtime proof: after `git stash -u` (clean tree), `make build && make check-binary-fresh` printed './off-by-one is up to date with source (version stamp changed, but code paths are unchanged)' with EXIT=0.
  ✓ Makefile check-binary-fresh must name the missing version stamp instead of claiming source drift when the stamp is unparseable (e.g. 0.1.0-dev).: Makefile:69-77 adds a `resolved=no` branch: when the candidate is not a resolvable commit and not sha-shaped, it prints 'ERROR: ./$(BINARY) carries no version stamp ($$stamp) — it was not built by 'make build'; run 'make build'' and exits 1. Runtime proof: stub ./off-by-one printing 'off-by-one version 0.1.0-dev' -> `make check-binary-fresh` output 'ERROR: ./off-by-one carries no version stamp (0.1.0-dev) — it was not built by 'make build'; run 'make build'' and exited non-zero (make Error 1, EXIT=2). It names the missing stamp and does NOT emit the 'source changed since it was built' drift message.
All three criteria verified: the dirty-tree guard fails with the literal 'built from a dirty tree' message, the unparseable-stamp path names the missing stamp, and README Quick Start uses `make build` (no `go build ./cmd/off-by-one`), with clean-tree `make build && make check-binary-fresh` passing.

## Summary

Judge Result: OB-GAP-070

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Makefile check-binary-fresh must exit non-zero with the literal message 'built from a dirty tree' when ./off-by-one reports a -dirty version stamp (grep -n 'built from a dirty tree' Makefile matches).: grep -n 'built from a dirty tree' Makefile matches at Makefile:50 (echo "ERROR: ./$(BINARY) was built from a dirty tree (stamp: $$stamp) ...") inside the `*-dirty)` case that runs `exit 1`. Runtime proof: replaced ./off-by-one with a stub printing 'off-by-one version 0.1.0-dev-dirty'; `make check-binary-fresh` output 'ERROR: ./off-by-one was built from a dirty tree (stamp: 0.1.0-dev-dirty) — the working tree had uncommitted changes when it was compiled; commit or stash them, then run 'make build'' and exited non-zero (make: *** [Makefile:43: check-binary-fresh] Error 1, EXIT=2).
  ✓ README.md Quick Start must build via 'make build' so the guard passes on a clean clone (grep -n 'go build ./cmd/off-by-one' README.md returns nothing).: `grep -n 'go build ./cmd/off-by-one' README.md` returns nothing (exit 1). README.md:289 (Quick Start section starting at :277) uses `make build`. The only `go build` hits are README.md:313 `go build ./...` and README.md:341 table entry, both in the separate 'Build, Test, Lint' section, not the Quick Start. Clean-clone runtime proof: after `git stash -u` (clean tree), `make build && make check-binary-fresh` printed './off-by-one is up to date with source (version stamp changed, but code paths are unchanged)' with EXIT=0.
  ✓ Makefile check-binary-fresh must name the missing version stamp instead of claiming source drift when the stamp is unparseable (e.g. 0.1.0-dev).: Makefile:69-77 adds a `resolved=no` branch: when the candidate is not a resolvable commit and not sha-shaped, it prints 'ERROR: ./$(BINARY) carries no version stamp ($$stamp) — it was not built by 'make build'; run 'make build'' and exits 1. Runtime proof: stub ./off-by-one printing 'off-by-one version 0.1.0-dev' -> `make check-binary-fresh` output 'ERROR: ./off-by-one carries no version stamp (0.1.0-dev) — it was not built by 'make build'; run 'make build'' and exited non-zero (make Error 1, EXIT=2). It names the missing stamp and does NOT emit the 'source changed since it was built' drift message.
All three criteria verified: the dirty-tree guard fails with the literal 'built from a dirty tree' message, the unparseable-stamp path names the missing stamp, and README Quick Start uses `make build` (no `go build ./cmd/off-by-one`), with clean-tree `make build && make check-binary-fresh` passing.

Overall: PASS ✓
