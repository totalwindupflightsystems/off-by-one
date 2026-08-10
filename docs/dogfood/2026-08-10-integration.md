# Off-by-One — Real-Use Integration Report (2026-08-10)

Field-test run by coding-hermes-dogfood. This is what actually happened when a real
user (an AI agent) used the documented API end-to-end, plus a scratch-instance
operator test. Everything below was executed live against the running lab
(`http://localhost:8766`) and a scratch build in `/tmp/dogfood-off-by-one/`.

**Verdict: 🟡 PROMISING-BUT-ROUGH** — the core loop works and answers are high
quality; read-only catalog discovery is broken and docs drift is recurring.

---

## 1. The workflow that works (verified end-to-end)

### 1.1 Discover a cached answer (the payoff)

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/discover \
  -H 'Content-Type: application/json' \
  -d '{"problem_class":"go-cli-migrations-path-resolution"}'
```

→ `200 {"found":true,"answer":{...}}` — a deep answer: root cause, two code fixes,
12-test evidence block, model signature. Same for `godot-save-checksum-canonicalization`.

**Gotcha (finding OB-GAP-023):** the tuple fields are hard filters, not scoring.
`{"problem_class":"godot-save-checksum-canonicalization","environment":"linux"}`
→ `{"found":false}` even though a verified answer exists under `env=docker`.
Class-only (no env/lang/version) reliably returns the best answer. Query class-only
first, then refine.

**Gotcha (finding OB-GAP-022):** the README's example class `go-nil-pointer-deref`
does not exist in the corpus → `404 not_found`. Use a class from `data/INDEX.md`.

### 1.2 Submit a problem (the ingest)

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/submit \
  -H 'Content-Type: application/json' \
  -d '{
    "problem_class":"dogfood-field-test-2026-08-10",
    "environment":"linux","language":"go","version":"1.26",
    "description":"...","cadence":"post-debug",
    "context":{"source":"coding-hermes-dogfood"}
  }'
```

→ `200 {"submission_id":"sub_0202bf","status":"queued","position":3,...}`

### 1.3 Poll the queue (the loop)

```bash
curl -s http://localhost:8766/api/v1/queue/sub_0202bf
```

Observed live: `pending/queued` → `in_progress/solver_running` (started ~15 min
after submit; solves run up to 30m, some fail at the 300s bwrap cap — see
diagnostics.md). Repeat until `complete` (then discover) or `failed`.

### 1.4 Browse (read-only paths)

```bash
curl -s "http://localhost:8766/api/v1/problems?q=sqlite&limit=3"   # FTS search, 32 hits
curl -s http://localhost:8766/api/v1/problems/godot-save-checksum-canonicalization/answers
curl -s http://localhost:8766/api/v1/taxonomy
curl -s http://localhost:8766/api/v1/stats
# {"total_problems":874,"total_answers":1012,"verified_answers":1012,...}
```

## 2. Error paths — all behaved correctly

| Probe | Result |
|---|---|
| submit without `problem_class` | `400 {"error":"invalid_request","message":"problem_class is required"}` |
| submit with bad `cadence` | `400 {"error":"invalid_request","message":"ingest: invalid cadence"}` |
| `POST /api/v1/export` (unconfigured) | `501 {"error":"not_configured","message":"export directory not configured"}` |
| `GET /api/v1/queue/sub_bogus` | `404 {"error":"not_found","message":"submission not found"}` |
| `GET /api/v1/nope` | `404 page not found` |
| `GET /ws/chat` without upgrade headers | `426 WebSocket protocol violation...` |

## 3. Scratch-instance operator tests (`/tmp/dogfood-off-by-one/`)

Built with `go build ./cmd/off-by-one` (clean, ~30s). Ran three ways:

1. **Normal, no keys, `--skip-sandbox`** (port 8877, fresh DB):
   - Startup log: `WARN: cron loop not started — no solver available ... queued
     submissions will NOT be processed until the server is restarted with a solver`
     → OB-GAP-005 fix confirmed live.
   - `stats` → `solver_available:false`; submit still accepted (`queued`).
   - **Persistence:** submitted `sub_68c0a0`, killed the process, restarted →
     `GET /api/v1/queue/sub_68c0a0` still `pending`, `queue_depth:1`. Data survives
     restarts. ✓ trustworthy.
2. **Readonly** (port 8878, same DB, `--readonly`):
   - `stats` → `readonly:true`.
   - submit → `403 {"error":"read_only","message":"catalog is read-only — submissions go through the upstream lab"}` (clear).
   - `GET /api/v1/problems`, `/taxonomy` → 200.
   - **🚨 `POST /api/v1/problems/discover` → 403** — the ONLY discovery endpoint is
     blocked in the mode whose entire purpose is serving discovery
     (→ **OB-GAP-020, P1**).
   - `GET /ws/chat` → `403 AI agent disabled in read-only catalog mode` (as documented).
3. **`--help` / `--version`:** 12 flags, all matching README table (verified tick 267; re-spotted OK).

## 4. Corpus as a consumer (no server needed)

- `data/answers.jsonl` = 1002 lines; `data/answers/` = 864 per-class files;
  `data/INDEX.md` (regenerated 2026-08-10 08:48 UTC) = 864 classes / 1002 answers.
- Answer quality sampled: raft log replication (full implementation + 11-test
  evidence), helios migrations (root cause + 2 fixes + 12-test evidence) — genuinely
  usable, not boilerplate.
- **Drift:** README.md:319 still says 812/948 (→ **OB-GAP-021**); live is 874/1012.
- **Hygiene:** test classes (`0009-test-self-dogfood`, `0016-test`,
  `docs-canary-readme-status-refresh`, …) are published as verified answers
  (→ **OB-GAP-025**).

## 5. What a NEW user needs that isn't documented

1. **Discover tuple semantics** — env/lang/version are exact filters; class-only is
   the forgiving query (OB-GAP-023).
2. **Readonly ≠ usable for agents** — no way to discover from a catalog instance
   (OB-GAP-020).
3. **The `related_problems` field is `null`** for new classes (docs show an array) —
   minor, tolerable.
4. **`GET /api/v1/problems/{class}` returns `"status":""`** while the list says
   `verified` — don't trust the detail status (OB-GAP-024).

## 6. If I had 1 hour of the maintainer's time

Fix OB-GAP-020 first (allow discover in readonly — it is a pure read; the whole
catalog story depends on it). Then stop the README count drift permanently
(OB-GAP-021) — it has burned three fixes in six days.
