# Off-by-One — Observability Spec

Status: **audit + target spec, revised 2026-11-16**. The audit body (§1–§8) was written 2026-11-03 by a
monitoring lane against the tree of that day; §0 is a landing log recording what has shipped against it since.
No service code was modified to produce the original audit, and §0 changes no code either — it corrects the
document to what the tree now measures.

How to read this document after the revision:

- §0 is the **current** status of every item the audit proposed. Read it first.
- In §2 and §3, rows and headings that have been overtaken carry an explicit **SUPERSEDED** /
  **LANDED** / **OPEN** marker with a date and a commit. Unmarked text is still the 2026-11-03 measurement.
- Any remaining present-tense sentence that the landings falsified is inside a marked block, never free-standing.

Revision log:

- **2026-11-03** — original audit (§1–§8), state as measured that day.
- **2026-11-04** — `62b1c03b` lands `GET /metrics/prom` and Tier-1 families (OBS-OB-001). This document was not
  updated at the time; the superseded wording below survived until this revision.
- **2026-11-06** — `e066e203` lands solve-resource + failure-cause families (OBS-OB-003).
- **2026-11-16** — `6c4e09aa` lands scheduler-health families (OBS-OB-002). **This revision** adds §0 and
  reconciles §2, §3, §4, §5, §6 and §8 against those three commits.
- **2026-11-16 (second pass)** — `fe3be63e` lands the six Tier-3 DB-derived families (OBS-OB-004; merged at
  `78b47e1c`). §3.4, §4.4 and §6 Step 4 are rewritten from target to measured post-landing state, the emitted family
  count is recounted from the registry, and every file:line citation kept was re-derived in the tree. The date on the
  OBS-OB-004 marker is corrected to the commit's own timestamp (2026-11-16 19:38 -0500); the first pass had written
  2026-11-17.

Naming convention for every metric below: `<product>_<subsystem>_<thing>_<unit>`.
Product prefix is `obo` (short form of off-by-one, matching the existing `oby-memcap` binary).

---

## 0. Landing log — what has shipped against this spec

Reconciled 2026-11-16 in two passes: the first from the tree of `wt/DOC-8` (branch point `e8f72f8f`), the second from
`97c47966` (the `wt/DOC-8-R2` branch point, level with `origin/master`) after `fe3be63e` merged. Every "landed" row was
checked by reading the code in that worktree — handler, registry, collector and the test that asserts the wire format —
not by trusting the board, and the §3.4 row was additionally checked on a scratch process against a copy of the live
database. Commit hashes are from `git log --oneline` in the same worktree and each is an ancestor of `HEAD`.

| Spec | Item | Status | Evidence verified in tree |
|---|---|---|---|
| §3.1 | Tier-1 families: `obo_http_up`, `obo_cron_loop_running`, `obo_solves_total`, `obo_queue_pending`, `obo_solve_duration_seconds` | **LANDED 2026-11-04 — `62b1c03b`** *"feat(metrics): Prometheus exposition at GET /metrics/prom (OBS-OB-001)"* | Route `internal/api/server.go:185` (`mux.HandleFunc("GET /metrics/prom", s.handleMetricsProm)`); handler `server.go:397-399` (explicitly never 404s — a nil `Prom` serves a zero-source registry so the route always exists); registry `internal/metrics/prom.go:128-130` (`NewProm` + `prometheus.NewRegistry()`), Tier-1 registrations `:132-218`; tests `internal/api/metrics_prom_test.go:33` (status + `Content-Type` + families) and `:96` (nil-receiver) |
| §3.2 | Scheduler health / starvation: `obo_cron_ticks_total`, `obo_cron_skip_reason_total`, `obo_host_load1`, `obo_host_mem_used_fraction`, `obo_cron_last_tick_timestamp_seconds` | **LANDED 2026-11-16 — `6c4e09aa`** *"obs(OBS-OB-002): emit gate/scheduler health metrics on /metrics/prom"* | Descriptors `internal/metrics/prom.go:275-299`, collector `:520-575`; single source `internal/metrics/metrics.go:541` `SchedulerHealth` (struct `:147-160`, host reading `HostSnapshot` `:168-170`); closed label sets `metrics.go:99-111` (`TickOutcomes`, `SkipReasons`); wired `cmd/off-by-one/main.go:318`; tests `internal/metrics/prom_scheduler_test.go:72` (every family pre-created at zero) and `:224` (closed vocabulary), plus `internal/cron/cause_test.go:19` (`scrapeProm` helper drives the real handler and fails on any non-200) |
| §3.3 | Solve resources / failure causes: `obo_solve_peak_rss_bytes`, `obo_solve_killed_total`, `obo_solve_over_budget_total`, `obo_solve_rejected_total`, `obo_solve_failure_total` | **LANDED 2026-11-06 — `e066e203`** *"feat(metrics): solve resource + failure-cause families on /metrics/prom (OBS-OB-003)"* | Registrations `internal/metrics/prom.go:223-267`; collectors `:441-466` (peak-RSS histogram), `:478-501` (killed / over-budget / rejected), `:581-600` (failure causes); source `internal/metrics/metrics.go:367` `SolveResources` (struct `:235-251`); cause vocabulary `internal/solver/cause.go:62-71`; tests `internal/api/metrics_prom_solve_test.go:16`, `internal/cron/cause_test.go:67-339` |
| §3.4 | Queue age, deferrals, catalogue, discovery, DB size (6 families) | **LANDED — `fe3be63e`** *"obs(OBS-OB-004): ship Tier-3 DB-derived gauges on /metrics/prom"* (commit stamp 2026-11-16 19:38 -0500; merged into `master` at `78b47e1c`, 21:14) | Registration `internal/metrics/prom.go:160-205` (`dbGaugeCollector`, `deferredTotalCollector`, `discoveryTotalCollector`); sampler `internal/metrics/prom_db.go` (`dbGaugeCache` `:85-104`, `RefreshDB` `:111-122`, `StartDBRefresh` `:129-149`, `Collect` `:174-231`); **15 s** ticker `prom_db.go:29`, started at `cmd/off-by-one/main.go:332`; **no SQL and no `stat()` inside the scrape handler** — `Collect` copies the cached struct only. Verified live 2026-11-16 on a scratch process (port 18966, DB copy, no solver): all six families present, `obo_queue_oldest_pending_age_seconds` 59322.92 vs 59323.2 from SQL, `obo_catalog_verified_answers` 5033 = SQL 5033, `obo_db_size_bytes{kind="main"}` 59064320 = `stat` 59064320, scrape 0.48-0.57 ms | Board row **OBS-OB-004** |
| §4.1 | In-process `client_golang` + `promhttp`, private registry, `/metrics/prom` as a new path, JSON `/metrics` preserved | **LANDED as recommended — `62b1c03b`** | `github.com/prometheus/client_golang v1.25.0` is in `go.mod:7` (the dependency was genuinely absent on 2026-11-03); private registry `internal/metrics/prom.go:129`, served by `promhttp.HandlerFor` at `:302`; JSON `/metrics` untouched at `internal/api/server.go:380-387` |
| §4.2 | Wire the previously-dead `internal/cron.Metrics` counters into the registry | **LANDED** — `62b1c03b` (loop counters), `e066e203` (resources), `6c4e09aa` (scheduler), `fe3be63e` (DB-derived) | `cmd/off-by-one/main.go:249-319` builds `PromConfig` from `loop.Metrics().Snapshot()` (`:298-309`) plus `observer.SolveResources` (`:312`) and `observer.SchedulerHealth` (`:318`); `main.go:322` feeds the duration histogram through `SetDurationObserver`; `main.go:332` starts the DB-gauge refresher |
| §5 | Fleet scrape job, `up{}`, product/service/host registry labels | **OPEN — out of repo scope** | No `job_name`, `scrape_configs` or `remote_write` anywhere in this repo (checked `*.yaml`, `*.yml`, `*.toml`, `*.md`); `deploy/` holds only `off-by-one.service`. Board row **OBS-OB-005** |
| §6 | Steps 0-4 verification | **Steps 0-4 have their series and were run against a scratch process; the deployed unit and the scrape job are still OPEN** | Step 0's three assertions are enforced by `TestMetricsPromEndpoint` (`internal/api/metrics_prom_test.go:33`) and were also measured live (§3.4 row: HTTP 200, `Content-Type: text/plain; version=0.0.4…`). Steps 1-4's series are asserted family-by-family in `internal/metrics/prom_test.go`, `prom_scheduler_test.go`, `prom_db_test.go` and `internal/api/metrics_prom_solve_test.go`, and all 21 families appeared in one live exposition. Still open: the *running deployment* (no restart since 2026-11-04) and the fleet scrape job (§5, OBS-OB-005) — see §6 |

**Emitted family count: 21.** 5 Tier-1 (§3.1) + 6 Tier-3 DB-derived (§3.4) + 5 solve-resource/cause (§3.3) + 5
scheduler (§3.2). `promhttp` drives one private registry, so the families on the wire are exactly the ones registered in
`internal/metrics/prom.go`; there is no second, hidden exposition path. Recount in the tree:
`grep -c '"obo_[a-z0-9_]*"' internal/metrics/prom.go` → **21**. The narrower pattern `'"obo_[a-z_]*"'` returns **20**
because it cannot match the digit in `obo_host_load1` (`prom.go:286`) — that one-character gap is the whole difference
between the two numbers, not a missing family. Confirmed on a live exposition 2026-11-16: 21 `# TYPE obo_*` lines with
the cron loop ticking.

One label correction against §3.1 as written: the `outcome` vocabulary on the wire is
`{succeeded, failed, rejected}` (`internal/metrics/prom.go:40`), not `{success, …}`. Queries and alerts must use the
emitted spelling. The `cause` vocabulary is the seven closed values in `internal/solver/cause.go:62-71` and matches §3.3
as written. Every family is emitted unconditionally except the three tick-dependent gauges (`obo_host_load1`,
`obo_host_mem_used_fraction`, `obo_cron_last_tick_timestamp_seconds`), which are omitted until the loop has a reading —
measured on a no-solver scratch probe: 18 of 21 present, the 3 absent exactly those.

---

## 1. What this service is, and what "healthy" means

Off-by-One is a **pre-solve lab**, not a GPU worker farm. It maintains a graph of
problem classes → pre-verified answers, accepts submissions onto a queue, and a single
background cron loop solves them by driving **pi-agent inside a bwrap sandbox**, then commits
the answer into the SQLite graph.

Evidence: `cmd/off-by-one/main.go:1-10` (process roles: graph store, ingest queue, API server,
cron loop); `internal/solver/piagent.go:50` (pi-agent + provider key); `internal/sandbox/bwrap.go`
(sandbox); `internal/cron/loop.go:459` (`Loop.Run` — loop wiring).

There is **no GPU component**: `grep -rin "gpu|nvidia|cuda|vram"` over `*.go` returns zero
implementation hits (only unrelated corpus strings in `data/INDEX.md` and a CSS comment in
`docs/landing-spec.md:247`). The "idle GPU time" framing in the task brief does not match this
repository. What it actually farms is **host idle time**, gated on load average and memory
(`internal/cron/loop.go` `checkIdle`, memory + load halves; `internal/cron/memgate.go`).

**Healthy means all five hold:**

1. The HTTP API answers `/health` 200 and `/metrics` 200 (not 404 `metrics_disabled`). **Since `62b1c03b`
   it must also answer `/metrics/prom` 200 with `Content-Type: text/plain; version=0.0.4`** — that route is
   registered unconditionally and serves a zero-source registry rather than 404, so a 404 here is a deployment
   fault, not a missing feature (`internal/api/server.go:185`, `:397-399`).
2. The cron loop is *running* — not in the `WARN: cron loop not started — no solver available`
   state (`cmd/off-by-one/main.go:225`).
3. Queue depth is **draining**, not accumulating: newest `complete` is recent, and
   `pending + deferred + in_progress` is not growing across hours.
4. Solves succeed at a stable rate and the failure rate is explained, not silent.
5. No solve approaches the memory cap: peak RSS stays well under the reject threshold.

### Current state as measured (2026-11-03, live process on `127.0.0.1:8766`, pid 606699)

```
GET /metrics  → {"solve_count":0,"skipped_host_pressure":89,"discovery_hits":1,
                 "discovery_misses":2,"over_budget":0,"rejected":0,"peak_rss_bytes_max":0,
                 "alert_threshold_bytes":4294967296,
                 "last_host":{"load1":14.77,"mem_used_fraction":0.2749,...},
                 "last_skip_reason":"load"}
GET /health   → {"status":"ok","uptime":"1h29m19s","version":"v0.2.1-26-gad43dc9c"}
GET /api/v1/stats → {"total_problems":3150,"total_answers":4885,"verified_answers":4857,
                     "queue_depth":20,"hit_rate":0.9943,"coverage":1.542,
                     "avg_solve_time":"4m42s","readonly":false,"solver_available":true}
```

Read that `/metrics` body carefully — it is the core finding of this audit:

- `skipped_host_pressure: 89` and `last_skip_reason: "load"` at `load1: 14.77`. The unit runs
  `-load-threshold 4` (`deploy/off-by-one.service:62`), so the loop has been skipping **every**
  tick for the last 1h29m of process uptime. The loop is alive but **starved**.
- `solve_count: 0` while the queue holds 20 unfinished entries. The lab is doing no work and
  the only signal saying so is a JSON field nobody scrapes.

This is precisely the 3am question — *"is it doing work?"* — and today the answer is "no" with
no alert anywhere.

> **Historical snapshot (2026-11-03).** The block above is a measurement, not present state, and it is left exactly as
> taken so the audit remains falsifiable. Two things have changed since: the *exposure* half landed (§0 — the starvation
> signal is on `/metrics/prom`), and the *starved loop* observation was never re-measured after the 2026-11-04 restart,
> so "no" is not a current claim. What has **not** changed is that nothing scrapes the endpoint, so the 3am question
> still has no alert behind it — see §5 and §8. `deploy/off-by-one.service:62` in the bullet below now points at the
> `ExecStart` line at `:69` in the current unit file.

---

## 2. Blind spots, with the code location where the state lives but is never emitted

**Table snapshot: 2026-11-03.** Rows carrying a **SUPERSEDED** / **PARTLY SUPERSEDED** marker have since changed
(landing log in §0); every unmarked row still reads as measured on that date. The column heading "Why it is
currently invisible" is the 2026-11-03 heading, kept verbatim so the audit stays readable as an audit.

| Operator question | Where the truth already lives | Why it *was* invisible on 2026-11-03 (see marker per row) |
|---|---|---|
| Is the process up? | `internal/api/server.go:370-376` (`handleHealth`) | Only answers if a human curls it; no `up{}` series, no scrape job. (`obo_http_up`, `62b1c03b`, is a constant 1 — it proves the exposition was served, not that a scraper is watching.) |
| **Is the loop doing work, or starving?** | `internal/metrics/metrics.go:642-644` `SkippedHostPressure` (it moved twice; was cited at `:224`, then `:563`), `internal/cron/loop.go:202` `recordIdleSkip`, called at `loop.go:644` | ~~Exposed only as JSON at `/metrics`; no Prometheus text format, no scrape job ⇒ unalertable.~~ **SUPERSEDED 2026-11-16 — OBS-OB-002, `6c4e09aa`.** The starvation signal is now emitted in Prometheus text format at `GET /metrics/prom`: `obo_cron_ticks_total{outcome}`, `obo_cron_skip_reason_total{reason}`, `obo_host_load1`, `obo_host_mem_used_fraction`, `obo_cron_last_tick_timestamp_seconds` (`internal/metrics/prom.go:520-575`, source `internal/metrics/metrics.go:541` `SchedulerHealth`). JSON `/metrics` is unchanged. What is still missing is the **scrape job**, not the exposition — §5. |
| How much work, and of what kind? | `internal/cron/loop.go:99-111` `Metrics` counters (`SolveAttempts`/`Success`/`Failed`), `MetricsSnapshot` at `loop.go:127-158` | ~~**Dead code path** … The per-solve win/loss counters are computed and thrown away.~~ **SUPERSEDED — `62b1c03b`, extended by `e066e203` and `6c4e09aa`.** Those counters are wired: `cmd/off-by-one/main.go:298-309` builds the exposition's `SolveCounts` from `loop.Metrics().Snapshot()`, so win/loss/rejected are scraped as `obo_solves_total{outcome}` and solve duration as `obo_solve_duration_seconds` (`main.go:322`). `handleMetrics` still serves the *Observer* JSON snapshot at `/metrics` — that was left alone deliberately, not as an oversight. |
| How long does a solve take? | `internal/ingest/queue.go:702` `AvgSolveTime` (mean over last 20) | ~~Only a rounded string in `/api/v1/stats`; no distribution, no per-model split, not scrappable.~~ **PARTLY SUPERSEDED 2026-11-04 — OBS-OB-001, `62b1c03b`.** `obo_solve_duration_seconds` is now a histogram labelled by `outcome` (`internal/metrics/prom.go:207-218`, bucket bounds `:36`, outcome vocabulary `:40`), fed per completed solve from `cmd/off-by-one/main.go:322`, so a p95 is computable from a scrape. **Still true:** there is no per-model split — §3.5 excludes `model` as a label deliberately, so per-model timing stays an offline SQL question; and `/api/v1/stats` `avg_solve_time` remains a rounded 20-sample mean, because the distribution lives only on `/metrics/prom`. |
| How often does it fail, and why? | `queue_entries.failure_reason` (`sql/schema/queue.sql:25`), 2196 failed rows on disk | ~~Free-text in SQLite … No counter, no labels, no alert.~~ **SUPERSEDED 2026-11-06 — OBS-OB-003, `e066e203`.** Failures are classified at the failure site, not by parsing text, into `obo_solve_failure_total{cause}` over a closed seven-value set (`internal/solver/cause.go:62-71`); the historical clusters map to `cause="timeout"` (112), `cause="solver_exit"` (40), `cause="toolchain_missing"` (29), `cause="invalid_problem"` (18). The SQLite free-text column is still the audit trail — it is no longer the only place the signal exists. |
| Is a queue/lag building? | `queue_entries.status` (`sql/schema/queue.sql:19`); live read at `queue.go:646-652` `Depth`; age now computed at `queue.go:667` `OldestPendingAge` | ~~Depth is a single number in `/api/v1/stats`; **age** of the oldest unfinished entry is never computed anywhere, so a 6-hour stuck entry looks like a 6-second one.~~ **SUPERSEDED 2026-11-16 — OBS-OB-004, `fe3be63e`.** `obo_queue_pending` and `obo_queue_oldest_pending_age_seconds` are both served on `/metrics/prom` from a 15 s cache (§3.4) — measured live at 59322.9 s against a 59323.2 s SQL answer. The 2026-11-03 observation (entries pending since 11:54Z, newest complete 18:24Z) is kept as the reason the family exists. |
| Are submissions deferring (gated) and why? | `queue_entries.gate_reason` (`queue.sql:26`), `deferred` status, counted in `Observer.Deferrals` (`internal/metrics/metrics.go:452`) | ~~Only visible per-row via `GET /api/v1/queue/{id}`. Live sample: 1 deferred with `gate_reason=load`. Not countable as a series.~~ **SUPERSEDED 2026-11-16 — OBS-OB-004, `fe3be63e`.** `obo_queue_deferred_total{reason}` is served on `/metrics/prom` over the closed `load` / `memory` / `load+memory` vocabulary, every reason present at 0 before it first fires (`internal/metrics/prom.go:385-404`, wired `cmd/off-by-one/main.go:297`). |
| Is memory about to be blown? | `internal/metrics/metrics.go:41` histogram buckets + `:181` `alertBytes`, `:286` `RecordSolve`, emitted via `:367` `SolveResources` | ~~Counters exist (`over_budget`, `peak_rss_bytes_max`) but are **only in JSON** …~~ **SUPERSEDED 2026-11-06 — OBS-OB-003, `e066e203`.** Now scraped: `obo_solve_peak_rss_bytes` (histogram, buckets `256MiB, 512MiB, 1GiB, 2.5GiB, 4GiB` — `metrics.go:41`), `obo_solve_over_budget_total`, `obo_solve_killed_total` (`internal/metrics/prom.go:223-267`). The *data* caveat survives the surface fix: on 2026-11-03 `solve_count: 0` meant the memory half of the gate had never recorded a solve, so these series start at zero and only become meaningful once solves run under the current binary. |
| Is its data growing / is the disk safe? | `off-by-one.db` is 56 MB with a 1.15 MB WAL; `answer_nodes` 4885 rows; sizes now read at `cmd/off-by-one/main.go:274-287` | ~~No size, no row-count, no WAL-bytes, no vacuum-age series.~~ **SUPERSEDED 2026-11-16 — OBS-OB-004, `fe3be63e`.** `obo_db_size_bytes{kind}` ∈ {`main`,`wal`} is served on `/metrics/prom` from a `stat()` taken by the 15 s refresher, never per scrape; measured live at 59064320 / 4305432 bytes, both exact against `stat`. **Still true:** there is no row-count and no vacuum-age series — growth in *rows* remains an offline SQL question. |
| Is the provider credential/quota about to fail? | `cmd/off-by-one/main.go:154-161` provider key resolution; startup-only `WARNING: DEEPSEEK_API_KEY is empty or placeholder` | **PARTLY SUPERSEDED 2026-11-06 — `e066e203`, OBS-OB-003.** The "counted nowhere" half is closed: a revoked or exhausted key now shows up as `obo_solve_failure_total{cause="solver_exit"}` (or `cause="timeout"`), and the 112-row `pi timed out after 25` cluster — the largest real cause — has a dedicated series instead of only free text. **Still true:** the credential is checked *once at boot* and never re-checked, so a mid-run revocation is only visible indirectly as a failure-rate climb. ~~And `obo_discovery_requests_total` (§3.4) is not emitted, so the 1-hit/2-miss discovery counter remains JSON-only.~~ **SUPERSEDED 2026-11-16 — OBS-OB-004, `fe3be63e`:** `obo_discovery_requests_total{outcome}` ∈ {`hit`,`miss`} is emitted (`internal/metrics/prom.go:410-433`, source `internal/metrics/metrics.go:603`), deliberately distinct from `obo_catalog_hit_rate`, which measures catalogue coverage rather than discovery outcomes. |

Correction to the static scan in the brief: the hypothesis `endpoint_metrics=3 … own_counter_or_gauge=1`
**undercounted**. There is a dedicated `internal/metrics` package, a registered
`GET /metrics` route, and substantial counter state in `internal/cron.Metrics`. The precise
defect *was* not "no metrics" — it was **"metrics exist in a bespoke JSON shape that no collector
speaks, over a route no collector is configured to read, and one counter set (`cron.Metrics`)
is not wired to any endpoint at all."**

**Updated 2026-11-16 (`62b1c03b`, `e066e203`, `6c4e09aa`, `fe3be63e`):** of those three defects, the first two are
fixed — a collector does speak the format now (`text/plain; version=0.0.4` on `/metrics/prom`) and `cron.Metrics` is
wired (`cmd/off-by-one/main.go:298-309`). The third stands: **no collector is configured to read the route**, because the
scrape job lives in the fleet monitoring stack outside this repository (§5). The audit's conclusion is therefore
half-resolved, and the remaining half cannot be closed by editing this repo.

---

## 3. Metric catalogue

Cardinality rule: labels below are the *complete* allowed set. **Never** label by
`submission_id`, `problem_class`, `model`, file path, prompt text, or timestamp — the first two
are per-request identifiers (`internal/metrics/metrics.go:63` `SubmissionID`, `:64` `ProblemClass`
show how tempting and how wrong that is). `outcome` and `reason` are closed enumerations.

### 3.1 Tier 1 — minimal viable emission (the 5 that answer "is it working?")

**LANDED 2026-11-04 — `62b1c03b` (OBS-OB-001).** All five families below are served at `GET /metrics/prom`
(`internal/metrics/prom.go:132-218`, registry `:128-130`); the table is unchanged from the audit because the
implementation matched it, with two marked exceptions: the `outcome` label spelling on `obo_solves_total`, and
`obo_queue_pending`, which carries no `status` label on the wire (§4.4).

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_http_up` | gauge | bool | — | Is the process answering at all? (drives `up`-style alerting) |
| `obo_cron_loop_running` | gauge | bool | — | Is the solver loop started, or sitting in the "no solver available" state? |
| `obo_solves_total` | counter | count | `outcome` ∈ {`succeeded`,`failed`,`rejected`} — **the wire spelling is `succeeded`, not `success`** (`internal/metrics/prom.go:40`) | Is the lab producing answers, and is the failure rate sane? |
| `obo_queue_pending` | gauge | count | ~~`status` ∈ {`pending`,`deferred`,`in_progress`}~~ **none** — one count for all three, matching `/api/v1/stats` `queue_depth` (`internal/metrics/prom.go:162-166`, §4.4) | Is work backing up? |
| `obo_solve_duration_seconds` | histogram | seconds | `outcome` | How long does a unit of work take, and what is the p95? |

`obo_solve_duration_seconds` buckets: `30, 60, 120, 180, 300, 600, 1200, 2400` — chosen around
the observed mean of 226 s (3354 completed solves, `AVG(completed_at-started_at)`) and the live
`avg_solve_time: 4m42s`.

### 3.2 Tier 2 — scheduler health and starvation

**LANDED 2026-11-16 — `6c4e09aa` (OBS-OB-002).** Collector `internal/metrics/prom.go:520-575`, sourced from
`internal/metrics/metrics.go:541` `SchedulerHealth` — the same Observer the loop records into, so a scrape cannot
disagree with the JSON snapshot. Both counters are pre-created at zero (`internal/metrics/prom_scheduler_test.go:72`),
because a missing series is invisible to `rate()` while a zero series is not; the three gauges are omitted until the
loop has a reading. Label sets are closed and match this table.

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_cron_ticks_total` | counter | count | `outcome` ∈ {`solved`,`empty`,`idle_gated`,`error`} | Is the loop ticking, and what does each tick do? |
| `obo_cron_skip_reason_total` | counter | count | `reason` ∈ {`load`,`memory`,`load+memory`} | Why is it not working right now? (the closed set is `internal/metrics/metrics.go:111` `SkipReasons`, produced by `TrimSpaceReason` `:732`) |
| `obo_host_load1` | gauge | ratio | — | How contended is the host the gate is judging? (mirrors `HostSnapshot.Load1`, `metrics.go:169`) |
| `obo_host_mem_used_fraction` | gauge | ratio (0–1) | — | How close is the host to the memory gate? (`metrics.go:170`) |
| `obo_cron_last_tick_timestamp_seconds` | gauge | unix seconds | — | How stale is the loop? (catches a hung or dead loop that still answers HTTP) |

### 3.3 Tier 2 — solve resources and failure causes

**LANDED 2026-11-06 — `e066e203` (OBS-OB-003).** Registrations `internal/metrics/prom.go:223-267`; collectors
`:441-466` (peak RSS histogram), `:478-501` (killed / over-budget / rejected counters), `:581-600` (failure causes),
sourced from `internal/metrics/metrics.go:367` `SolveResources` (struct `:235-251`). One design note worth keeping: the `cause` label is
**classified at the failure site**, never by parsing `failure_reason` text, and the vocabulary is closed in
`internal/solver/cause.go:62-71` — so `ClassifyCause` cannot grow the label cardinality, and unattributable failures
land in `cause="other"` rather than inventing an eighth series. The "enumerate from `failure_reason` prefix matching"
line in the last row is the *original* suggestion and is the one part the implementation deliberately did better.

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_solve_peak_rss_bytes` | histogram | bytes | — | Distribution of solve memory; catches a 47 GB-style regression before the OOM. Buckets: `268435456, 536870912, 1073741824, 2684354560, 4294967296` |
| `obo_solve_killed_total` | counter | count | — | How often does the memory cap actually kill a solve? (`metrics.go:246` `Killed`) |
| `obo_solve_over_budget_total` | counter | count | — | How often does a solve exceed the *alert* threshold? (`metrics.go:248` `OverBudget`) |
| `obo_solve_rejected_total` | counter | count | — | How often does the harness refuse a near-cap solve? (`metrics.go:251` `Rejected`) |
| `obo_solve_failure_total` | counter | count | `cause` ∈ {`timeout`,`solver_exit`,`toolchain_missing`,`invalid_problem`,`commit`,`stale_reaped`,`other`} | Why do solves fail? Enumerate from `queue_entries.stage` (`solver_failed`,`commit_failed`) + `failure_reason` prefix matching (`solver/failure_hint.go`). This is the one metric that turns 2196 opaque failure rows into an alertable series. |

### 3.4 Tier 3 — queue age, catalogue, and storage

**LANDED — `fe3be63e` (OBS-OB-004), commit stamp 2026-11-16 19:38 -0500, merged into `master` at `78b47e1c`.**
All six families below are served at `GET /metrics/prom`, and no SQL statement or `stat()` runs inside the scrape
handler: the readings are held in `dbGaugeCache` (`internal/metrics/prom_db.go:85-104`), filled by `RefreshDB`
(`:111-122`) on a **15 s** ticker (`DefaultDBRefreshInterval`, `:29`) started at `cmd/off-by-one/main.go:332`, with the
first read taken synchronously so a fresh process serves real values immediately. `dbGaugeCollector.Collect`
(`:174-231`) only copies the cached struct, so a scrape storm cannot become a SQL storm (§4.4). Each leg carries its own
`…OK` flag: a failed read omits that series instead of reporting 0, and one dead leg never blanks the others.

| Metric | Type | Unit | Labels | Fed from — and the question it answers |
|---|---|---|---|---|
| `obo_queue_oldest_pending_age_seconds` | gauge | seconds | — | `now - MIN(created_at)` over `status IN ('pending','deferred','in_progress')`, read by `Queue.OldestPendingAge` (`internal/ingest/queue.go:667`). Is a submission stuck? An empty queue is a real 0; a failed query omits the series (`prom_db.go:187-193`). |
| `obo_queue_deferred_total` | counter | count | `reason` ∈ {`load`,`memory`,`load+memory`} | `Observer.Deferrals` (`internal/metrics/metrics.go:452`), one increment per entry the host-pressure gate actually stamped deferred; every closed reason is present at 0 on every scrape (`internal/metrics/prom.go:385-404`, wired `main.go:297`). How often does the gate postpone real work? (column `queue_entries.gate_reason`, `queue.sql:26`) |
| `obo_catalog_verified_answers` | gauge | count | — | `graph.Store.Stats().VerifiedAnswers` (`internal/graph/store.go:575`, query `:595-601`) — the same figure `/api/v1/stats` serves (`prom_db.go:194-203`, wired `main.go:269-273`). Is the catalogue growing? |
| `obo_catalog_hit_rate` | gauge | ratio | — | Derived in the collector as verified/total from that same `Stats` read (`prom_db.go:206-210`), 0 on an empty catalogue, so it cannot disagree with the two counts arithmetically. Does the catalogue actually answer lookups? |
| `obo_discovery_requests_total` | counter | count | `outcome` ∈ {`hit`,`miss`} | `Observer.DiscoveryCounts` (`internal/metrics/metrics.go:603`) over the closed `DiscoveryOutcomes` set (`:119`), wired `main.go:293-296`; collector `internal/metrics/prom.go:410-433`. Is discovery finding answers? |
| `obo_db_size_bytes` | gauge | bytes | `kind` ∈ {`main`,`wal`} | `os.Stat` on the `--db` path and its `-wal` sibling (`main.go:274-287`); closed `kind` set `DBKinds` (`prom_db.go:39`), a missing `-wal` is a real 0, a failed `stat` omits that part. Is the store growing unboundedly? |

**Measured on the wire, 2026-11-16** (scratch process on `:18966`, copy of the live 59 MB store, no solver — so the
loop never ticked):

```
obo_queue_oldest_pending_age_seconds 59322.92099744   # SQL: 59323.2  (Δ 0.3s = cache age)
obo_queue_pending                    27               # SQL: 27
obo_catalog_verified_answers         5033             # SQL: 5033 of 5061 -> hit rate 0.994467…
obo_catalog_hit_rate                 0.9944674965421854
obo_db_size_bytes{kind="main"}       5.906432e+07     # stat: 59064320
obo_db_size_bytes{kind="wal"}        4.305432e+06     # stat: 4305432
obo_queue_deferred_total{reason="load"} 0   (all three reasons present at 0)
obo_discovery_requests_total{outcome="hit"} 0  {outcome="miss"} 0
```

Re-measured 2026-11-16 21:36 on the same port: five consecutive scrapes of that store took **0.48-0.57 ms**
(the first, cold scrape 3.1 ms), and a copy without the `-wal` sibling rendered `obo_db_size_bytes{kind="wal"} 0`
— the documented "a missing `-wal` is a real 0, not a missing reading" behaviour, observed rather than assumed.
The `15 s` refresh is the only lag source: the gauge is 0-15 s stale by construction, which is why the Step 4
threshold (`> 21600`) is expressed in hours.

### 3.5 Explicitly excluded

- `submission_id`, `problem_class`, `model` as labels — unbounded cardinality. Model-level
  solve counts, if ever needed, belong in an aggregate offline query against `queue_entries`,
  not in a scrape.
- Per-request HTTP metrics (route/method/status) — valuable but belongs to a shared
  instrumentation layer (see row 4); it is not what makes this product observable.
- Anything derived by re-querying SQLite on every scrape — see §4.4 for why the DB-derived
  gauges must be cached, not computed per request.

---

## 4. Exposure mechanism

### 4.1 In-process `client_golang` + `promhttp` on the existing server — **LANDED 2026-11-04, `62b1c03b`**

The recommendation below is kept verbatim as the audit wrote it, with the landing marked per bullet. Read the
strikethroughs as "done", not as "withdrawn".

The HTTP surface already exists and already routes `/metrics`
(`internal/api/server.go`, `mux.HandleFunc("GET /metrics", s.handleMetrics)`; the route table itself has since moved
to `server.go:184`), so the change is additive and small:

- ~~Add `github.com/prometheus/client_golang/prometheus` + `.../promhttp` to `go.mod`
  (currently absent — `grep -rn "prometheus|promhttp|client_golang"` over `*.go`/`*.mod`
  returns **zero** hits).~~ **Done in `62b1c03b`:** `github.com/prometheus/client_golang v1.24.1` is a direct require
  at `go.mod:7` (now `v1.25.0`), imported at `internal/metrics/prom.go:27-28`. That zero-hit grep is now the opposite of true — it is
  a good example of why a vendored dependency claim in a document needs re-checking against `go.mod`, not memory.
- ~~Register a private `prometheus.Registry` (not the global default) owned by the `Server`, and
  serve `promhttp.HandlerFor(reg, ...)` at a new path or under content negotiation.~~ **Done:** private registry at
  `internal/metrics/prom.go:129`, `promhttp.HandlerFor(reg, promhttp.HandlerOpts{})` at `:302`, held on
  `api.Server.Prom` (`internal/api/server.go:71-79`) and served by `handleMetricsProm` at `server.go:397-399`.
- **Do not replace the existing JSON `/metrics`** — the dogfood docs and operator runbooks
  already use it (`docs/dogfood/diagnostics.md:1075-1116`,
  `docs/dogfood/2026-10-16-operator-integration.md:25,48,92`). Preserve JSON, add text format.
  **Honoured:** `handleMetrics` still serves the JSON snapshot at `internal/api/server.go:380-387`, untouched, and a
  regression test pins that (`TestMetricsJSONUnchangedAfterProm`, `internal/api/metrics_prom_test.go:156`).

Suggested split, to avoid breaking existing consumers — **implemented exactly this way:**

- `GET /metrics` — **keep** the current JSON snapshot (unchanged).
- `GET /metrics/prom` — **new**, `promhttp`, `Content-Type: text/plain; version=0.0.4`.

Content negotiation was **not** used; the new-path option was chosen, as recommended. The route is registered
unconditionally and a nil `Prom` serves a zero-source registry (`internal/metrics/prom.go:310`: `sharedZeroProm`,
with nil-receiver-safe `Handler`/`Gatherer`), so the endpoint never 404s — a deliberate choice that makes "the route
answers HTML/404" diagnosable as a deployment fault rather than a missing feature.

### 4.2 Wiring the dead counters — **LANDED (`62b1c03b`, extended by `e066e203` / `6c4e09aa`)**

~~The counters in `internal/cron.Metrics` (`loop.go:101-117`) are computed but exposed nowhere.~~
Both options the audit offered were declined in favour of a third, cleaner one: `main` injects **accessors** into
`metrics.PromConfig` instead of handing the server a `*cron.Metrics` or teaching `cron.Metrics` to implement
`Collector` (`cmd/off-by-one/main.go:249-319`):

- `SolveCounts` closes over `loop.Metrics().Snapshot()` → `obo_solves_total{outcome}`;
- `SolveResources` is `observer.SolveResources` → the §3.3 families;
- `Scheduler` is `observer.SchedulerHealth` → the §3.2 families;
- `main.go:322` installs the histogram sink (`loop.Metrics().SetDurationObserver(prom.ObserveSolveDuration)`), so
  `obo_solve_duration_seconds` is recorded per completed solve rather than rebuilt at scrape time.

The effect of keeping the loop's counters as the single source is that the exposition and the JSON surfaces cannot
drift: both read the same `Snapshot()` / `Observer`.

~~`main.go:241` is the natural template: one more field on `api.Server`, assigned in `main`.~~
That is still the shape used — `api.Server.Prom` — but the field carries the exposition object, not the raw counters.

### 4.3 Solving the loop-starvation case specifically — **query now runnable (`6c4e09aa`)**

`obo_cron_skip_reason_total{reason}` and `obo_host_load1` together make the current silent
starvation alertable: `rate(obo_cron_skip_reason_total{reason="load"}[30m]) > 0` sustained for
30 minutes while `rate(obo_solves_total[1h]) == 0` is an unambiguous "the lab is idle-by-gate".

Both sides of that query exist in the tree as of `6c4e09aa`. It is untested against live data, because nothing
scrapes the endpoint yet (§5) — so treat the *expression* as reviewed, not as exercised.

### 4.4 DB-derived gauges: cache, do not query per scrape — **partly implemented, with one deviation**

`obo_queue_pending`, `obo_queue_oldest_pending_age_seconds`, `obo_catalog_*`, and
`obo_db_size_bytes` each require a SQLite query or a `stat()`. The queue's paged reads were
explicitly re-indexed to avoid full sorts (`sql/schema/queue.sql:32-45`, "status-filtered page
17.3ms → 0.11ms"), which shows the authors care about query cost on this store. Refresh these
into gauges **on a ticker (15 s)** inside the process, not in the scrape handler, so a scrape
storm cannot turn into a SQL storm. `Depth` (`queue.go:646-652`) is an index-only `COUNT` and is
safe; `oldest_pending_age` over an index on `(status, created_at)` is likewise cheap.

Note SQLite single-writer semantics: the cron loop already holds the writer. Read-only gauge
refreshes should use the existing read path (or a `mode=ro` connection) rather than contending
for the write lock mid-solve.

**What shipped (2026-11-16 — `62b1c03b`, then `fe3be63e` which moved the last family onto the cache):** the advice
was followed. The first landing did collect `obo_queue_pending` *per scrape* inside `Collect`, behind a 2 s timeout —
the deviation this section used to describe — and `fe3be63e` removed it: every DB-derived family is now a cache read.

- the readings live in `dbGaugeCache` (`internal/metrics/prom_db.go:85-104`) and are filled by `RefreshDB`
  (`:111-122`) under a 2 s refresh timeout (`:34`) that also serialises refreshes (`dbRefreshMu`), so a slow source
  cannot stack goroutines against a wedged store;
- `StartDBRefresh` (`:129-149`) takes the first reading **synchronously** — a fresh process serves real values on its
  first scrape rather than an empty gauge set — then re-reads every **15 s** (`DefaultDBRefreshInterval`, `:29`);
- `dbGaugeCollector.Collect` (`:174-231`) copies the cached struct under a read lock and touches neither SQLite nor
  the filesystem, which is what makes the scrape budget independent of DB size;
- each leg has its own `…OK` flag, so a failed read (a locked store, a missing `-wal`) omits only its own series
  (`:180-230`) instead of blanking the others or fabricating a zero.

Measured: a 59 MB store with a 4.3 MB WAL answered in 0.40-0.65 ms per scrape (five scrapes, scratch process), and
`obo_queue_oldest_pending_age_seconds` tracked the SQL answer to 0.3 s — the refresh interval, not a query, is the
only source of lag. The refresher only reads (`Queue.Depth`, `Queue.OldestPendingAge`, `graph.Store.Stats`,
`os.Stat`), so the single-writer note above still applies unchanged to the cron loop.

Also: `obo_queue_pending` is emitted **without** a `status` label, unlike the §3.1 table. One number for
pending+deferred+in_progress matches `/api/v1/stats` and avoids a label whose values the collector cannot cheaply
decompose; if per-status depth is wanted later, it needs three `Depth`-style counts, not a relabel of this one.

---

## 5. Collection path

**Status 2026-11-16: exposition side DONE (`62b1c03b` + `e066e203` + `6c4e09aa` + `fe3be63e`), collection side OPEN.**
Everything below the line is the audit's recommendation and still stands, with one measured change: the endpoint
exists now, so the open question is purely "who scrapes it". Nothing in this repository configures a scrape — no
`job_name`, `scrape_configs` or `remote_write` appears in any tracked `*.yaml`/`*.yml`/`*.toml`/`*.md`, and `deploy/`
contains only `off-by-one.service`. Tracked as board row **OBS-OB-005** ("the monitoring stack is out of repo scope").

The fleet posture (given): agents **push** to Prometheus via `remote_write`; anything
scrape-only needs either a new scrape job in the fleet Prometheus config or push.

- **Recommended:** a scrape job (the service is a long-running HTTP server, so pull is correct
  and gives `up{}` for free):

  ```yaml
  - job_name: off-by-one
    metrics_path: /metrics/prom
    static_configs:
      - targets: ["off-by-one.internal:8766"]
        labels:
          product: off-by-one
          service: off-by-one
          host: <hostname>
  ```

  This config lives on `fleet-monitor` and is **out of scope for this audit** — it must be
  added by whoever owns the monitoring stack, not by this product.
- **Fallback if no scrape job is permitted:** a tiny sidecar/alloy `prometheus.scrape` +
  `remote_write` on the off-by-one host, pushing the same series. `alloy` is already in the
  fleet stack.
- **Never** have the app talk to Prometheus directly; the product must not own transport.
- Bind address: the unit currently serves on `:8766` (`deploy/off-by-one.service:69` — the `ExecStart` line, since the
  args moved down the file; `-load-threshold 4 -cron-interval 60s -solve-mem-mb 12288`, listening on `127.0.0.1:8766`
  per `ss -ltnp`). A scrape from another host needs either a
  bind change or a local scrape agent. `main.go` already warns loudly when binding
  non-loopback because there is no auth — so **prefer the local-agent push/scrape** over
  exposing the port.

---

## 6. Verification — how a reviewer proves each metric is live

**Status of this section, 2026-11-16 (second pass).** Step 0 is enforced as a unit test; Steps 1-4 all have their
series in the tree as of `fe3be63e`, and all four were run against **scratch processes** on 2026-11-16 — `:18966`
(DB copy, no solver, 18 families) and `:18967` (solver wired, loop ticking, 21 families) — see §3.4's measured block
and each step's status line. What remains unproven is the **deployed** unit: it has not restarted since 2026-11-04,
so the process on `127.0.0.1:8766` cannot serve the newer families, and a docs task must not restart a production
unit. No step has been run against the fleet Prometheus, because nothing scrapes the endpoint (§5).

Step 0 — the endpoint speaks Prometheus at all:

```bash
curl -s -D- http://127.0.0.1:8766/metrics/prom | head -20
# DONE when: HTTP/1.1 200, Content-Type: text/plain; version=0.0.4
# and at least one line starts with "# HELP obo_"
```

**Step 0 status: DONE in-repo, UNVERIFIED against the running process.** All three assertions above are enforced by
`TestMetricsPromEndpoint` (`internal/api/metrics_prom_test.go:33`), including the trap it guards against: the SPA
catch-all also answers `/metrics/prom` with 200, so a status-only check would pass on an HTML body. The test asserts
the `Content-Type` prefix and the absence of `<!DOCTYPE`.

**The same curl, against a scratch process instead (2026-11-16).** The binary built from this tree, started on
`:18966` with a copy of the live DB and no solver, answered:

```
HTTP/1.1 200 OK
Content-Type: text/plain; version=0.0.4; charset=utf-8; escaping=underscores
```

with 18 `# TYPE obo_*` lines, and **21** once the cron loop was allowed to tick — the 3 missing on the no-solver run
are exactly the tick-dependent gauges (`obo_host_load1`, `obo_host_mem_used_fraction`,
`obo_cron_last_tick_timestamp_seconds`). The `charset` / `escaping` parameters are appended by `client_golang` v1.25;
the asserted prefix `text/plain; version=0.0.4` is unchanged.

Why not the deployed unit: it has not restarted since **2026-11-04 17:28:53** (`systemctl show off-by-one -p
ActiveEnterTimestamp`), which predates `e066e203` (2026-11-06), `6c4e09aa` and `fe3be63e` (both 2026-11-16); the
binary on disk was rebuilt 2026-11-16 14:48, also before the last two. So the running process cannot be serving the
newer families, and a docs task must not restart a production unit — the restart is owner-gated (board row
OBS-OB-003's deferral note). **To close this:** restart the unit, then re-run Step 0 and confirm 21 families appear.

Step 1 — tier 1 series present and non-zero:

```promql
obo_http_up{product="off-by-one"}                       # == 1
obo_cron_loop_running{product="off-by-one"}             # == 1
sum by (outcome) (increase(obo_solves_total[1h]))       # non-empty once solves run
obo_queue_pending                                        # matches /api/v1/stats queue_depth
histogram_quantile(0.95, rate(obo_solve_duration_seconds_bucket[1h]))   # sane (< 3600)
```

**Label caveat (verified 2026-11-16):** no `obo_*` family carries a `product`/`service`/`host` label of its own — every
descriptor in `internal/metrics/prom.go` uses `nil` variable labels. The `{product="off-by-one"}` selectors above only
work once the scrape job attaches those as target labels (§5) or Prometheus adds them via `external_labels`. Against a
bare `curl` of the process, drop the selector. Likewise `sum by (outcome)` yields `succeeded|failed|rejected`, not
`success`.

Cross-check against the JSON surface, which is the current ground truth:

```bash
curl -s http://127.0.0.1:8766/metrics | jq '.skipped_host_pressure'
curl -s http://127.0.0.1:8766/api/v1/stats | jq '.queue_depth'
```

Step 2 — starvation is detectable (the reason this work matters):

```promql
rate(obo_cron_skip_reason_total{reason="load"}[30m]) > 0
# and: obo_cron_last_tick_timestamp_seconds should still advance — a starving loop ticks
```

Step 3 — memory regression is detectable:

```promql
histogram_quantile(0.99, rate(obo_solve_peak_rss_bytes_bucket[24h])) > 1073741824
increase(obo_solve_killed_total[1h]) > 0
```

Step 4 — stuck queue is detectable:

```promql
obo_queue_oldest_pending_age_seconds > 21600    # 6h
```

**Step 4 status: PASSES on new evidence (2026-11-16 — `fe3be63e`).** `obo_queue_oldest_pending_age_seconds` is Tier 3
(OBS-OB-004, §3.4) and *is* emitted: descriptor `internal/metrics/prom.go:167-171`, gauge written by the cached
collector `internal/metrics/prom_db.go:187-193`, fed by `Queue.OldestPendingAge` (`internal/ingest/queue.go:667`).
Measured on the scratch process — `obo_queue_oldest_pending_age_seconds 59322.92099744` against `59323.2` from

```sql
SELECT ROUND((julianday('now') - julianday(MIN(created_at))) * 86400, 1)
  FROM queue_entries WHERE status IN ('pending','deferred','in_progress');
```

— a 0.3 s difference, i.e. the age of the cached reading, not a disagreement. So `> 21600` is evaluable from a scrape.

**Residual gap, stated exactly:** the *deployed* unit cannot answer it yet (no restart since 2026-11-04, above), and
nothing scrapes `/metrics/prom` (§5, OBS-OB-005). The gap is the deployment and the scrape job — **not** the family.
An empty result is now diagnostic in only two ways: the binary predates `fe3be63e`, or the refresher has not completed
a read (check the `…OK` flags in `prom_db.go`); it no longer means "the family does not exist".

**Step 3 status: series exist as of `e066e203`** (`obo_solve_peak_rss_bytes_bucket`, `obo_solve_killed_total`);
unexercised live for the restart reason in Step 0.

**Done looks like:** all of Steps 0–4 answerable, and Prometheus shows `up{job="off-by-one"} == 1`
with ≥ 21 `obo_*` families (the exact set §0 counts: 5 Tier-1 + 6 Tier-3 + 5 solve-resource/cause + 5 scheduler),
plus one alert rule firing on Step 2 in a deliberately gated
repro (run with `-load-threshold 0.1` and confirm the alert fires, then restore).
Note that this done-condition spans two owners: the family count is this repo's work and is testable now; `up{}`, the
`job` label and the alert rule all require the fleet scrape job (§5, OBS-OB-005), which is outside this repository.

---

## 7. Deliberately not instrumented, and why

- **Per-submission / per-problem-class / per-model labels.** `SubmissionID` and `ProblemClass`
  are per-request identifiers (`internal/metrics/metrics.go:63-64`). One label per submission is
  unbounded cardinality and will destroy the TSDB. Solve durations are labelled by `outcome`
  only; per-model analysis is an offline SQL question against `queue_entries`.
- **The `/metrics` JSON payload itself as a metric.** Do not encode free-text
  `last_skip_reason` or `failure_reason` as a label value beyond the fixed vocabulary.
- **Prometheus client self-metrics initially.** `client_golang` emits `go_*` and
  `promhttp_*` by default; they are noise here (the fleet already sees ~2000 families of
  node/exporter/self noise for ~20 real application series). Register a private registry and
  **do not** register `prometheus.NewGoCollector()` unless process-level detail is specifically
  wanted. **Held at landing (checked 2026-11-16):** `internal/metrics/prom.go` registers no Go or
  process collector, so `/metrics/prom` exposes the application families only — the 21 in §0 and nothing else.
- **Pushgateway for this service.** It is long-running, so pull is correct. (A pushgateway /
  textfile completion record *would* be right for a short-lived CLI lane — e.g. the `seed`
  subcommand at `cmd/off-by-one/seed.go:172`, which prints a one-shot summary and exits. That
  is out of scope here but note it: seed runs are invisible after they exit.)
- **Per-request HTTP latency/status metrics.** Correctly belong to a shared instrumentation
  middleware, not hand-rolled per handler. Filed as a separate later row.

---

## 8. Open questions this audit could not resolve

- **Scrape job — OPEN, re-checked 2026-11-16.** Whether the fleet Prometheus has any `off-by-one` scrape job.
  Inside this repo the answer is definitionally no: no `job_name`, `scrape_configs` or `remote_write` appears in any
  tracked config-like file, and `deploy/` holds only the systemd unit. The 2026-11-03 evidence (0 scraper traffic,
  process up 1h29m, `solve_count: 0`, no scrape series) still points the same way, and board row OBS-OB-005 is still
  `pending` with the note "the monitoring stack is out of repo scope". **What would close it:** the `fleet-monitor`
  Prometheus config containing `job_name: off-by-one` with `metrics_path: /metrics/prom`, plus a live query returning
  `up{job="off-by-one"} == 1`. Neither is obtainable from inside this repository, so it stays OPEN here rather than
  being asserted either way.
- **Live family count — ANSWERED for a scratch process, OPEN for the deployed one.** A binary built from this tree
  serves all **21** committed families: measured 2026-11-16 on `:18967` with the real solver wiring and the production
  load gate, the exposition carried 21 `# TYPE obo_*` lines, with `obo_cron_loop_running 1`,
  `obo_cron_ticks_total{outcome="idle_gated"} 2`, `obo_cron_skip_reason_total{reason="load"} 2`,
  `obo_host_load1 14.53`. The process on `127.0.0.1:8766` cannot be serving them — it started 2026-11-04 17:28, before
  `e066e203`, `6c4e09aa` and `fe3be63e`. Evidence that would close the deployed half: an owner-approved restart, then
  the Step 0 curl.
- Whether the `-load-threshold 4` vs observed `load1 ≈ 14.8` starvation is intended backlog
  shedding or a sizing mistake. That is an owner/product question, not an observability one —
  but it is exactly why the signal must exist. **As of `6c4e09aa` the signal exists**: the question is now answerable
  from `obo_cron_skip_reason_total{reason="load"}` instead of by reading logs, once something scrapes it.
