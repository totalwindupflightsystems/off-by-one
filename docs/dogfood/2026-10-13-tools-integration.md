# Dogfood run 2026-10-13 — required_tools end-to-end + fresh-bunker re-proof of DF-36

**Run:** off-by-one-dogfood tick, 2026-10-13. Board: DF-OFF-BY-ONE-41,
INSTALL-OFF-BY-ONE-2026-10-13. Verdict: ✅ SHIPPABLE.

## Promise under test

"An agent can declare `required_tools` with its submission and the sandbox will
provision them" (README Request Fields: `required_tools` … "resolved on the
host and mounted read-only when available", example `["jq", "parallel"]`).
First exercise of this surface in 16 runs.

## What was done (real use)

1. Submitted a brand-new class `go-vet-copylocks-value-receiver-on-sync-mutex-20261013`
   (sub_ae14b0) to the live service (:8766) with
   `required_tools: ["jq", "parallel"]` — jq installed on the host, parallel NOT.
2. Watched the solve: queued 00:41:02, sandbox start 00:41:59, solved 00:43:37
   (1m38s vs ETA 4m35s), answer #4463, status verified.
3. Discovered the answer: genuinely correct (value receiver copies the embedded
   mutex; pointer-receiver fix diff; before/after vet+test evidence; and a
   toolchain note that Go 1.26 dropped copylocks from default test vet flags —
   independently plausible and checkable).
4. Re-proved closed findings on the deployed HEAD (f851ecbd build, service
   running since 10-12 14:24):
   - **DF-36** (complete, d18586d6): discover with `version: "1.26.1"` against
     `so-nil-pointer-deref` → `found: true` + `version_warnings: ["Requested
     version \"1.26.1\" has no exact answer; serving the best verified answer
     for this problem class (answer version \"\")."]` — fallback + warning both
     live. Also re-proved on the fresh bunker seed.
   - **DF-37** (complete): duplicate submit of the same tuple → HTTP 409 with
     the real `submission_id` (`sub_ae14b0`), not the old empty string.
   - **DF-38** (complete, a5926274): ETA 4m35s quoted at submit; actual 1m38s —
     the old ~2x-over-ETA pattern is gone (under-ETA this time; single sample).
5. Fresh install leg on bunker-mvp (see the INSTALL row): clone → make build
   84s → seed 23s → serve → discover fallback green on a fresh machine.
6. Measured (Step 2b): discover warm 10.6ms ± 2.0 (hyperfine n=20, live
   instance), cold 1.4ms after `drop_caches` (SQLite pages re-faulted cheaply);
   submit 41ms; nothing a user would feel. No PERF row.

## What failed

**DF-OFF-BY-ONE-41 (P2): the degrade is silent at the API.** The only record
that `parallel` was dropped is the server-log line:

```
WARN sandbox: could not resolve declared tools on host; solve will proceed without them missing_tools=[parallel]
```

(journalctl -u off-by-one, 00:41:59). The submit 200 response, the queue status
object (both pending and complete states) and the discover response carry no
tool field. A remote agent that declared required_tools cannot learn its tool
was unavailable — it has no access to the daemon log — and may assume its
toolchain was provisioned. The sandbox side honored the documented
degrade-gracefully contract exactly (internal/sandbox/tools.go:15); the gap is
that no API surface echoes `resolved`/`missing` back to the caller.

## Working recipe for the tools path

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/submit -H 'Content-Type: application/json' -d '{
  "problem_class": "<fresh-slug>",
  "environment": "linux", "language": "go", "version": "",
  "description": "...", "error_message": "...",
  "cadence": "pre-phase",
  "required_tools": ["jq", "parallel"]
}'
# then: grep the server log for missing_tools=[...] until DF-41 lands;
# poll /api/v1/queue/{submission_id}; discover without a version field.
```

## Environment notes (for the next runner)

- Live instance: systemd `off-by-one.service`, `--port 8766 -load-threshold -1
  -cron-interval 60s`, binary built 10-12 14:24 from f851ecbd-era HEAD.
- The solver executes as the service user; answer text may carry the service
  user's home path — expected: redaction to `~` happens on the export/publish
  path (sanitizeHostPaths, internal/export/git.go:617), not at solve time
  (re-proven by run 15; this run's fresh answer carries `/home/kara` pre-export
  by design).
- bunker-mvp quirks: `bunker exec` stdin pipe does not forward (stage scripts
  with `bunker cp <local> <agent>:<relative-target>`, then exec); scp with an
  explicit `$HOME/...` remote path fails, relative target works; run everything
  under `$HOME` (`/tmp` not trusted on bunker agents).
