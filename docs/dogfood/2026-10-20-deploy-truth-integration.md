# Dogfood run 19 — 2026-10-20 — "is HEAD what is running, and does the SPA tell the truth?"

Run type: skill-driven dogfood tick (`coding-hermes-dogfood`), executor = the lane itself
(the retired `dogfood-scheduler-tick.sh` / `dogfooding.ts` driver was NOT used).
Target: `off-by-one` — workdir `/home/kara/.hermes/stand-in/dogfood/off-by-one`,
control checkout `/home/kara/off-by-one`.
Control HEAD at run time: `fda8b3a288f29ac8441b267ea945eb81c9ad9e17` (2026-10-20 05:15 -05).
Executed on: `karaHermes-mde-7840hs` — **note the host was heavily loaded** (`/proc/loadavg`
11–20 on a 16-core box for the whole run), which is itself part of the evidence (see finding 1).

## Verdict

🟡 **PROMISING-BUT-ROUGH for the DEPLOYMENT / TRUTH surface** (the engine itself stays SHIPPABLE:
discover 0.68 ms warm, dedup, gate deferral, WS upgrade all verified working on a HEAD build).

The product promises two things in its own README and unit files: the lab runs *during idle
cycles* rather than competing with the host, and the verified-answer catalog is what agents
consume. Both hold **in the repo's code** — and neither holds **in the deployment that is actually
serving `:8766`**, which no gate in the repo checks.

## Promise (null hypothesis)

> "A user can run this lab on a busy box: it solves only during idle cycles, bounds every solve,
> and hands an agent a pre-verified answer instead of a fresh debug."

## What was actually used (real use, not tests)

| Surface | How it was driven | Result |
|---|---|---|
| Live daemon `:8766` | `systemctl cat/status`, `/proc/<pid>/exe`, `strings /proc/<pid>/exe`, `/metrics`, `/api/v1/stats`, journal | **running image ≠ HEAD** and **running flags ≠ vendored unit** (findings 1–2) |
| Scratch HEAD daemon `:18801` | built `go build -o /tmp/oby-run19/oby ./cmd/off-by-one`, seeded from `data/` (2785 classes / 2883 answers / 12546 edges, 68.7 s cold on the loaded host) | gate deferral, dedup, taxonomy truncation all reproduced |
| HTTP API as a submitter | `POST /problems/submit` (3 arms), `GET /queue/{id}` polled at t=2/6/12/20 s, `POST /problems/discover` ×20, `/metrics`, `/api/v1/taxonomy` ×3 | findings 3–5 |
| Web SPA in a real browser | Explore drill-down to an answer, Submit twice (queued + dedup), chat question, passive 200 s banner watch, WS upgrade timing | 1000/2785 classes rendered; DF-42/43/44 fixes **verified held** |
| Fresh machine (install leg) | `bunker-qa.sh launch/collect` → agent `b1167809` on `bunker-mvp` at HEAD `fda8b3a2`, 20 cells | install PASS (see below) |

## Findings (rows filed this run)

| Row | P | One line |
|---|---|---|
| `DF-OFF-BY-ONE-29-R2` | P1 | The **running** unit still ships the pre-fix `ExecStart`: `-load-threshold -1` (idle gate off) and no `-solve-mem-mb`; measured at `loadavg(1) = 20.01` while solves ran back-to-back and `skipped_host_pressure` stayed `0`. Recurrence of DF-29; DF-30's residual, recorded but never worked. |
| `DF-OFF-BY-ONE-52` | P2 | `make gate-deploy` has **no caller** anywhere (CI runs only the hermetic `check-deploy-test`), so the tick-442 "rebuild **+ redeploy**" claim was unfalsifiable: `/proc/2852325/exe → (deleted)`, running stamp `v0.1.1-367-g69472a2a` vs on-disk `v0.1.1-370-ga8e8014f`, and `gate-deploy` returned rc=1 for 4 h 45 m. |
| `DF-OFF-BY-ONE-37-R2` | P2 | The dedup 409 returns `submission_id: ""` for the **answer-corpus arm** (the queue arm is fine) — the documented "poll the existing submission" contract is unsatisfiable there. |
| `DF-OFF-BY-ONE-53` | P2 | `GET /api/v1/taxonomy` still truncates at 1000 classes (9.8 MB payload, 2785-class lab, no total/notice) and the Explore UI renders 1000 rows under "Browse the full **problem-class tree**". The 2026-09-25 finding (logged as `DF-OFF-BY-ONE-21`) has never existed on the board — 0 rows. |
| `DF-OFF-BY-ONE-54` | P2 | The Submit form's fixed env vocabulary (Any/dev/staging/prod/ci/local) cannot express the corpus/docs values (`docker`, `linux`), so the documented "already answered → 409" dedup is unreachable from the SPA: the UI queues a fresh solve of an answered class while telling the user "1 existing solution found." |
| `QA-OFF-BY-ONE-20-R2` | P3 | The fresh-system battery still grades `ui-probe` **N/A "no web UI detected"** for a build that serves an SPA this run drove end-to-end in a browser. |
| `DF-OFF-BY-ONE-55` | P3 | The **board-lint pre-commit hook is inert in this checkout**: `git config core.hooksPath = /home/kara/.hermes/hooks` and `git rev-parse --git-path hooks/pre-commit` resolves there, so the `.git/hooks/pre-commit` that `boardctl install` wrote (the duplicate-id / dangling-dep gate the board's own `BT-053-ROLLOUT` row reports as installed) never runs. Found while landing this run's own commit. Fix owner: boardctl's install path. |
| `INSTALL-OFF-BY-ONE-1` | P3 | Install leg PASS at HEAD `fda8b3a2` (details below). |

All eight rows were appended with `board_next_id.py`-minted ids and the board still validates
(`boardctl validate` → `RESULT: OK`, 280 warnings, 0 errors).

## Re-proofs on this run's HEAD (fixed regressions re-run, not just claimed)

* **DF-OFF-BY-ONE-45 — host-pressure-gate deferral is now visible to the submitter. FIXED.**
  Scratch instance `-load-threshold 4` (host load 11–21):
  `GET /api/v1/queue/sub_7fe5c6` → `{"status":"deferred","stage":"gated","position":1,
  "gate_reason":"load","estimated_time":""}` at t=2/6/12/20 s — byte-stable, no fixed ETA.
  `/metrics` → `"skipped_host_pressure":23, "last_skip_reason":"load"`.
* **DF-OFF-BY-ONE-47 — the normal skip no longer logs as an error. FIXED.**
  Journal instead shows `cron: deferred — host busy (load): used 12.230 / threshold 4.000`.
* **DF-OFF-BY-ONE-42 — Explore version node renders its children. FIXED.**
  Browser drill: class → env `docker` → lang `go` → version `latest` → answer rows; clicking the
  answer fills the detail pane (4285 chars, 5 `<pre>` with 50 syntax-highlighted `<code>` blocks).
* **DF-OFF-BY-ONE-43 — a 409 dedup renders as success. FIXED.**
  Second SPA submit of the same tuple: panel shows `DEDUPLICATED … Already queued (sub_fd245d),
  position 1`, wire body `409 {"status":"deduplicated","submission_id":"sub_fd245d"}`.
* **DF-OFF-BY-ONE-44 — chat banner pile-up. FIXED.** One in-place status line: a 200 s passive
  watch on an open panel produced exactly **1** `.chat-msg-system` element; the user message is
  echoed (no dropped sent message); reconnect does not append banners.
* **DF-OFF-BY-ONE-26 — WebSocket upgrade latency. FIXED.** `/ws/chat` upgrade measured **1 ms** and
  **10 ms** (was 25 s in run 13).
* **DF-OFF-BY-ONE-51 — the memcap round-trip test is cap-tolerant. FIXED.** The battery's
  `chaos-resource` cell (3 GiB ambient cap — the exact shape that graded rc=1 on 2026-10-16) now
  grades **PASS**: "suite survives 3G memory cap (cap=3145728KB child_saw=3145728)".
* **DF-OFF-BY-ONE-37 (queue arm) — holds**: the queue-arm 409 carries the real `submission_id`.
  Its **corpus arm** does not — that is `DF-OFF-BY-ONE-37-R2`.

## Step 2b — what using it costs (measured, nothing filed)

Measured on the scratch HEAD instance unless noted; the control host was loaded (11–20), so these
are conservative numbers.

| Operation | Command | Warm | Cold / notes |
|---|---|---|---|
| discover (headline) | `POST /api/v1/problems/discover` ×20 | 0.68 ms p50 (min 0.60 / mean 0.79 / max 2.56) | 1.4 ms first call after start — **nothing a user would feel** |
| submit | `POST /api/v1/problems/submit` | 0.4–58 ms | includes dedup/related-problem lookup |
| taxonomy | `GET /api/v1/taxonomy` ×3 | 0.15–0.18 s for **9.78 MB** | not a PERF row: it is DF-21/53's *shape* problem (1000 of 2785 classes in 9.8 MB) |
| Explore first render | browser, tab click → 1000 rows | 0.54 s | 1003 expansion containers |
| WS chat upgrade | `new WebSocket('/ws/chat')` | 1 ms / 10 ms | was 25 s (DF-26, fixed) |
| chat answer | one question | n/a — **not measured**: on the scratch box the runner failed for a missing key (see below) |
| seed (fresh install) | `oby seed -dir data -db scratch.db` | — | 68.7 s for 2785 classes/2883 answers/12546 edges at loadavg 12–20 (prior runs: 23–36 s idle) |

**No PERF row is filed**: the only number a user would feel is the 9.8 MB taxonomy payload, and
that is already carried by `DF-OFF-BY-ONE-53`. Everything else is sub-100 ms on a loaded box.

## Install leg (fresh machine, at HEAD `fda8b3a2`)

Driver: `~/.hermes/scripts/bunker-qa.sh launch` → `collect` (per the tick contract; the retired
dagger driver was not used).

```
launch : agent=b1167809 @ bunker-mvp (tailscale), QA HEAD fda8b3a288f29ac8441b267ea945eb81c9ad9e17, ttl 4h
collect: "pulled 20 evidence row(s), merge-deduped by ts; temp agent destroyed"
cells  : OK 16 / PASS 1 / N/A 4 / UNVERIFIED 2
```

Green and load-bearing: `repo-head` (HEAD fda8b3a2, ff-only), `toolchain-bootstrap` (go 1.26.5 into
`~/tools` + gcc/g++ wrappers + make + compose/buildx + node v22.15.0), **`fresh-install` OK**,
**`ci-pass` native suite PASS**, **`upgrade` v0.1.0 → HEAD reinstall clean**, `chaos-errorpath` OK
(rc=2 clean error on missing config), `chaos-resource` PASS (see DF-51 above).
N/A: `docker-deploy`, `chaos-shutdown` (no compose file), `chaos-corruption` (no db/state in repo),
`ui-probe` (**false N/A — filed as QA-OFF-BY-ONE-20-R2**).
UNVERIFIED and deliberately **not** filed: `ci-act` rc=1 with a truncated act log (the harness's own
rule is that hosted CI is authoritative) and `chaos-disconnect` (disc.log 0 bytes — a vacuous cell;
this repo's native suite has no network path for the cut to exercise). Both shapes are already
carried by existing harness rows (`QA-OFF-BY-ONE-*`); re-filing them from a dogfood run would add no
information.

## Not-filed observations (recorded so the next tick does not re-chase them)

* **Chat failure is cause-blind.** On the scratch box (no API keys) the panel rendered
  "Agent encountered an error. Please try again." in <5 s while the server logged the real cause
  (`chat: agent runner error: pi-agent exec: exit status 2 (stdout=`) and had already warned at
  startup (`WARNING: DEEPSEEK_API_KEY is empty or placeholder`). **Attribution: the underlying
  failure was this leg's missing key, not a product defect** — the finding is only the silence, and
  it is the same class DF-46/47 closed for the solve path (the submit path, by contrast, fails
  closed with `503 solver_unavailable` + remediation). Not filed: the repro needs an unkeyed box,
  and the class is already documented. Worth a row if a second instance is seen on a keyed node.
* `/proc/<pid>/exe` deleted-after-rebuild is a *reliable* staleness tell; `make check-binary-fresh`
  returns rc=0 in exactly that state (it only inspects the artifact on disk). Use
  `make check-deploy`, not `check-binary-fresh`, when the question is "is the service current?".
* `cron: deferral stamp: peek queue head failed: queue: not found (continuing)` prints every tick
  on an **empty** queue (scratch only; `journalctl -u off-by-one | grep -c 'peek queue head failed'`
  over 6 h on the live service = **0**), so it was not filed.
* cgroup accounting sanity check (so a future run does not misread it): the live `off-by-one.service`
  cgroup shows `Memory: 5.8G` while `/metrics.peak_rss_bytes_max` says 75 MB — that is **not** an
  under-report: `memory.stat` is `anon 11.5 MB / file 5.9 GB`, i.e. page cache from the sandbox's
  file reads, not process RSS.

## What this run leaves behind

* 7 board rows (table above), all `pending`, ids minted by `board_next_id.py`, board validates.
* This report.
* `docs/dogfood/diagnostics.md` §20 — the deploy-truth diagnostic trail and the commands that
  discriminate "the repo has it" from "the service runs it".
* `skills/off-by-one-usage/SKILL.md` — pitfalls 40–42 (running-vs-repo config, the corpus-arm 409,
  taxonomy truncation) for the next agent that lands in this repo.
* `.coding-hermes/dogfood-log.md` — run 19 entry with the verdict.

No code was changed. No repo visibility or permissions were touched, no credentials were minted;
the install leg ran on an ephemeral bunker agent with existing access and was destroyed.

## ch:trace

| | |
|---|---|
| row | `DF-OFF-BY-ONE-29-R2`, `DF-OFF-BY-ONE-52`, `DF-OFF-BY-ONE-37-R2`, `DF-OFF-BY-ONE-53`, `DF-OFF-BY-ONE-54`, `QA-OFF-BY-ONE-20-R2`, `DF-OFF-BY-ONE-55`, `INSTALL-OFF-BY-ONE-1` |
| spec | `deploy/off-by-one.service`, `docs/api-reference.md` (submit/dedup), `README.md` (taxonomy/catalog) |
| procedure | manual dogfood run 19 (commands inline above); install leg via `bunker-qa.sh` |
| evidence | `docs/dogfood/2026-10-20-deploy-truth-integration.md`, `/tmp/bunker-qa-evidence-20261020T105356Z-822880.jsonl`, `docs/dogfood/diagnostics.md` §20 |
| witness | `live:systemctl+proc+metrics`, `live:http-409`, `live:browser`, `bunker-qa` at HEAD `fda8b3a2` |
| verdict | FAIL for findings 1–6; PASS for the install leg |
| commit | control HEAD `fda8b3a2` (pre-run); this report's commit on `master` |
