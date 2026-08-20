# Verdict: OB-GAP-052

**Task:** Stale binary: repo-root ./off-by-one predates HEAD; make check-binary-fresh fails; live server serves stale build
**Evaluated:** 2026-08-20T16:32:30.683082
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m11:31AM[0m [32mINF[0m [1mscanned ~4924920 bytes (4.92 MB) in 740ms[0m
[90m11:31AM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
- ✓ **tier2**
  - COMPLETE
  ✓ make check-binary-fresh exits 0; README Quick Start says rebuild after pull with make build + check-binary-fresh; binary serving /health built from HEAD; go build ./..., go test ./..., gitreins guard pass: (1) `make check-binary-fresh` exits 0: output './off-by-one is up to date with source', EXIT=0. (2) README.md line 273 (added in commit d9bd229) states 'Rebuild after pulling: ... after git pull, run make build and verify with make check-binary-fresh before running'. (3) Repo-root ./off-by-one binary reports version '9bff245-dirty' matching HEAD 9bff245; server log shows 'off-by-one 9bff245-dirty listening' and GET /health returned HTTP 200 {"status":"healthy"}. (4) go build ./... exit 0. (5) go test ./... -count=1: all 13 packages ok. (6) gitreins guard: go build exit 0, go vet ./... exit 0, go test all ok (guard runs build/lint/tests per config).
All sub-parts of the criterion verified: check-binary-fresh exits 0, README documents rebuild-after-pull, binary is built from HEAD (9bff245) and serves /health 200, and build/vet/test all pass.

## Summary

Judge Result: OB-GAP-052

Stage tier1: PASS
    ✓ secrets: [90m11:31AM[0m [32mINF[0m [1mscanned ~4924920 bytes (4.92 MB) in 740ms[0m
[90m11:31AM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin

Stage tier2: PASS
  COMPLETE
  ✓ make check-binary-fresh exits 0; README Quick Start says rebuild after pull with make build + check-binary-fresh; binary serving /health built from HEAD; go build ./..., go test ./..., gitreins guard pass: (1) `make check-binary-fresh` exits 0: output './off-by-one is up to date with source', EXIT=0. (2) README.md line 273 (added in commit d9bd229) states 'Rebuild after pulling: ... after git pull, run make build and verify with make check-binary-fresh before running'. (3) Repo-root ./off-by-one binary reports version '9bff245-dirty' matching HEAD 9bff245; server log shows 'off-by-one 9bff245-dirty listening' and GET /health returned HTTP 200 {"status":"healthy"}. (4) go build ./... exit 0. (5) go test ./... -count=1: all 13 packages ok. (6) gitreins guard: go build exit 0, go vet ./... exit 0, go test all ok (guard runs build/lint/tests per config).
All sub-parts of the criterion verified: check-binary-fresh exits 0, README documents rebuild-after-pull, binary is built from HEAD (9bff245) and serves /health 200, and build/vet/test all pass.

Overall: PASS ✓
