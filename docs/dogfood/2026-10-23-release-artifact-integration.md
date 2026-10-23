# Off-by-One — Dogfood run 20 (2026-10-23): the RELEASED artifact

**Verdict:** ✅ SHIPPABLE for the core promise (the released v0.2.0 binary installs,
seeds and serves the catalog with zero undocumented steps) — with four prose/UI
truth defects that the artifact itself exposes, filed as DF-OFF-BY-ONE-57…61.

**Angle (first exercise in 21 runs):** every previous install leg built from a
source checkout on a fresh box. This run drove the **delivery chain** — the
artifact a stranger actually downloads from GitHub Releases — and asked the
question the release notes cannot answer: *does the thing behind the download
button do what the README says it does?*

**Promise under test:** "`curl` the release binary, verify its SHA256, clone the
corpus, `seed`, serve, and discover a pre-verified answer — zero to a working
catalog in under a minute, no toolchain."

## Delivery-chain identity (resolved before anything was graded)

| identity | value |
|---|---|
| release tag | `v0.2.0` → tag commit `eabb6741d94efec99ffa83faaf87e33a9b395ad0` (2026-10-22 23:54:51 -0500) |
| Release object | published 2026-10-23T05:26:21Z, 5 assets |
| asset | `off-by-one-v0.2.0-linux-amd64` 11,727,010 B |
| `SHA256SUMS` | `c2638ce9…a013fc  off-by-one-v0.2.0-linux-amd64` → `sha256sum -c` **OK** |
| tarball | `off-by-one-v0.2.0-linux-amd64.tar.gz` 4,975,194 B → `sha256sum -c` **OK**, contains **only** the binary (1 entry) |
| checkout HEAD | `ea5b963a` (master) — 2 commits ahead of the tag, both CI/board only |
| running binary | `/health` → `{"status":"ok","uptime":"4s","version":"v0.2.0"}` |

The tag is 2 commits behind master; both of those commits touch `.github/` and the
board only, so "released artifact" and "checkout" are the same program here. That
was checked, not assumed — and it is why the findings below are product findings
rather than deployment drift.

## What was actually run (verbatim, in an isolated scratch)

```bash
# 1. the docs' own release recipe (README "Install from a release")
V=v0.2.0
base=https://github.com/totalwindupflightsystems/off-by-one/releases/download/$V
curl -fsSLO $base/off-by-one-$V-linux-amd64        # 0.40 s
curl -fsSLO $base/SHA256SUMS                       # 0.28 s
sha256sum -c --ignore-missing SHA256SUMS           # OK
chmod +x off-by-one-$V-linux-amd64
git clone --depth 1 https://github.com/totalwindupflightsystems/off-by-one   # 5.67 s
cd off-by-one && ../off-by-one-$V-linux-amd64 seed                           # 58.97 s
../off-by-one-$V-linux-amd64 --port 18801 -load-threshold 4                  # serve
```

Seed output:

```
seed complete: files=2856; classes=2856 created / 0 existing;
               answers=2955 created / 1530 skipped; edges=12872 created
```

Zero-to-catalog: **65.3 s** (download + verify + clone + seed) on a host at
loadavg 8.26 — against the README's "under a minute" clean-box claim. Everything
worked on the first try; no undocumented step, no sudo, no toolchain.

## What worked (driven, not inferred)

- **install + SHA256 chain** — both assets verify; `--version` → `off-by-one v0.2.0`.
- **seed** — 2856 classes / 2955 answers / 12,872 edges from a depth-1 clone; idempotent (1530 answers skipped on the same corpus).
- **serve** — binds loopback, reports its version in `/health`, cron loop starts with the shipped `loadThreshold=4.0`.
- **discover** — `python-sdk-e2e-battery` → `found:true` with the full solution text; unknown class → `404 {"error":"not_found"}`.
- **submit / queue** — new tuple → `200 sub_29044a` with `position:1`; same tuple again → `409 deduplicated reason:"queued"` carrying the same id; queue read keeps the tool-classification arrays.
- **dedup, answered arm** — `409` with `submission_id:""`, `position:0`, `reason:"answered"`, `next_action:"POST /api/v1/problems/discover"` (the 33ad3dd4 contract, present in the release).
- **taxonomy** — `{"limit":1000,"offset":0,"total":2856}` + "Showing 1000 of 2856 problem classes." in the UI (DF-53's fix holds in the artifact).
- **web UI** (headless Chrome, real DOM) — SPA renders, status bar live (`queue=1 cache=2955 hitrate=100.0%`), Explore drills class → env → lang → version → answer and the leaf renders the solution (DF-42 holds).
- **error contract** — invalid cadence → `400` naming the accepted values and echoing the rejected one.
- **gate** — at loadavg 8.26 with `-load-threshold 4` the lab deferred: `cron: deferred — host busy (load): used 8.260 / threshold 4.000`, `/metrics.skipped_host_pressure=1`, `last_skip_reason:"load"`.

## What failed (filed)

| row | defect | evidence |
|---|---|---|
| **DF-OFF-BY-ONE-57** | README's dedup prose promises "the existing `submission_id` … plus that entry's queue `position`" on every 409; the answered arm returns empty id + position 0. `docs/api-reference.md` and `docs/integration.md` were corrected by 33ad3dd4, README was not. | README.md:218-232 vs the live 409 body captured above |
| **DF-OFF-BY-ONE-58** | `seed` parses the corpus's `created_at` (seed.go:35,43) and never writes it; the graph insert omits the column (store.go:231,345) so the schema default stamps the seed-run clock. 2856 classes → **60 distinct** timestamps, all inside 2026-10-23 12:52:0x–5x; corpus says class 112 is `2026-07-25 01:10:08` with answers on 2026-07-25 and 2026-10-01. | `select count(distinct created_at), count(*) from problem_classes` = 60\|2856 |
| **DF-OFF-BY-ONE-59** | the SPA version chip renders **`vv0.2.0`** — `app.js:175` prepends `'v'` to a `/health` version that already carries it. The REVIEW-OB-021 fix wired the chip; the two halves were never composed in a browser. | browser `#status-version`.textContent = `vv0.2.0` |
| **DF-OFF-BY-ONE-60** | the README release recipe pins `V=v0.1.1`, so the documented first command installs an artifact **two releases behind** v0.2.0. Driven for real: v0.1.1's `/health` has no `version` (chip stays `v0.0.0`) and its answered-arm 409 has **no** `reason`/`next_action` — the pre-33ad3dd4 contract. | v0.1.1 binary, same seeded graph, same request |
| **DF-OFF-BY-ONE-61** | a healthy startup on an empty queue logs `cron: deferral stamp: peek queue head failed: queue: not found (continuing)` — error-shaped normal state, the class DF-47 fixed for the load-gate line. | serve.log at 07:53:03 and again on the 07:55 cold restart |

## Performance (Step 2b) — nothing a user would feel

Measured on the released binary, scratch HOME/state dir, host at loadavg 8.26.

| operation | command | number |
|---|---|---|
| download + verify | `curl` ×2 + `sha256sum -c` | 0.40 s + 0.28 s |
| corpus clone | `git clone --depth 1` | 5.67 s |
| **seed** (headline install op) | `off-by-one-v0.2.0-linux-amd64 seed` | **58.97 s** on ext4 (user 6.49 s, sys 1.45 s) vs **3.83 s** to `/dev/shm` — **15.4×**, see below |
| cold start → first `/health` | start + poll | **14 ms** |
| first `discover` after cold start | `curl POST /api/v1/problems/discover` | **1.8 ms** |
| **warm `discover`** | `hyperfine --warmup 3 --runs 20` | **7.0 ms ± 0.8 ms** (min 5.5, max 8.8; includes curl process start) |
| taxonomy page | `GET /api/v1/taxonomy` | 116.7 ms / 10.2 MB (1000 classes) |

One PERF row was filed — **not by this delivery**: the sibling delivery of this same tick
(`dogfood-off-by-one-2026-10-23-1243b`) profiled the seed step with strace and filed
`PERF-OFF-BY-ONE-1` (per-statement fsync, no transaction). I reproduced its core number
independently before crediting it: the SAME released binary and corpus seeded to `/dev/shm` in
**3.83 s** against this run's 58.97 s on ext4 (15.4×), and `internal/seed/seed.go` contains no
`Begin`/`Commit`/`tx.` at all (grep returns zero) — the loader autocommits every statement, so the
install step pays ~9k fsyncs. My first reading of the number ("I/O + link scan") was wrong: the wall
time is the autocommit path, not the link scan. A real user waits ~1 minute on real disk; that is a
perceptible cost, so it belongs in a PERF row and does.
Discover/submit/cold-start stay in the single-digit-millisecond range; no row is justified there.

## Install leg (fresh machine)

Two arms, both on `bunker-mvp`, both at HEAD `aae20cb5`/`ea5b963a`:

**A. Fleet battery** (`bunker-qa.sh launch/collect`, agent `d677d23c`, QA HEAD `ea5b963a`,
20 cells harvested from the agent and committed to
`docs/dogfood/evidence/2026-10-23-battery-off-by-one.jsonl`):

| cell | verdict |
|---|---|
| repo-head | OK (pull ff-only) |
| toolchain-bootstrap ×8 | OK — go 1.27.3 (floor 1.26.9), gcc/g++ wrappers, make 4.3, compose plugin, buildx v0.38.1, node v22.15.0 |
| fresh-install | OK (documented build path, module downloads clean) |
| ci-pass | OK — native suite PASS |
| ci-act | **UNVERIFIED** — `act` rc=1, truncated log (hosted CI is authoritative) |
| upgrade | OK — v0.1.1 tree → HEAD reinstall clean |
| chaos-errorpath | OK (clean error on missing config, rc=2) |
| chaos-resource / chaos-corruption | INFO — OOM under a 3 GiB cap (expected); truncated-db path selected by the harness |
| docker-deploy / chaos-shutdown | N/A — no compose file |
| ui-probe | INFO — "go-native start command is not a long-running server"; the harness cannot see this repo's embedded SPA (same class as QA-OFF-BY-ONE-20-R2) |
| chaos-disconnect | **UNVERIFIED** — `disc.log` 0 bytes (QA-OFF-BY-ONE-5-R2's vacuous cell) |

Two harness notes, neither a product defect: `collect` reported "still running" off a stale sentinel for
10+ minutes after the driver had exited (the 20 cells were pulled directly with `bunker exec`), and the
first `bunker destroy` was refused for live `sshd`/`dbus-daemon` under the agent uid — the destroy
succeeded after killing the session dbus, and the agent is gone from `bunker list` with its local SSH key
removed.

**B. Independent verbatim arm** (same fresh agent): `git clone --depth 1` → `CLONE_HEAD=aae20cb5` →
`make build` 10.29 s (**warm** — the battery's fresh-install cell had filled the Go caches; cold is 72–84 s
on earlier runs) → `--version` = `off-by-one aae20cb` → `seed` **37.84 s**, `files=2856 / answers 2955
created + 1530 skipped / edges 12872` → serve on `:18901` → `/health {"status":"ok","uptime":"0s",
"version":"aae20cb"}` → `discover` `found:true` in **7.2 ms** → `stats` 2856/2955/2955 with
`solver_available:false` (no pi/bwrap on a bare agent — the documented keyless degradation).

Verdict: **install leg PASS**, with the act/chaos-disconnect cells honestly UNVERIFIED and the ui-probe
harness blind spot unchanged.

## Deliberately not re-driven

- The DF-45/46/47/48/49/51/52/53/54 operator and deploy-truth legs (run 19, 2026-10-20) — the artifact under test here is the same code as that HEAD plus two CI/board commits.
- The export/import round-trip (DF-50 / REVIEW-OB-016) — needs a pushable target repo; unchanged by this release.
- The PII/scrub surface (DF-33/34/35) — checked only as "the corpus seeded", not content-audited again.

## How to reproduce

```bash
mkdir -p /tmp/ob1-dogfood && cd /tmp/ob1-dogfood
V=v0.2.0; base=https://github.com/totalwindupflightsystems/off-by-one/releases/download/$V
curl -fsSLO $base/off-by-one-$V-linux-amd64 && curl -fsSLO $base/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS && chmod +x off-by-one-$V-linux-amd64
git clone --depth 1 https://github.com/totalwindupflightsystems/off-by-one
cd off-by-one && ../off-by-one-$V-linux-amd64 seed
../off-by-one-$V-linux-amd64 --port 18801 -load-threshold -1 &
curl -s -X POST http://127.0.0.1:18801/api/v1/problems/discover \
  -H 'Content-Type: application/json' -d '{"problem_class":"python-sdk-e2e-battery"}'
```

## ch:trace

- run: dogfood tick `off-by-one-dogfood-2026-10-23-12-43-25`
- rows: `DF-OFF-BY-ONE-57` … `DF-OFF-BY-ONE-61`
- artifact: v0.2.0 / tag `eabb6741` / asset sha256 `c2638ce9…a013fc`
- evidence: this file, `docs/dogfood/diagnostics.md` §21, `.coding-hermes/dogfood-log.md`
