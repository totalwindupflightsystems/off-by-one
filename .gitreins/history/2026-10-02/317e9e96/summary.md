# Verdict: SKIPPED-install-bunker-OB-2026-09-25

**Task:** Prove release-binary installability on bunker-las-03
**Evaluated:** 2026-10-02T12:03:52.861437
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out
- ✓ **tier2**
  - COMPLETE
  ✓ A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200: Evidence: docs/dogfood/evidence/2026-10-02-install-bunker-las-03-raw.log (the only non-harness file changed in HEAD 03c80c26). Fresh agent on bunker-las-03: L22 `bunker exec oby-install-test2 --server bunker-las-03`; clause-fix run creates agent oby-install-test3 (L73-75). Download from GitHub Releases: L47-53 `curl -sfLO .../releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64`. SHA256SUMS verify: L28 `off-by-one-v0.1.1-linux-amd64: OK`. --version: L29 `off-by-one v0.1.1` (agent3 run L90-91 repeats both). /health ok on :8766: agent3 clause-fix run proves the port free pre-serve (L94-95 host :8766 listener count = 0, ss_probe_exit=1) then L96-98 `{"status":"ok","uptime":"1s"}` HTTPSTATUS:200 exit=0 — uptime 1s plus the pre-serve port-free probe attributes the 200 to the fresh agent, closing the ec697ed0 orphan-serve defect. Discover 200: L99-104 corpus deploy (tar czf + bunker cp + tar xzf) and `seed -dir data` with seed_exit=0 (`seed complete: files=2433; classes=2433 created...`), then L105-106 discover response HTTPSTATUS:200 exit=0. Caveats (noted, not disqualifying): the proof is transcript-only (guards.tests=false, no Go code changed, so no automated test exists for this path); the agent3 discover body is truncated in the log so `found":true` is literally visible only in the agent2 run (L54, L56, on :8799) while agent3 shows the 200 status; and the discover 200 requires an out-of-band corpus seed the bare release asset does not ship, which the log itself documents.
All six clauses of the install criterion are raw-verified in the committed transcript, including the previously-failing :8766 attribution (port proven free pre-serve, uptime 1s) and the discover 200 after the documented seed flow.

## Summary

Judge Result: SKIPPED-install-bunker-OB-2026-09-25

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out

Stage tier2: PASS
  COMPLETE
  ✓ A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200: Evidence: docs/dogfood/evidence/2026-10-02-install-bunker-las-03-raw.log (the only non-harness file changed in HEAD 03c80c26). Fresh agent on bunker-las-03: L22 `bunker exec oby-install-test2 --server bunker-las-03`; clause-fix run creates agent oby-install-test3 (L73-75). Download from GitHub Releases: L47-53 `curl -sfLO .../releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64`. SHA256SUMS verify: L28 `off-by-one-v0.1.1-linux-amd64: OK`. --version: L29 `off-by-one v0.1.1` (agent3 run L90-91 repeats both). /health ok on :8766: agent3 clause-fix run proves the port free pre-serve (L94-95 host :8766 listener count = 0, ss_probe_exit=1) then L96-98 `{"status":"ok","uptime":"1s"}` HTTPSTATUS:200 exit=0 — uptime 1s plus the pre-serve port-free probe attributes the 200 to the fresh agent, closing the ec697ed0 orphan-serve defect. Discover 200: L99-104 corpus deploy (tar czf + bunker cp + tar xzf) and `seed -dir data` with seed_exit=0 (`seed complete: files=2433; classes=2433 created...`), then L105-106 discover response HTTPSTATUS:200 exit=0. Caveats (noted, not disqualifying): the proof is transcript-only (guards.tests=false, no Go code changed, so no automated test exists for this path); the agent3 discover body is truncated in the log so `found":true` is literally visible only in the agent2 run (L54, L56, on :8799) while agent3 shows the 200 status; and the discover 200 requires an out-of-band corpus seed the bare release asset does not ship, which the log itself documents.
All six clauses of the install criterion are raw-verified in the committed transcript, including the previously-failing :8766 attribution (port proven free pre-serve, uptime 1s) and the discover 200 after the documented seed flow.

Overall: FAIL ✗
