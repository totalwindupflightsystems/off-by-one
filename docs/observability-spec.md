# Off-by-One — Observability Spec

Status: **audit + target spec**. Written 2026-11-03 by a monitoring lane. No service code was
modified to produce this document.

Naming convention for every metric below: `<product>_<subsystem>_<thing>_<unit>`.
Product prefix is `obo` (short form of off-by-one, matching the existing `oby-memcap` binary).

---

## 1. What this service is, and what "healthy" means

Off-by-One is a **pre-solve lab**, not a GPU worker farm. It maintains a graph of
problem classes → pre-verified answers, accepts submissions onto a queue, and a single
background cron loop solves them by driving **pi-agent inside a bwrap sandbox**, then commits
the answer into the SQLite graph.

Evidence: `cmd/off-by-one/main.go:1-10` (process roles: graph store, ingest queue, API server,
cron loop); `internal/solver/piagent.go:50` (pi-agent + provider key); `internal/sandbox/bwrap.go`
(sandbox); `internal/cron/loop.go:286` (loop wiring).

There is **no GPU component**: `grep -rin "gpu|nvidia|cuda|vram"` over `*.go` returns zero
implementation hits (only unrelated corpus strings in `data/INDEX.md` and a CSS comment in
`docs/landing-spec.md:247`). The "idle GPU time" framing in the task brief does not match this
repository. What it actually farms is **host idle time**, gated on load average and memory
(`internal/cron/loop.go` `checkIdle`, memory + load halves; `internal/cron/memgate.go`).

**Healthy means all five hold:**

1. The HTTP API answers `/health` 200 and `/metrics` 200 (not 404 `metrics_disabled`).
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

---

## 2. Blind spots, with the code location where the state lives but is never emitted

| Operator question | Where the truth already lives | Why it is currently invisible |
|---|---|---|
| Is the process up? | `internal/api/server.go:303-309` (`/health`) | Only answers if a human curls it; no `up{}` series, no scrape job. |
| **Is the loop doing work, or starving?** | `internal/metrics/metrics.go:224` `SkippedHostPressure`, `loop.go:580` `recordIdleSkip` | Exposed only as JSON at `/metrics`; no Prometheus text format, no scrape job ⇒ unalertable. |
| How much work, and of what kind? | `internal/cron/loop.go:101-111` `Metrics` counters (`SolveAttempts`/`Success`/`Failed`), `MetricsSnapshot` at `loop.go:176-186` | **Dead code path**: `MetricsSnapshot`'s own comment says "*It is what /api/v1/stats (or the local metrics endpoint) will eventually expose*" (`loop.go:174-176`). `handleMetrics` (`server.go:313-320`) serves the *Observer* snapshot instead, which has no attempts/success/failed at all. The per-solve win/loss counters are computed and thrown away. |
| How long does a solve take? | `internal/ingest/queue.go:673-692` `AvgSolveTime` (mean over last 20) | Only a rounded string in `/api/v1/stats`; no distribution, no per-model split, not scrappable. |
| How often does it fail, and why? | `queue_entries.failure_reason` (`sql/schema/queue.sql:26`), 2196 failed rows on disk | Free-text in SQLite. Grouped offline it is ~1981 empty reasons, 112 `pi timed out after 25`, 40 `pi exited with code 1`, 29 `bwrap: Can't find source path`, 18 `invalid problem file`. No counter, no labels, no alert. |
| Is a queue/lag building? | `queue_entries.status` (`sql/schema/queue.sql:19`); live read at `queue.go:646-652` `Depth` | Depth is a single number in `/api/v1/stats`; **age** of the oldest unfinished entry is never computed anywhere, so a 6-hour stuck entry looks like a 6-second one. Observed: entries pending since 11:54Z while the newest complete is 18:24Z. |
| Are submissions deferring (gated) and why? | `queue_entries.gate_reason` (`queue.sql:27`), `deferred` status | Only visible per-row via `GET /api/v1/queue/{id}`. Live sample: 1 deferred with `gate_reason=load`. Not countable as a series. |
| Is memory about to be blown? | `internal/metrics/metrics.go:39` alert threshold, `:121-138` `RecordSolve` | Counters exist (`over_budget`, `peak_rss_bytes_max`) but are **only in JSON**, and `solve_count: 0` means the memory half of the gate has never recorded a solve at all. |
| Is its data growing / is the disk safe? | `off-by-one.db` is 56 MB with a 1.15 MB WAL; `answer_nodes` 4885 rows | No size, no row-count, no WAL-bytes, no vacuum-age series. |
| Is the provider credential/quota about to fail? | `cmd/off-by-one/main.go:154-161` provider key resolution; startup-only `WARNING: DEEPSEEK_API_KEY is empty or placeholder` | The credential is checked **once at boot** and never re-checked. A revoked or exhausted key degrades into a stream of `pi-agent exec: exit status 1` failures counted nowhere. Live `/api/v1/metrics` shows 1 hit / 2 misses; the `pi timed out after 25` cluster (112) is the largest real failure cause and has no dedicated counter. |

Correction to the static scan in the brief: the hypothesis `endpoint_metrics=3 … own_counter_or_gauge=1`
**undercounted**. There is a dedicated `internal/metrics` package (314 lines), a registered
`GET /metrics` route, and substantial counter state in `internal/cron.Metrics`. The precise
defect is not "no metrics" — it is **"metrics exist in a bespoke JSON shape that no collector
speaks, over a route no collector is configured to read, and one counter set (`cron.Metrics`)
is not wired to any endpoint at all."**

---

## 3. Metric catalogue

Cardinality rule: labels below are the *complete* allowed set. **Never** label by
`submission_id`, `problem_class`, `model`, file path, prompt text, or timestamp — the first two
are per-request identifiers (`internal/metrics/metrics.go:53` `SubmissionID`, `:54` `ProblemClass`
show how tempting and how wrong that is). `outcome` and `reason` are closed enumerations.

### 3.1 Tier 1 — minimal viable emission (the 5 that answer "is it working?")

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_http_up` | gauge | bool | — | Is the process answering at all? (drives `up`-style alerting) |
| `obo_cron_loop_running` | gauge | bool | — | Is the solver loop started, or sitting in the "no solver available" state? |
| `obo_solves_total` | counter | count | `outcome` ∈ {`success`,`failed`,`rejected`} | Is the lab producing answers, and is the failure rate sane? |
| `obo_queue_pending` | gauge | count | `status` ∈ {`pending`,`deferred`,`in_progress`} | Is work backing up? |
| `obo_solve_duration_seconds` | histogram | seconds | `outcome` | How long does a unit of work take, and what is the p95? |

`obo_solve_duration_seconds` buckets: `30, 60, 120, 180, 300, 600, 1200, 2400` — chosen around
the observed mean of 226 s (3354 completed solves, `AVG(completed_at-started_at)`) and the live
`avg_solve_time: 4m42s`.

### 3.2 Tier 2 — scheduler health and starvation

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_cron_ticks_total` | counter | count | `outcome` ∈ {`solved`,`empty`,`idle_gated`,`error`} | Is the loop ticking, and what does each tick do? |
| `obo_cron_skip_reason_total` | counter | count | `reason` ∈ {`load`,`memory`,`load+memory`} | Why is it not working right now? (the vocabulary is fixed by `internal/metrics/metrics.go:309-314` `TrimSpaceReason`) |
| `obo_host_load1` | gauge | ratio | — | How contended is the host the gate is judging? (mirrors `HostSnapshot.Load1`, `metrics.go:71`) |
| `obo_host_mem_used_fraction` | gauge | ratio (0–1) | — | How close is the host to the memory gate? (`metrics.go:72`) |
| `obo_cron_last_tick_timestamp_seconds` | gauge | unix seconds | — | How stale is the loop? (catches a hung or dead loop that still answers HTTP) |

### 3.3 Tier 2 — solve resources and failure causes

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_solve_peak_rss_bytes` | histogram | bytes | — | Distribution of solve memory; catches a 47 GB-style regression before the OOM. Buckets: `268435456, 536870912, 1073741824, 2684354560, 4294967296` |
| `obo_solve_killed_total` | counter | count | — | How often does the memory cap actually kill a solve? (`metrics.go:59` `Killed`) |
| `obo_solve_over_budget_total` | counter | count | — | How often does a solve exceed the *alert* threshold? (`metrics.go:62`, `:127`) |
| `obo_solve_rejected_total` | counter | count | — | How often does the harness refuse a near-cap solve? (`metrics.go:61`, `loop.go:698`) |
| `obo_solve_failure_total` | counter | count | `cause` ∈ {`timeout`,`solver_exit`,`toolchain_missing`,`invalid_problem`,`commit`,`stale_reaped`,`other`} | Why do solves fail? Enumerate from `queue_entries.stage` (`solver_failed`,`commit_failed`) + `failure_reason` prefix matching (`solver/failure_hint.go`). This is the one metric that turns 2196 opaque failure rows into an alertable series. |

### 3.4 Tier 3 — queue age, catalogue, and storage

| Metric | Type | Unit | Labels | Question it answers |
|---|---|---|---|---|
| `obo_queue_oldest_pending_age_seconds` | gauge | seconds | — | Is a submission stuck? (computed as `now - min(created_at)` over `status IN ('pending','deferred','in_progress')` — nothing in the codebase computes this today) |
| `obo_queue_deferred_total` | counter | count | `reason` ∈ {`load`,`memory`,`load+memory`} | How often does the gate postpone real work? (`gate_reason`, `queue.sql:27`) |
| `obo_catalog_verified_answers` | gauge | count | — | Is the catalogue growing? (`graph/stats` `VerifiedAnswers`, `internal/graph/store.go:575`) |
| `obo_catalog_hit_rate` | gauge | ratio | — | Does the catalogue actually answer lookups? (`store.go:577`, `:605`) |
| `obo_discovery_requests_total` | counter | count | `outcome` ∈ {`hit`,`miss`} | Is discovery finding answers? (`metrics.go:229-233`) |
| `obo_db_size_bytes` | gauge | bytes | `kind` ∈ {`main`,`wal`} | Is the store growing unboundedly? (56 MB main / 1.15 MB WAL today) |

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

### 4.1 Recommendation: in-process `client_golang` + `promhttp` on the existing server

The HTTP surface already exists and already routes `/metrics`
(`internal/api/server.go:97-118`, `mux.HandleFunc("GET /metrics", s.handleMetrics)`), so the
change is additive and small:

- Add `github.com/prometheus/client_golang/prometheus` + `.../promhttp` to `go.mod`
  (currently absent — `grep -rn "prometheus|promhttp|client_golang"` over `*.go`/`*.mod`
  returns **zero** hits).
- Register a private `prometheus.Registry` (not the global default) owned by the `Server`, and
  serve `promhttp.HandlerFor(reg, ...)` at a new path or under content negotiation.
- **Do not replace the existing JSON `/metrics`** — the dogfood docs and operator runbooks
  already use it (`docs/dogfood/diagnostics.md:1075-1116`,
  `docs/dogfood/2026-10-16-operator-integration.md:25,48,92`). Preserve JSON, add text format.

Suggested split, to avoid breaking existing consumers:

- `GET /metrics` — **keep** the current JSON snapshot (unchanged).
- `GET /metrics/prom` — **new**, `promhttp`, `Content-Type: text/plain; version=0.0.4`.

If content negotiation is preferred over a new path, serve text/plain only when
`Accept: text/plain` or `application/openmetrics-text`, else keep JSON. Either is acceptable;
the new path is the lower-risk one and is what the verification section below assumes.

### 4.2 Wiring the dead counters

The counters in `internal/cron.Metrics` (`loop.go:101-117`) are computed but exposed nowhere.
Either:

- have `Loop` register its counters into the same Prometheus registry via a `Describe`-style
  accessor, or
- simplest: emit them as `obo_cron_*` / `obo_solves_total` from the existing `Metrics` struct
  by adding a `Collector` implementation, and have the server hold the `*cron.Metrics` the way
  it already holds `*metrics.Observer` (`main.go:241` `apiServer.Observer = observer`).

`main.go:241` is the natural template: one more field on `api.Server`, assigned in `main`.

### 4.3 Solving the loop-starvation case specifically

`obo_cron_skip_reason_total{reason}` and `obo_host_load1` together make the current silent
starvation alertable: `rate(obo_cron_skip_reason_total{reason="load"}[30m]) > 0` sustained for
30 minutes while `rate(obo_solves_total[1h]) == 0` is an unambiguous "the lab is idle-by-gate".

### 4.4 DB-derived gauges: cache, do not query per scrape

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

---

## 5. Collection path

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
- Bind address: the unit currently serves on `:8766` (`deploy/off-by-one.service:62`,
  listening on `127.0.0.1:8766` per `ss -ltnp`). A scrape from another host needs either a
  bind change or a local scrape agent. `main.go:298` already warns loudly when binding
  non-loopback because there is no auth — so **prefer the local-agent push/scrape** over
  exposing the port.

---

## 6. Verification — how a reviewer proves each metric is live

Step 0 — the endpoint speaks Prometheus at all:

```bash
curl -s -D- http://127.0.0.1:8766/metrics/prom | head -20
# DONE when: HTTP/1.1 200, Content-Type: text/plain; version=0.0.4
# and at least one line starts with "# HELP obo_"
```

Step 1 — tier 1 series present and non-zero:

```promql
obo_http_up{product="off-by-one"}                       # == 1
obo_cron_loop_running{product="off-by-one"}             # == 1
sum by (outcome) (increase(obo_solves_total[1h]))       # non-empty once solves run
obo_queue_pending                                        # matches /api/v1/stats queue_depth
histogram_quantile(0.95, rate(obo_solve_duration_seconds_bucket[1h]))   # sane (< 3600)
```

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

**Done looks like:** all of Step 0–1 green, and Prometheus shows `up{job="off-by-one"} == 1`
with ≥ 12 `obo_*` families, plus one alert rule firing on Step 2 in a deliberately gated
repro (run with `-load-threshold 0.1` and confirm the alert fires, then restore).

---

## 7. Deliberately not instrumented, and why

- **Per-submission / per-problem-class / per-model labels.** `SubmissionID` and `ProblemClass`
  are per-request identifiers (`internal/metrics/metrics.go:53-54`). One label per submission is
  unbounded cardinality and will destroy the TSDB. Solve durations are labelled by `outcome`
  only; per-model analysis is an offline SQL question against `queue_entries`.
- **The `/metrics` JSON payload itself as a metric.** Do not encode free-text
  `last_skip_reason` or `failure_reason` as a label value beyond the fixed vocabulary.
- **Prometheus client self-metrics initially.** `client_golang` emits `go_*` and
  `promhttp_*` by default; they are noise here (the fleet already sees ~2000 families of
  node/exporter/self noise for ~20 real application series). Register a private registry and
  **do not** register `prometheus.NewGoCollector()` unless process-level detail is specifically
  wanted.
- **Pushgateway for this service.** It is long-running, so pull is correct. (A pushgateway /
  textfile completion record *would* be right for a short-lived CLI lane — e.g. the `seed`
  subcommand at `cmd/off-by-one/seed.go:172`, which prints a one-shot summary and exits. That
  is out of scope here but note it: seed runs are invisible after they exit.)
- **Per-request HTTP latency/status metrics.** Correctly belong to a shared instrumentation
  middleware, not hand-rolled per handler. Filed as a separate later row.

---

## 8. Open questions this audit could not resolve

- Whether the fleet Prometheus now has any `off-by-one` scrape job: `/root/fleet-monitoring/`
  was not readable from this session, so this is **unverified**. Given the observed 0 scraper
  traffic to `/metrics` (the process has been up 1h29m and `solve_count` is 0, and no scrape
  series exist), the working assumption is no job.
- Whether the `-load-threshold 4` vs observed `load1 ≈ 14.8` starvation is intended backlog
  shedding or a sizing mistake. That is an owner/product question, not an observability one —
  but it is exactly why the signal must exist.
