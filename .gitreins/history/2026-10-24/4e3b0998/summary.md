# Verdict: DF-OFF-BY-ONE-58

**Task:** seed: preserve corpus created_at timestamps
**Evaluated:** 2026-10-24T01:20:23.699339
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ go build succeeds; seed a corpus file with known created_at values; sqlite3 query shows problem_classes.created_at and answer_nodes.created_at match the corpus values (not the seed run's clock); re-running seed is idempotent (same timestamps); go test ./internal/seed/... ./internal/graph/... passes: All four sub-requirements verified with actual command output. (1) BUILD: `go build ./...` exit_code=0 (no output); `go build -o /tmp/ob1 ./cmd/off-by-one` exit_code=0. (2) TIMESTAMP PRESERVATION: created controlled corpus /tmp/ctest/data/answers/9001-ts-probe.json with class created_at="2021-03-04 05:06:07" and answers "2022-11-12 13:14:15"/"2023-01-02 03:04:05"; seed run wall clock was 2026-10-23 20:19:58. `sqlite3 /tmp/ctest/probe.db "SELECT id,title,created_at FROM problem_classes"` -> `1|ts-probe-class|2021-03-04 05:06:07`; `SELECT id,class_id,lang,created_at FROM answer_nodes` -> `1|1|go|2022-11-12 13:14:15` and `2|1|python|2023-01-02 03:04:05` — exact corpus match, not the seed clock. Cross-checked on the real corpus: fresh seed of 2863 files gave `classes=2863 created / 0 existing; answers=4492 created / 0 skipped`, `problem_classes WHERE title='unknown'` -> `2026-07-08 01:34:24` (matches data/answers/0001-unknown.json), and `COUNT(DISTINCT created_at)=2863` (not one seed-run stamp). (3) IDEMPOTENCY: re-ran seed after 2s sleep -> `classes=0 created / 1 existing; answers=0 created / 2 skipped`; `diff before.txt after.txt` produced empty output, timestamps byte-identical. (4) TESTS: `go test ./internal/seed/... ./internal/graph/... -count=1` exit_code=0 -> `ok .../internal/seed 2.761s` and `ok .../internal/graph 3.371s`; targeted run shows `--- PASS: TestSeedPreservesCorpusCreatedAt (0.37s)`, `--- PASS: TestSeedIdempotentRerun (0.30s)`, `--- PASS: TestCreatedAtExplicitVsDefault (0.00s)`. Implementation: internal/graph/store.go:410-412 inserts `created_at` via `COALESCE(NULLIF(?, ''), datetime('now'))`; internal/seed/seed.go:206 passes `cf.CreatedAt` to `UpsertProblemClassAt` and seed.go:261 passes `ans.CreatedAt` to `CreateAnswerNodeAt`. LSP diagnostics: 0 findings.
Build succeeds, seeded corpus timestamps (class and answers) are stored verbatim rather than re-stamped with the seed run's clock, re-seeding is byte-identical idempotent, and go test ./internal/seed/... ./internal/graph/... passes with exit_code=0.

## Summary

Judge Result: DF-OFF-BY-ONE-58

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ go build succeeds; seed a corpus file with known created_at values; sqlite3 query shows problem_classes.created_at and answer_nodes.created_at match the corpus values (not the seed run's clock); re-running seed is idempotent (same timestamps); go test ./internal/seed/... ./internal/graph/... passes: All four sub-requirements verified with actual command output. (1) BUILD: `go build ./...` exit_code=0 (no output); `go build -o /tmp/ob1 ./cmd/off-by-one` exit_code=0. (2) TIMESTAMP PRESERVATION: created controlled corpus /tmp/ctest/data/answers/9001-ts-probe.json with class created_at="2021-03-04 05:06:07" and answers "2022-11-12 13:14:15"/"2023-01-02 03:04:05"; seed run wall clock was 2026-10-23 20:19:58. `sqlite3 /tmp/ctest/probe.db "SELECT id,title,created_at FROM problem_classes"` -> `1|ts-probe-class|2021-03-04 05:06:07`; `SELECT id,class_id,lang,created_at FROM answer_nodes` -> `1|1|go|2022-11-12 13:14:15` and `2|1|python|2023-01-02 03:04:05` — exact corpus match, not the seed clock. Cross-checked on the real corpus: fresh seed of 2863 files gave `classes=2863 created / 0 existing; answers=4492 created / 0 skipped`, `problem_classes WHERE title='unknown'` -> `2026-07-08 01:34:24` (matches data/answers/0001-unknown.json), and `COUNT(DISTINCT created_at)=2863` (not one seed-run stamp). (3) IDEMPOTENCY: re-ran seed after 2s sleep -> `classes=0 created / 1 existing; answers=0 created / 2 skipped`; `diff before.txt after.txt` produced empty output, timestamps byte-identical. (4) TESTS: `go test ./internal/seed/... ./internal/graph/... -count=1` exit_code=0 -> `ok .../internal/seed 2.761s` and `ok .../internal/graph 3.371s`; targeted run shows `--- PASS: TestSeedPreservesCorpusCreatedAt (0.37s)`, `--- PASS: TestSeedIdempotentRerun (0.30s)`, `--- PASS: TestCreatedAtExplicitVsDefault (0.00s)`. Implementation: internal/graph/store.go:410-412 inserts `created_at` via `COALESCE(NULLIF(?, ''), datetime('now'))`; internal/seed/seed.go:206 passes `cf.CreatedAt` to `UpsertProblemClassAt` and seed.go:261 passes `ans.CreatedAt` to `CreateAnswerNodeAt`. LSP diagnostics: 0 findings.
Build succeeds, seeded corpus timestamps (class and answers) are stored verbatim rather than re-stamped with the seed run's clock, re-seeding is byte-identical idempotent, and go test ./internal/seed/... ./internal/graph/... passes with exit_code=0.

Overall: PASS ✓
