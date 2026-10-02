# Verdict: SKIPPED-install-bunker-OB-2026-09-25

**Task:** Prove release-binary installability on bunker-las-03
**Evaluated:** 2026-10-02T11:39:53.951999
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out
- ✗ **tier2**
  - INCOMPLETE
  ✗ A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200: Criterion 1: "A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200"

Evidence FOR (transcript artifact committed at docs/dogfood/evidence/2026-10-02-install-bunker-las-03.md, 94 lines):
- L32-36: `bunker spawn oby-install-test --server bunker-las-03 --ttl 2h` -> "Agent created: oby-install-test"
- L40-43: `bunker exec ... 'echo alive; uname -m; curl --version'` -> alive / x86_64
- L47-53: curl -sfLO .../releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64 + SHA256SUMS; `sha256sum -c --ignore-missing SHA256SUMS` -> "off-by-one-v0.1.1-linux-amd64: OK"; `--version` -> "off-by-one v0.1.1"
- L57-60: serve `--port 8766`; `curl -s http://localhost:8766/health` -> {"status":"ok","uptime":"1s"}
- L62-66: pre-seed discover -> {"error":"not_found"} (correct for empty node)
- L73-81: after seed, POST /api/v1/problems/discover {"problem_class":"so-nil-pointer-deref"} -> {"found":true,"answer":{...}}
- L84-86: `bunker destroy oby-install-test --server bunker-las-03` -> destroyed, key removed

Code cross-checks (all consistent with transcript):
- cmd/off-by-one/main.go:51-52,91: version var overridden via -ldflags; `--version` prints "off-by-one %s" -> "off-by-one v0.1.1" matches
- internal/api/server.go:212-217 handleHealth -> 200 {"status":"ok","uptime":...} matches
- internal/api/handlers.go:395-444 handleDiscover -> writeJSON(w, http.StatusOK, out) on found; 404 only on graph.ErrNotFound -> matches
- README.md:368-385 documents exactly this release path (V=v0.1.1, SHA256SUMS, :8766)
- Corroborating prior live runs on the same host: docs/dogfood/2026-09-25-integration.md:64-77 (agent ea8d84b3, sha256 OK, health 200, discover found:true), docs/dogfood/diagnostics.md:648-652, docs/dogfood/2026-09-23-integration.md:16,59

WEAKNESSES (why this is not a clean PASS):
1. The transcript is NOT raw captured output. It is a hand-written markdown document with prose ("Method: local `bunker` CLI -> spawn a fresh agent...", "This is a live remote-host proof - the verification is the transcript below, captured verbatim from the session commands"). No timestamps, no session id, no shell log, no exit codes, no HTTP status codes printed (the discover line shows only the JSON body, never "200"; the doc's own Verdict section asserts "discover 200" without showing it).
2. The seed step is elided: L73-74 shows `./off-by-one-v0.1.1-linux-amd64 seed ...` with a literal "..." in place of the arguments, and the corpus was shipped via an out-of-band `tar xzf /tmp/oby-data.tgz` (L73) - the /tmp/oby-data.tgz creation is never shown.
3. The criterion's discover-200 step only succeeds after a seed step that is NOT part of the criterion and that the release asset cannot satisfy (doc itself: "the release asset ships no corpus"). The pre-seed discover in the transcript returns not_found, i.e. the bare release binary does not satisfy the discover clause.
4. No test exists for this behaviour (repo has no test that exercises the release-install path; .gitreins/config.yaml guards.tests=false, go.tests=true). The task is a live-host proof, so no test is expected - but that also means nothing in the repo can be re-run to reproduce the claim.
5. Both prior gitreins verdicts on this task FAILED: e560e642 (tier1 tests "Command timed out") and e1079cf6 (tier2 INCOMPLETE - "evidence was transcript-only"). The current commit only adds the same transcript as a committed artifact; no new verification was performed.

VERDICT: FAIL - the criterion demands a live end-to-end proof on bunker-las-03; the only artifact is a self-authored, non-raw transcript with elided commands and no HTTP status codes, and the discover-200 clause depends on an unshown out-of-band corpus deployment. No reproducible test or raw log backs it.
Partial verdict — evaluation hit resource cap before all criteria verified

## Summary

Judge Result: SKIPPED-install-bunker-OB-2026-09-25

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out

Stage tier2: FAIL
  INCOMPLETE
  ✗ A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200: Criterion 1: "A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200"

Evidence FOR (transcript artifact committed at docs/dogfood/evidence/2026-10-02-install-bunker-las-03.md, 94 lines):
- L32-36: `bunker spawn oby-install-test --server bunker-las-03 --ttl 2h` -> "Agent created: oby-install-test"
- L40-43: `bunker exec ... 'echo alive; uname -m; curl --version'` -> alive / x86_64
- L47-53: curl -sfLO .../releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64 + SHA256SUMS; `sha256sum -c --ignore-missing SHA256SUMS` -> "off-by-one-v0.1.1-linux-amd64: OK"; `--version` -> "off-by-one v0.1.1"
- L57-60: serve `--port 8766`; `curl -s http://localhost:8766/health` -> {"status":"ok","uptime":"1s"}
- L62-66: pre-seed discover -> {"error":"not_found"} (correct for empty node)
- L73-81: after seed, POST /api/v1/problems/discover {"problem_class":"so-nil-pointer-deref"} -> {"found":true,"answer":{...}}
- L84-86: `bunker destroy oby-install-test --server bunker-las-03` -> destroyed, key removed

Code cross-checks (all consistent with transcript):
- cmd/off-by-one/main.go:51-52,91: version var overridden via -ldflags; `--version` prints "off-by-one %s" -> "off-by-one v0.1.1" matches
- internal/api/server.go:212-217 handleHealth -> 200 {"status":"ok","uptime":...} matches
- internal/api/handlers.go:395-444 handleDiscover -> writeJSON(w, http.StatusOK, out) on found; 404 only on graph.ErrNotFound -> matches
- README.md:368-385 documents exactly this release path (V=v0.1.1, SHA256SUMS, :8766)
- Corroborating prior live runs on the same host: docs/dogfood/2026-09-25-integration.md:64-77 (agent ea8d84b3, sha256 OK, health 200, discover found:true), docs/dogfood/diagnostics.md:648-652, docs/dogfood/2026-09-23-integration.md:16,59

WEAKNESSES (why this is not a clean PASS):
1. The transcript is NOT raw captured output. It is a hand-written markdown document with prose ("Method: local `bunker` CLI -> spawn a fresh agent...", "This is a live remote-host proof - the verification is the transcript below, captured verbatim from the session commands"). No timestamps, no session id, no shell log, no exit codes, no HTTP status codes printed (the discover line shows only the JSON body, never "200"; the doc's own Verdict section asserts "discover 200" without showing it).
2. The seed step is elided: L73-74 shows `./off-by-one-v0.1.1-linux-amd64 seed ...` with a literal "..." in place of the arguments, and the corpus was shipped via an out-of-band `tar xzf /tmp/oby-data.tgz` (L73) - the /tmp/oby-data.tgz creation is never shown.
3. The criterion's discover-200 step only succeeds after a seed step that is NOT part of the criterion and that the release asset cannot satisfy (doc itself: "the release asset ships no corpus"). The pre-seed discover in the transcript returns not_found, i.e. the bare release binary does not satisfy the discover clause.
4. No test exists for this behaviour (repo has no test that exercises the release-install path; .gitreins/config.yaml guards.tests=false, go.tests=true). The task is a live-host proof, so no test is expected - but that also means nothing in the repo can be re-run to reproduce the claim.
5. Both prior gitreins verdicts on this task FAILED: e560e642 (tier1 tests "Command timed out") and e1079cf6 (tier2 INCOMPLETE - "evidence was transcript-only"). The current commit only adds the same transcript as a committed artifact; no new verification was performed.

VERDICT: FAIL - the criterion demands a live end-to-end proof on bunker-las-03; the only artifact is a self-authored, non-raw transcript with elided commands and no HTTP status codes, and the discover-200 clause depends on an unshown out-of-band corpus deployment. No reproducible test or raw log backs it.
Partial verdict — evaluation hit resource cap before all criteria verified

Overall: FAIL ✗
