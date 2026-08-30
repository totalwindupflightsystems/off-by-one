# Off-by-One — Dogfood Integration Report 2026-08-30 (sync-focused run)

**Run type:** DuckBrain focused-sync field test (cron target: `off-by-one-sync`,
namespace `off-by-one`). This run used the sync workflow itself as the product
under test, plus a live probe of the lab's core value loop.

**Verdict:** 🟡 PROMISING-BUT-ROUGH — the sync workflow works end-to-end and the
lab's core loop (submit → solve → discover) is healthy, but two real gaps
surfaced: a committed-but-undeployed fix (OB-GAP-062) and a 25-day-stalled
feeder proof-key series (OB-GAP-063).

---

## 1. What the sync project is

`off-by-one-sync` is a scheduler project whose workdir
(`~/.hermes/sync-workdirs/off-by-one-sync`) is **intentionally empty** — it is
not a code repo. Its job: run a focused DuckBrain namespace sync for the
`off-by-one` namespace every ~6h (cooldown 21600s), per the fleet default
prompt (namespace `duckbrain-sync`). The real subject is the **off-by-one
pre-solve lab** at `/home/kara/off-by-one` (Go monorepo, live server :8766).

## 2. How to run the sync (the right way, verified this run)

1. **Auth:** DuckBrain HTTP at `localhost:3000` requires `X-API-Key`. The
   scoped token for this namespace lives in `~/.duckbrain/auth.json` under
   `apiKeys[].name == "off-by-one-foreman"` (grant: namespace `off-by-one`).
   Do NOT use the schedulerd-sync key (no grant for this ns).
2. **Preflight:** test-write the rotating daily key
   `/sync/write-test-YYYY-MM-DD` (domain `config`) in the target namespace,
   then verify by recall. HTTP 201 + recall hit = healthy.
3. **Scan sources** (cheat sheet: `context-sync-duckbrain` skill →
   `references/off-by-one-sync.md`):
   - Scheduler: `GET 127.0.0.1:9090/api/v1/projects/off-by-one` (nested under
     `project`; `latest_tick` for spawn/completion).
   - Live server: `GET localhost:8766/health` + `/api/v1/stats`.
   - Git: `git log origin/master..HEAD` (branch IS master), `git status`.
   - CI: `gh run list -R totalwindupflightsystems/off-by-one`.
   - Feeder proof: `/off-by-one/feeder/submissions/YYYY-MM-DD` keys.
4. **Write facts** in batches ≤4, domains only from
   `person|event|concept|message|config|raw_note`. Write `/sync/last-run`
   LAST.
5. **Report** under 20 lines: keys written, verification, anomalies.

## 3. Live probe results (the lab's core loop)

| Probe | Result |
|---|---|
| `GET /health` | `{"status":"ok","uptime":"87h45m"}` |
| `GET /api/v1/stats` | 1377 problems, 1554 answers, all "verified", queue 0, hit_rate 1.0, coverage 1.13, solver available |
| `POST /api/v1/problems/discover` (go-linear-scan-register-allocator) | `found:true`, full answer (solution/evidence/signatures) — value prop holds |
| Feeder cron (jobs.json `3ac3112f61b5`) | ALIVE: 4×/day, last run 2026-08-30 05:06, status ok, 3 problems queued |
| CI | green (last 5 runs success, incl. pages deploy) |
| Git | 0 unpushed; WIP = events.jsonl (foreman's own uncommitted tick event) |

## 4. Errors / friction hit this run

1. **`/api/health` returns 404** on DuckBrain — the health endpoint is not at
   that path; use `/api/keys?namespace=...` or a test-write as the liveness
   probe. (Minor; the sync prompt says "DuckBrain HTTP reachable" without
   pinning a path.)
2. **`/api/keys` tree shape varies** — `tree` may be an object or a list;
   the walker must handle both. (Minor.)
3. **Feeder proof keys stalled 25 days** (Aug 5 → Aug 30) while the feeder
   demonstrably runs — the feeder prompt has no DuckBrain write step, so the
   "ingest-activity proof" the cheat sheet relies on is missing. Filed as
   OB-GAP-063.
4. **Deploy lag:** OB-GAP-060 fix committed Aug 30 00:24 but live server runs
   the Aug 24 binary (uptime 87h+). Stats still serve pre-fix numbers. Filed
   as OB-GAP-062.

## 5. What a new sync agent needs that isn't documented

- The **auth key location** (`~/.duckbrain/auth.json`, token name
  `off-by-one-foreman`) — the cheat sheet says "write-test fingerprint" but
  never says where the key lives or that the MCP server is disabled.
- The **feeder proof-key stall** is a known open finding
  (`/findings/off-by-one/feeder-duckbrain-submissions-key-stalled-2026-08-05`)
  — a sync that sees no feeder keys should check the finding before
  re-alarming.
- The **deploy-lag check** (binary mtime vs server uptime) is the single most
  valuable 10-second probe for this project — worth promoting into the cheat
  sheet's check list.

## 6. Files left behind

- `.coding-hermes/board/tasks.jsonl` — OB-GAP-062, OB-GAP-063 (JSONL format)
- `docs/dogfood/2026-08-30-integration.md` — this report
- `docs/dogfood/diagnostics.md` — §6 appended (sync-run diagnostics)
- `.coding-hermes/dogfood-log.md` — run log appended
- DuckBrain: `/sync/write-test-2026-08-30` (preflight proof) + this run's
  findings keys
