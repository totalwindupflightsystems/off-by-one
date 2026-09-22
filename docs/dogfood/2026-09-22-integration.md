# Dogfood Integration Report — 2026-09-22 (run #8)

**Target:** off-by-one (pre-solve lab), workdir stand-in `releng-lane/off-by-one`
**Angle:** the surfaces no prior run touched — the **flat-file corpus as a
no-server consumer product** (README's "use it directly" promise) and the
**public catalog ob1.it.com** (README's flagship link). Prior runs #1–#7 all
exercised the HTTP API; this run used the repo the way an outside user would.

## The promise under test

README §"Answer Database — Browse & Share": *"The pre-solve lab has verified
answers … published as flat files in this repo so anyone can use them without
running a server"* — fetch one file by URL, shallow-clone, `grep` locally.
Plus: the public catalog at https://ob1.it.com is "searchable" and "synced
from the lab every 6h".

## Real use #1 — corpus solves a live problem on this box (L3 proof)

This host has a genuine, documented problem: `python3`→3.11 while `pip`
resolves to python3.14 (PEP 668 externally-managed). Instead of debugging,
I used off-by-one as a user would:

1. `git clone --depth 1` the public repo — **3s, 68MB**, 2051 class files,
   2141 jsonl lines.
2. Local scan of `data/answers.jsonl` for "externally-managed" — **62ms warm**
   → 3 hits, including `python-pip-interpreter-mismatch-pep668`
   (answer_id 1347, solved by the lab itself 2026-08-20).
3. Read the answer's solution; **applied it verbatim**: `python3.12 -m venv`
   (1s) → `.venv/bin/pip install requests` → clean install,
   **zero PEP 668 error** — `requests 2.34.2 on Python 3.12.13` in the venv.

That is the full value proposition working end-to-end **without ever starting
a server**: a real problem, solved from flat files, in about 2 minutes. The
answer was even about this exact machine, because the lab runs here.

Timings: clone+cold-scan ≈ 4s; warm jsonl scan 62–64ms; single-class grep 5ms.
Comfortably fast; **no PERF row filed** (nothing a user would feel).

## Real use #2 — live lab cross-check (server path still healthy)

- `GET /health` ok, uptime 22h42m; stats 2142 classes / 2335 answers /
  **2307 verified**, hit_rate 0.988, avg_solve_time 3m27s, solver_available.
- `POST discover` on the pep-668 class → `found:true`, 1689-char solution, 10ms.
- Queue fully reconciles: pending 1 / in_progress 1 / complete 2165 /
  failed 2003; `queue_depth` = 2 = pending+in_progress. The list endpoint
  now shows pending rows newest-first with an honest total — **DF-OFF-BY-ONE-10
  (the one open P1 from run #7) is live-verified FIXED** (the `?status=pending`
  filter works too).

## Real use #3 — freshness triangle (numbers, then the root cause)

| Source | Classes | Verified answers | Stamped |
|---|---|---|---|
| Live DB (`/api/v1/stats`) | 2142 | 2307 | live |
| `data/` corpus in repo (COUNTS.md + clone) | 2051 | 2141 | 2026-09-22 16:00 UTC |
| ob1.it.com (static HTML + title/sitemap) | 1062 | 1144 | 2026-08-18 03:11 |

- **Corpus vs live** — reconciles exactly, exporter is faithful:
  2142 live classes − 76 probe/canary classes matching the exporter's
  `EXCLUDED_CLASS_PATTERNS` − 15 classes holding only `failed`-status answers
  (verified in SQLite read-only) = 2051 classes / 2141 answers. No content lost.
  The **real defect is a docs inversion**: README says "the exported corpus is
  the set of verified answers" with stats applying "one further exclusion";
  in truth the corpus is a *strict subset* of the stats-verified set (it
  additionally drops probe classes and failed-solve answers). → DF-OFF-BY-ONE-14 (P2).
- **Catalog vs everything** — ob1.it.com serves the **Aug 18 tree byte-identical**
  (`diff -q` clean on a sampled class page), title/metadata advertise
  1062 classes / 1144 answers, sitemap has 1063 URLs, today's new class 404s.
  **Root cause proven**: `scripts/sync-answers.sh` (the only caller of
  `generate-static-site.py`) has **no active invocation anywhere** — the active
  cron `ob1-distribute.sh` (4×/day, deployed copy md5-identical to the tracked
  one) regenerates `data/` in PART 1 and ships binary+DB in PART 2; the
  merged script's own header claims it absorbed the job that regenerated
  `site/`. → DF-OFF-BY-ONE-12 (P1).
- **Catalog "search" is not search** — the SPA ships zero JS: no `<input>`,
  no `<script>`, no `/api` calls; the "Search the catalog" CTA anchors to a
  static list. A static index is fine; advertising search is not. → folded
  into DF-OFF-BY-ONE-12 acceptance.

## Install leg (ephemeral bunker, las-bunker-03) — PASSED

agent `92f4b025`, bare Debian user, no sudo, destroyed after (list clean).

| Step | Result |
|---|---|
| `git clone` (public HTTPS, the fresh-user path — no credentials) | 15s, HEAD 859081c |
| Go bootstrap per README's no-sudo tarball recipe | ok, go1.26.8 (README pinned 1.26.8 — current) |
| `make build` | rc=0, 17MB binary |
| `./off-by-one seed` | 34s — 2051 classes / 2141 answers / 8510 edges |
| serve `--port 18766` (`OFF_BY_ONE_LOAD_THRESHOLD=-1`) | `/health` ok |
| smoke: `discover` pep-668 class | `found:true` in 10ms |

`install_seconds` ≈ clone 15 + bootstrap ~60 + build ~180 + seed 34 ≈ **~5 min
end-to-end on 4 slow vCPU**; every command worked **verbatim** from the
README. Run #7's install findings (DF-OFF-BY-ONE-11: Go bootstrap recipe,
clone access note) are now fixed in README and re-verified green.

Two bunker-leg frictions that are **not** project bugs but belong in the
diagnostic trail:

- **Shared `/tmp` collision #1 (stale artifact):** the README's Go tarball
  recipe writes `/tmp/go.tgz`; a leftover from the 2026-09-19 run's agent
  (different uid, sticky /tmp) made `curl -o /tmp/go.tgz` die with
  `curl: (23)`. Fix for the recipe: write to `$HOME` (this run worked around
  it with `~/go.tgz`). Filed as part of DF-OFF-BY-ONE-13.
- **Shared `/tmp` collision #2 (concurrent agents):** `sync-answers.sh` and
  the run pattern redirect build/site logs to fixed `/tmp/*.log` paths; a
  sibling fleet agent on the same box overwrote them mid-read (I briefly
  "saw" another project's Rust build). Per-agent paths (`/tmp/<id>-…`) fix
  it. → DF-OFF-BY-ONE-13 (P3).

## Errors hit (verbatim) and what each meant

- `curl: (23) client returned ERROR on write of 1369 bytes` → stale
  cross-agent `/tmp/go.tgz` (sticky /tmp), not disk (124G free), not network.
- `make build` output showing `Compiling hilo_ffi … Finished in 33m 44s` →
  shared `/tmp/build.log` overwritten by a concurrent bunker agent; my own
  per-agent log showed `make_build_rc=0`.
- `GET /api/v1/problems?limit=3000` returning 100 rows → documented page cap
  (api-reference.md documents `offset`; limit clamps to 100, OB-GAP-081).
  My probe's fault, not the API's — no finding.
- `GET /api/v1/problems` list rows carry `title`, not `class_id`/`class` →
  first join-attempt produced a bogus `None` set; correct field is `title`.

## Verdict context

The no-server corpus promise **holds** (and solved a real problem on the
host). The public-catalog promise **does not hold today** (35 days stale,
orphaned regeneration, advertised search doesn't exist). Overall run verdict:
**SHIPPABLE with one open P1** — everything a user touches works; the P1 is
that the showcase mirror is quietly frozen.
