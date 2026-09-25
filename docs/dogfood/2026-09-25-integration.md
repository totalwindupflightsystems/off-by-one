# Dogfood Integration Report — off-by-one, 2026-09-25

Run: off-by-one-dogfood tick 2026-09-25 02:20 UTC. 11th dogfood run; first run to
use the **public deployment (ob1.it.com) as the consumer endpoint** — runs #1-10
always started a local server. Verdict: **SHIPPABLE**.

## Angle

Prior runs covered: CLI/API on local instances (1-8), export/import (2), corpus
no-server path + public catalog freshness (9), Muster bridge consumer (10). This
run took the surface none of them touched end-to-end: **the public internet
deployment as an agent consumer sees it** — Cloudflare Worker bot/browser split,
public REST reads, SPA deep-link rendering — plus live-verification of the two
fixes landed yesterday (DF-12 site regeneration, DF-18 connect-muster port).

## What a real agent user does over public HTTPS — and what happened

1. **Bot fetch** (UA `GPTBot/1.0`): `GET /` → static index advertising
   "2189 verified answers" — matches ground truth (2189 class pages tracked in
   git, 2190 sitemap locs, generated 2026-09-24 17:48). The DF-12 fix is
   **live on the public site**: no more frozen 1062/1144 numbers.
2. **Crawler consistency**: sitemap (2189 class locs) == deployed class pages
   (2189) == git tree (2189). robots.txt + sitemap served. One-time transient
   404s from raw.githubusercontent (origin) resolved on retry — worker retries
   would be nice-to-have, not filed (unreproducible).
3. **Browser fetch**: `GET /` passes through to the live app SPA; the proxied
   readonly API answers (`/api/v1/stats` → 200, `readonly:true`).
4. **Deep-link render**: `https://ob1.it.com/classes/0416-docker-compose-host-port-default-drift`
   (extension-less) under a browser UA → SPA shell → headless Chrome dump shows
   the full answer body rendered client-side, including syntax-highlighted
   yaml/json/bash blocks. Under a bot UA the same URL serves the static page
   (worker mapPath handles extension-less paths). Shell-200 ≠ rendered — this
   one renders.
5. **Full consumer loop**: `/openapi.json` (17.4KB) →
   `POST /api/v1/problems/discover` `{"problem_class":"python-pip-interpreter-mismatch-pep668"}`
   → **found:true, answer 1347, 0.51s** over public HTTPS, 2 related classes
   attached. One miss on the way: sending `{"title": ...}` (a natural guess)
   → 400 `unknown field "title"`; the schema requires `problem_class` and the
   400 does not say so. Minor; the error name (`invalid_request`) is at least
   honest.
6. **Search**: `GET /api/v1/problems?q=pep668` → exact class, public.
7. **Catalog walk**: `GET /api/v1/taxonomy` → **the one real defect of this
   run** (DF-OFF-BY-ONE-21): tree contains 1000 classes / 1011 answers while
   stats reports 2283/2478 and the site advertises 2189. `handlers.go:681`
   hardcodes `ListProblemClassesWithCounts(ctx, 1000, 0)` — no pagination, no
   total field, no docs. A consumer walking the catalog silently sees 55% of
   the corpus. Reproduced locally (identical 1000/1011). Also 8MB JSON /
   ~1.3s public (N+1 ListAnswers per class).

## Live-verified fixes (board rows from 2026-09-24)

- **DF-12** (site frozen since 2026-08-18): site/ regenerated in-repo 09-24
  17:43-17:48, pushed, and the public worker serves the fresh content within a
  day. Numbers on the public landing page match the live DB to within one sync
  cycle. CLOSED in effect.
- **DF-18** (connect-muster.sh hardcoded ports): script now derives PORT from
  `SERVER_URL`/`OFF_BY_ONE_URL` (explicit `:<port>` wins), guards against
  spawning a daemon when the URL points at a remote host, and
  `MUSTER_HEALTH_URL` is overridable. `--check-only` green against the live
  server. CLOSED in effect.
- **DF-17** partially unblocked (the binary question remains open — not
  retested this run; row stays pending).

## Install leg (ephemeral bunker, las-bunker-03)

Documented release-binary path, verbatim from README §"Install from a release":

| step | result |
|---|---|
| spawn agent (TTL 2h) | ea8d84b3 |
| clone (public HTTPS, depth 1) | 20.7s total incl. connect, HEAD 3722faa |
| download + sha256 verify | 8.6s, `off-by-one-v0.1.1-linux-amd64: OK` |
| seed | **36s** — files=2189; classes=2189/0 existing; answers=2281/0 skipped; edges=9280 |
| serve `-port 8766` | health 200; cron loop started; OpenAPI sha printed |
| discover smoke | found:true, **10ms** avg ×3 |
| destroy | exit 0, agent gone from list |

Zero toolchain, zero sudo, zero docs deviation. Fresh machine → working catalog
in ~65s. (Bunker quirk worth knowing: `/tmp` is shared across that host's
agents — a stale sibling file made my first `> /tmp/seed.log` fail with EACCES;
use `$HOME` for scratch on bunker agents.)

## Perf (Step 2b)

| operation | warm | note |
|---|---|---|
| public discover (HTTPS, CF edge) | 646ms ±376ms (n=10) | 0.51s single-shot; RTT-dominated |
| public bot class page | 902ms ±678ms (n=10) | CF + raw origin fetch |
| local discover (fresh bunker instance) | 10ms | |
| public taxonomy | 1.30s ±0.24s, 8MB | the only slow read; symptom of DF-21 |

Sub-second for everything a user does per-answer; nothing here justifies a
profile. The taxonomy 8MB/1.3s is a shape problem, not a tuning problem — it is
part of DF-21, not a separate PERF row.

## Verdict

**SHIPPABLE.** The public catalog promise (the thing that was broken a week
ago) now holds end-to-end: fresh content, consistent numbers, working bot
split, working deep links, working consumer loop. One new P2 (DF-21 taxonomy
cap) — real but narrow. Board: DF-21 pending; DF-12/DF-18 verified fixed.
