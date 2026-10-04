# Verdict: DF-OFF-BY-ONE-28

**Task:** Install docs: agent-unique port + fresh-uptime probe on shared-host netns installs
**Evaluated:** 2026-10-04T12:52:14.751639
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Docs-only change adds agent-unique-port warning and fresh-uptime probe guidance to install docs, referencing the 2026-10-02 evidence file; no .go files changed; go build ./... passes: Commit 7dd4e6d4 touches ONLY README.md (+2) and docs/integration.md (+2); `git show --name-only 7dd4e6d4 | grep -c '\.go$'` = 0 and `git diff --name-only 7dd4e6d4~1 7dd4e6d4 | grep '\.go$'` returns nothing (working tree also has no .go changes). README.md:416 adds the 'Port hygiene on shared-host / bunker installs' warning (port is host-global not agent-private; nohup serve survives `bunker destroy`; observed 54-minute-uptime stale listener; rule = pick an agent-unique port, not default 8766, and kill the serve before destroy; consumer probes must assert /health uptime is seconds/fresh, not just status:ok) and links the evidence file docs/dogfood/evidence/2026-10-02-install-bunker-las-03-raw.log. docs/integration.md:48 adds the 'Fresh-install probes' guidance (assert uptime is seconds; long uptime = stale server) cross-referencing the README note. The referenced evidence file exists (26117 bytes, dated 2026-10-02) and corroborates the claims: line 33 `{"status":"ok","uptime":"54m5s"}`, line 35 'ANOMALY: localhost:8766 on agent2 shows uptime 54m5s — orphaned serve from DESTROYED agent1 (shared host netns; port 8766 not agent-private)', line 72 root cause 'nohup ... & serve survives bunker destroy ... use an agent-unique port AND kill the serve before destroy'. `go build ./...` ran fresh with exit_code=0 and no output.
Docs-only commit 7dd4e6d4 adds the agent-unique-port warning and fresh-uptime probe guidance to README.md and docs/integration.md, references the existing 2026-10-02 evidence log that corroborates the 54m uptime anomaly, changes no .go files, and `go build ./...` passes (exit 0).

## Summary

Judge Result: DF-OFF-BY-ONE-28

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Docs-only change adds agent-unique-port warning and fresh-uptime probe guidance to install docs, referencing the 2026-10-02 evidence file; no .go files changed; go build ./... passes: Commit 7dd4e6d4 touches ONLY README.md (+2) and docs/integration.md (+2); `git show --name-only 7dd4e6d4 | grep -c '\.go$'` = 0 and `git diff --name-only 7dd4e6d4~1 7dd4e6d4 | grep '\.go$'` returns nothing (working tree also has no .go changes). README.md:416 adds the 'Port hygiene on shared-host / bunker installs' warning (port is host-global not agent-private; nohup serve survives `bunker destroy`; observed 54-minute-uptime stale listener; rule = pick an agent-unique port, not default 8766, and kill the serve before destroy; consumer probes must assert /health uptime is seconds/fresh, not just status:ok) and links the evidence file docs/dogfood/evidence/2026-10-02-install-bunker-las-03-raw.log. docs/integration.md:48 adds the 'Fresh-install probes' guidance (assert uptime is seconds; long uptime = stale server) cross-referencing the README note. The referenced evidence file exists (26117 bytes, dated 2026-10-02) and corroborates the claims: line 33 `{"status":"ok","uptime":"54m5s"}`, line 35 'ANOMALY: localhost:8766 on agent2 shows uptime 54m5s — orphaned serve from DESTROYED agent1 (shared host netns; port 8766 not agent-private)', line 72 root cause 'nohup ... & serve survives bunker destroy ... use an agent-unique port AND kill the serve before destroy'. `go build ./...` ran fresh with exit_code=0 and no output.
Docs-only commit 7dd4e6d4 adds the agent-unique-port warning and fresh-uptime probe guidance to README.md and docs/integration.md, references the existing 2026-10-02 evidence log that corroborates the 54m uptime anomaly, changes no .go files, and `go build ./...` passes (exit 0).

Overall: PASS ✓
