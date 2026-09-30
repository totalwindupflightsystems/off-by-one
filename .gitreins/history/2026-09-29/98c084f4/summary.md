# Verdict: REVIEW-OB-010

**Task:** Normalize corpus env/version to canonical tokens
**Evaluated:** 2026-09-29T18:28:31.155730
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	29.825s
- ✓ **tier2**
  - COMPLETE
  ✓ data/answers.jsonl env+version values contain no prose markers (space/semicolon/tilde); solver/export normalizes; guard extended: All three clauses verified with fresh command output. (1) Corpus clean: python scan of all 2460 lines of data/answers.jsonl for [ ;~] in environment/version -> 'FINAL jsonl offenders: 0'; data/answers/*.json per-class scan -> 0 offenders; site/ badge scan (class='badge v'>...) -> 0 offenders. Commit d771210f rewrote 600 lines of data/answers.jsonl (591 env + 220 version prose values normalized/dropped). (2) Normalization wired: internal/graph/normalize.go (new) defines NormalizeEnv/NormalizeVersion with tokenForbidden=[\s;~]; internal/graph/store.go:343 CreateAnswerNode (write funnel for solver/seed/import) applies them; internal/export/git.go:337-338 renderItem applies them before path/markdown render; scripts/export-answers.py:139/160 define normalize_env/normalize_version and main() lines 230-231 apply them to the exported env/version. (3) Guard extended: scripts/check-corpus-hygiene.sh now calls scripts/check-corpus-tokens.py (field-precise JSON env/version check) plus BADGE_PATTERN "class='badge v'>[^<]*[ ;~]"; selftest arms 6/7/8 cover prose env, prose version, prose badge; CI wires it at .github/workflows/ci.yml:187,190. Test evidence (fresh, cache-defeating): `bash scripts/check-corpus-hygiene.sh` -> 'corpus hygiene OK: no /home/<user> paths and no prose env/version tokens' EXIT=0; `bash scripts/tests/check-corpus-hygiene-selftest.sh` -> '18/18 checks passed — ALL GREEN' EXIT=0; `python3 scripts/tests/export_answers_normalize_test.py` -> 'Ran 4 tests ... OK' EXIT=0; `go test ./internal/graph/... -run Normalize -count=1` -> 'ok github.com/.../internal/graph 0.007s' with 43 subtests PASS and 0 FAIL; `go build ./...` EXIT=0; `go test ./... -short -count=1 -p 1 -timeout 180s` -> all 14 packages ok, TEST_EXIT=0.
Corpus env/version fields are prose-free (0 offenders across answers.jsonl, per-class JSON, and site badges), normalization is wired into the store write funnel, export renderer, and export script, and the hygiene guard plus its 18-check self-test and CI wiring all pass green.

## Summary

Judge Result: REVIEW-OB-010

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	29.825s

Stage tier2: PASS
  COMPLETE
  ✓ data/answers.jsonl env+version values contain no prose markers (space/semicolon/tilde); solver/export normalizes; guard extended: All three clauses verified with fresh command output. (1) Corpus clean: python scan of all 2460 lines of data/answers.jsonl for [ ;~] in environment/version -> 'FINAL jsonl offenders: 0'; data/answers/*.json per-class scan -> 0 offenders; site/ badge scan (class='badge v'>...) -> 0 offenders. Commit d771210f rewrote 600 lines of data/answers.jsonl (591 env + 220 version prose values normalized/dropped). (2) Normalization wired: internal/graph/normalize.go (new) defines NormalizeEnv/NormalizeVersion with tokenForbidden=[\s;~]; internal/graph/store.go:343 CreateAnswerNode (write funnel for solver/seed/import) applies them; internal/export/git.go:337-338 renderItem applies them before path/markdown render; scripts/export-answers.py:139/160 define normalize_env/normalize_version and main() lines 230-231 apply them to the exported env/version. (3) Guard extended: scripts/check-corpus-hygiene.sh now calls scripts/check-corpus-tokens.py (field-precise JSON env/version check) plus BADGE_PATTERN "class='badge v'>[^<]*[ ;~]"; selftest arms 6/7/8 cover prose env, prose version, prose badge; CI wires it at .github/workflows/ci.yml:187,190. Test evidence (fresh, cache-defeating): `bash scripts/check-corpus-hygiene.sh` -> 'corpus hygiene OK: no /home/<user> paths and no prose env/version tokens' EXIT=0; `bash scripts/tests/check-corpus-hygiene-selftest.sh` -> '18/18 checks passed — ALL GREEN' EXIT=0; `python3 scripts/tests/export_answers_normalize_test.py` -> 'Ran 4 tests ... OK' EXIT=0; `go test ./internal/graph/... -run Normalize -count=1` -> 'ok github.com/.../internal/graph 0.007s' with 43 subtests PASS and 0 FAIL; `go build ./...` EXIT=0; `go test ./... -short -count=1 -p 1 -timeout 180s` -> all 14 packages ok, TEST_EXIT=0.
Corpus env/version fields are prose-free (0 offenders across answers.jsonl, per-class JSON, and site badges), normalization is wired into the store write funnel, export renderer, and export script, and the hygiene guard plus its 18-check self-test and CI wiring all pass green.

Overall: PASS ✓
