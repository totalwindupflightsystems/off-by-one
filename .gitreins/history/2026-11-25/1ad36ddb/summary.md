# Verdict: DOC-14

**Task:** P5 — README --version example 'off-by-one 0287082' matches no build mode (source build prints 0.3.0-dev; release ldflags inject the tag)
**Evaluated:** 2026-11-25T06:12:08.107415
**Result:** ✓ PASS

## Criteria

- ✓ **Falsifiable: README.md's `--version` row shows the real from-source form (`off-by-one 0.3.0-dev`, matching `go build ./cmd/off-by-one && ./off-by-one --version`) and names the release form (tag injected via -ldflags, e.g. `off-by-one v0.3.0`); the literal 0287082 no longer occurs anywhere in README.md.

GATE ARM (card-manager 2026-11-25T01:47Z): Run `grep -c 0287082 README.md` from the repo root; expect 0. Re-verified at origin/master 839a1f24 this pass: the count is still 1 (the literal sits in the --version example row), so the finding is open and this row is dispatchable as written.**
  - Gate arm: `grep -c 0287082 README.md` => `0` (grep rc=1, no matches). README.md:330 now reads: `| `--version` | Print the version string and exit: `off-by-one 0.3.0-dev` from source; release builds inject the tag via `-ldflags` (e.g. `off-by-one v0.3.0`) |`. Both forms empirically reproduced: `go build ./cmd/off-by-one && ./off-by-one --version` => `off-by-one 0.3.0-dev` (exit 0); `go build -ldflags "-X main.version=v0.3.0" ... && --version` => `off-by-one v0.3.0` (exit 0). Source default at cmd/off-by-one/main.go:53 `var version = "0.3.0-dev"`, printed at main.go:92 `fmt.Printf("off-by-one %s\n", version)`. Commit d5292994 changed exactly this one README line (1 insertion, 1 deletion). Tests: `go test ./... -short -count=1 -p 1 -timeout 300s` => REAL_EXIT=0, 15 packages `ok`, zero FAIL lines.
- ✓ **The documented from-source form is reproducible by the exact command the criterion names — `go build ./cmd/off-by-one && ./off-by-one --version` prints `off-by-one 0.3.0-dev` (not just that the string appears in README).**
  - Ran the criterion's exact command in the repo root: `go build ./cmd/off-by-one && ./off-by-one --version` => stdout `off-by-one 0.3.0-dev`, exit=0. Matches README.md:330 verbatim. Backed by cmd/off-by-one/main.go:53 (default `0.3.0-dev`) and main.go:92 (print format `off-by-one %s`).
- ✓ **The documented release form is real — building with `-ldflags "-X main.version=<tag>"` yields `off-by-one v0.3.0`, and the Makefile actually wires that ldflag (so the README's claim about release builds is not invented).**
  - `go build -ldflags "-X main.version=v0.3.0" -o /tmp/obo-doc14-rel ./cmd/off-by-one && /tmp/obo-doc14-rel --version` => `off-by-one v0.3.0`, exit=0. Makefile:8 `LDFLAGS := -X main.version=$(VERSION)` and Makefile:11 `go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one` confirm the release injection path the README names is the real mechanism.
- ✓ **No other README passage contradicts the corrected row (no remaining stale `--version` example stamp in README.md).**
  - `grep -nE "off-by-one [0-9a-f]{6,}" README.md` => no matches (rc=1). The only `off-by-one <version>` examples in README.md are both on line 330 (`off-by-one 0.3.0-dev`, `off-by-one v0.3.0`). The pre-existing `0.1.0-dev` at README.md:507 is not a --version example — it quotes the Makefile:78 error text `carries no version stamp ($$stamp)` verbatim, so it does not contradict the corrected row.

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Falsifiable: README.md's `--version` row shows the real from-source form (`off-by-one 0.3.0-dev`, matching `go build ./cmd/off-by-one && ./off-by-one --version`) and names the release form (tag injected via -ldflags, e.g. `off-by-one v0.3.0`); the literal 0287082 no longer occurs anywhere in README.md.

GATE ARM (card-manager 2026-11-25T01:47Z): Run `grep -c 0287082 README.md` from the repo root; expect 0. Re-verified at origin/master 839a1f24 this pass: the count is still 1 (the literal sits in the --version example row), so the finding is open and this row is dispatchable as written.: Gate arm: `grep -c 0287082 README.md` => `0` (grep rc=1, no matches). README.md:330 now reads: `| `--version` | Print the version string and exit: `off-by-one 0.3.0-dev` from source; release builds inject the tag via `-ldflags` (e.g. `off-by-one v0.3.0`) |`. Both forms empirically reproduced: `go build ./cmd/off-by-one && ./off-by-one --version` => `off-by-one 0.3.0-dev` (exit 0); `go build -ldflags "-X main.version=v0.3.0" ... && --version` => `off-by-one v0.3.0` (exit 0). Source default at cmd/off-by-one/main.go:53 `var version = "0.3.0-dev"`, printed at main.go:92 `fmt.Printf("off-by-one %s\n", version)`. Commit d5292994 changed exactly this one README line (1 insertion, 1 deletion). Tests: `go test ./... -short -count=1 -p 1 -timeout 300s` => REAL_EXIT=0, 15 packages `ok`, zero FAIL lines.
  ✓ The documented from-source form is reproducible by the exact command the criterion names — `go build ./cmd/off-by-one && ./off-by-one --version` prints `off-by-one 0.3.0-dev` (not just that the string appears in README).: Ran the criterion's exact command in the repo root: `go build ./cmd/off-by-one && ./off-by-one --version` => stdout `off-by-one 0.3.0-dev`, exit=0. Matches README.md:330 verbatim. Backed by cmd/off-by-one/main.go:53 (default `0.3.0-dev`) and main.go:92 (print format `off-by-one %s`).
  ✓ The documented release form is real — building with `-ldflags "-X main.version=<tag>"` yields `off-by-one v0.3.0`, and the Makefile actually wires that ldflag (so the README's claim about release builds is not invented).: `go build -ldflags "-X main.version=v0.3.0" -o /tmp/obo-doc14-rel ./cmd/off-by-one && /tmp/obo-doc14-rel --version` => `off-by-one v0.3.0`, exit=0. Makefile:8 `LDFLAGS := -X main.version=$(VERSION)` and Makefile:11 `go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one` confirm the release injection path the README names is the real mechanism.
  ✓ No other README passage contradicts the corrected row (no remaining stale `--version` example stamp in README.md).: `grep -nE "off-by-one [0-9a-f]{6,}" README.md` => no matches (rc=1). The only `off-by-one <version>` examples in README.md are both on line 330 (`off-by-one 0.3.0-dev`, `off-by-one v0.3.0`). The pre-existing `0.1.0-dev` at README.md:507 is not a --version example — it quotes the Makefile:78 error text `carries no version stamp ($$stamp)` verbatim, so it does not contradict the corrected row.
README.md:330 now documents both real build modes (`off-by-one 0.3.0-dev` from source, `off-by-one v0.3.0` via -ldflags), the literal 0287082 is gone (grep count 0), and both forms were reproduced by actually building and running the binary.
- ✓ **fix_verification**
  - fix_verification: 0 finding(s) verified

## Summary

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Falsifiable: README.md's `--version` row shows the real from-source form (`off-by-one 0.3.0-dev`, matching `go build ./cmd/off-by-one && ./off-by-one --version`) and names the release form (tag injected via -ldflags, e.g. `off-by-one v0.3.0`); the literal 0287082 no longer occurs anywhere in README.md.

GATE ARM (card-manager 2026-11-25T01:47Z): Run `grep -c 0287082 README.md` from the repo root; expect 0. Re-verified at origin/master 839a1f24 this pass: the count is still 1 (the literal sits in the --version example row), so the finding is open and this row is dispatchable as written.: Gate arm: `grep -c 0287082 README.md` => `0` (grep rc=1, no matches). README.md:330 now reads: `| `--version` | Print the version string and exit: `off-by-one 0.3.0-dev` from source; release builds inject the tag via `-ldflags` (e.g. `off-by-one v0.3.0`) |`. Both forms empirically reproduced: `go build ./cmd/off-by-one && ./off-by-one --version` => `off-by-one 0.3.0-dev` (exit 0); `go build -ldflags "-X main.version=v0.3.0" ... && --version` => `off-by-one v0.3.0` (exit 0). Source default at cmd/off-by-one/main.go:53 `var version = "0.3.0-dev"`, printed at main.go:92 `fmt.Printf("off-by-one %s\n", version)`. Commit d5292994 changed exactly this one README line (1 insertion, 1 deletion). Tests: `go test ./... -short -count=1 -p 1 -timeout 300s` => REAL_EXIT=0, 15 packages `ok`, zero FAIL lines.
  ✓ The documented from-source form is reproducible by the exact command the criterion names — `go build ./cmd/off-by-one && ./off-by-one --version` prints `off-by-one 0.3.0-dev` (not just that the string appears in README).: Ran the criterion's exact command in the repo root: `go build ./cmd/off-by-one && ./off-by-one --version` => stdout `off-by-one 0.3.0-dev`, exit=0. Matches README.md:330 verbatim. Backed by cmd/off-by-one/main.go:53 (default `0.3.0-dev`) and main.go:92 (print format `off-by-one %s`).
  ✓ The documented release form is real — building with `-ldflags "-X main.version=<tag>"` yields `off-by-one v0.3.0`, and the Makefile actually wires that ldflag (so the README's claim about release builds is not invented).: `go build -ldflags "-X main.version=v0.3.0" -o /tmp/obo-doc14-rel ./cmd/off-by-one && /tmp/obo-doc14-rel --version` => `off-by-one v0.3.0`, exit=0. Makefile:8 `LDFLAGS := -X main.version=$(VERSION)` and Makefile:11 `go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one` confirm the release injection path the README names is the real mechanism.
  ✓ No other README passage contradicts the corrected row (no remaining stale `--version` example stamp in README.md).: `grep -nE "off-by-one [0-9a-f]{6,}" README.md` => no matches (rc=1). The only `off-by-one <version>` examples in README.md are both on line 330 (`off-by-one 0.3.0-dev`, `off-by-one v0.3.0`). The pre-existing `0.1.0-dev` at README.md:507 is not a --version example — it quotes the Makefile:78 error text `carries no version stamp ($$stamp)` verbatim, so it does not contradict the corrected row.
README.md:330 now documents both real build modes (`off-by-one 0.3.0-dev` from source, `off-by-one v0.3.0` via -ldflags), the literal 0287082 is gone (grep count 0), and both forms were reproduced by actually building and running the binary.

Stage fix_verification: PASS
  fix_verification: 0 finding(s) verified

Overall: PASS ✓
