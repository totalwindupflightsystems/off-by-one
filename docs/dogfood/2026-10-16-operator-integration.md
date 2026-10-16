# Dogfood Run 2026-10-16 (run 18) — Operator/Substrate Angle: Idle Gate, Memory Bounds, Fresh-Box Install

**Run:** off-by-one-dogfood tick 2026-10-16T11:33Z. Target HEAD 84c211b6; scratch binaries and the
running unit both resolve to HEAD's code (`make gate-deploy` PASS; on-disk stamp
`v0.1.1-311-g3db0de63`).
**Verdict:** 🟡 PROMISING-BUT-ROUGH for the OPERATOR surface (the lab itself stays SHIPPABLE):
the solvers and the catalog work, but the two limits an operator configures to protect a shared
box — the idle gate and the per-solve memory/timeout caps — are invisible or cause-blind to the
submitter. Three new rows (DF-45/46/47) plus a docs drift (DF-48) and a board-integrity error
(DF-49).
**Promise tested:** "I can run this on a *busy* box: it only solves when the host is idle, it bounds
each solve in memory and time, and I can tell what it is doing and why." (No prior run drove the
idle gate, the memory bounds, or the gate's log/observability surface — 19 runs covered the catalog,
the API, the UI, export/import, privacy, and install.)

## What I did as a real operator (three legs, all on HEAD code)

### Leg A — the idle gate, from the submitter's chair (`--mem-fraction 0.001`, gate tripped)

Submitted a problem and polled it exactly as a submitting agent (or the Muster consumer) would:

```
submit 200 (23.6ms): {"submission_id":"sub_4fcc15","status":"queued","position":1,"estimated_time":"30s"}
queue at t=2/6/12/20s: {"status":"pending","stage":"queued","position":1,"estimated_time":"30s"}   ← identical every time
/metrics same window: {"skipped_host_pressure":11,"last_skip_reason":"memory",
                       "last_host":{"load1":0,"mem_used_fraction":0.368,"reason":"memory"}}
serve.log:            cron: tick error: cron: system is not idle     (×10 in 20s at interval 2s)
```

The daemon knows everything; the per-submission surface says nothing, and the quoted ETA does not
move. Reproduced on a seeded node (2693 classes / 2791 answers) for a full 15s window. With the
LIVE unit's config (`-cron-interval 60s`, avg_solve_time 5m41s) a submitter can be quoted minutes,
wait them, and never learn the box was gated throughout. → **DF-OFF-BY-ONE-45** (P2); the
`cron: tick error:` mislabelling of a normal gate decision is filed separately as **DF-47** (P3).

**Bonus re-proof (DF-OFF-BY-ONE-41 fix):** the submit response and the queue entry BOTH carry the
submit-time tool classification on HEAD —
`"resolved_tools":["jq"],"missing_tools":["parallel"]` for a submission declaring `["jq","parallel"]`
on a host without `parallel`. The row may be closed: the degrade is now visible to the submitting
agent on both surfaces.

### Leg B + Arm 3 — the memory/timeout bounds, as the submitter is told about them

Two arms, two different configured limits firing, two different — and equally useless — reasons:

| Arm | Config | Observed | Submitter sees |
|---|---|---|---|
| B (timeout) | `--solve-mem-mb 128 --solve-timeout 30s`, `OB1_BWRAP_TIMEOUT=60` | wall 30.0s (= the configured solve timeout), `/metrics` peak_rss **0.022 GiB** ≪ 128 MiB cap | `failure_reason: "pi-agent exec: signal: killed (stdout=)"` |
| 3 (cap) | `--solve-mem-mb 32 --solve-timeout 300s` | died in 67ms, peak_rss 0.016 GiB | `failure_reason: "pi-agent exec: exit status 139 (stdout=)"` (SIGSEGV) |

Neither string names the limit, its configured value, or the knob to change — even though the daemon
logs `cron loop started: … solve-mem-mb=32 reject-rss=25 MiB` at startup and tracks
killed/peak/alert-threshold in `/metrics`. The docs put the disambiguation on the operator
(docs/api-reference.md "Solve memory bounds": *"verify the configured value before treating it as a
solver failure"*). Emitter: `internal/solver/piagent.go:272`. → **DF-OFF-BY-ONE-46** (P2).

**Fail-closed HELD in both arms:** `/api/v1/stats` shows `total_answers 0`, `total_problems 0` on both
scratch nodes — a killed or capped solve commits nothing. The defect is attribution, not integrity.

### Install leg — fresh box, README Quick Start verbatim (PASSED)

ephemeral bunker **bunker-mvp** (las-01/02/03/04 all offline or tailscale-stale), agent `oby-df1016`,
fresh box had **no toolchain** (go1.22.2, make, git only):

```
clone     13s rc=0   (HEAD 84c211b6 == local HEAD)
cp .env.example .env  rc=0
make build 72s rc=0  → stamp v0.1.1-316-g84c211b6
seed      28s rc=0   → 2693 classes / 2791 answers / 12018 edges
serve (agent-unique port 18766)  health {"uptime":"3s"}  (seconds = our own server)
submit, no solver keys → 503 solver_unavailable with the documented remediation message  ✓ never a silent accept
discover (headline, cold) 23ms found:true (so-nil-pointer-deref, answer 57); warm 22ms
serve killed before destroy; destroy rc=0; re-checked >90s: agent + key gone, list clean
```

113s of install vs the README's quoted "~114s when building from source" — the documented number is
honest. The one wrinkle is docs-side and filed as **DF-48**: the README's no-sudo recipe installs
go1.26.8 while `go.mod` now requires `go 1.26.9` (drift from c17711cb, which touched go.mod only);
run-verified with `GOTOOLCHAIN=go1.26.8 go build ./...` →
`go: go.mod requires go >= 1.26.9 (running go 1.26.8; …)`.

**Board-integrity side-find (DF-49):** the pre-write `boardctl validate` the dogfood skill mandates
returns `RESULT: FAIL (1 error)` on a snapshot taken *before* this run wrote anything —
`events.jsonl line 427: event id 424 descends below earlier id 1425`. Same result after the writes
(no new errors from this run: 220 → 226 rows, id set diff = exactly the 6 rows below, no duplicates).

## Perf (Step 2b) — nothing fileable

| Operation | Warm | Cold (first call after restart) |
|---|---|---|
| `POST /api/v1/problems/discover` (headline payoff) | 13.0 ms ± 5.7 (n=20, includes curl start) | 4.8 ms first call, 2.6 ms second |
| `GET /metrics` | 14.2 ms ± 6.8 (n=20) | — |
| `GET /api/v1/problems?q=nil+pointer&limit=50` (FTS over 2691 classes) | 52.1 ms ± 7.1 (n=10) | — |
| `POST /api/v1/problems/submit` | 6.6–23.6 ms (n=2) | — |
| `GET /api/v1/taxonomy` | 212 ms, 9.3 MB payload (shape issue, already DF-21) | — |
| gate decision (skip path) | bounded by the cron interval, not compute — 11 skips in 20 s at interval 2 s | — |

No PERF row: nothing here is slow enough that a user would feel it (the same conclusion as runs
14–17 on the same operations). The one real wait in this run's territory is structural, not
compute-bound: a gated submission waits for the host, and the wait has no representation in the API
(DF-45) — that is the correctness/UX row, not a perf row.

## Value judgment

- **Does it work?** Yes, and the invariants hold under abuse: gated box → nothing runs; capped solve →
  nothing committed; keyless box → honest 503; killed solve → nothing stored. The integrity ladder
  survived every leg.
- **Is it useful?** Yes — the idle-gate + memory-bound design is exactly what makes the lab safe to
  co-host, and the fresh-box install is a one-command, sub-2-minute path.
- **Is it usable?** For the *operator*, no, not yet: the two knobs you set to protect the box are the
  two whose effects the submitter cannot see (DF-45) or cannot attribute (DF-46), and the log calls
  the gate's normal resting state an error (DF-47).
- **Is it trustworthy?** Yes: no bad state, no leaked work, install/telemetry numbers honest.
- **Is it fast enough?** Comfortably (table above).

## What this run did NOT re-drive (stated so the next lane doesn't assume coverage)

- The web-UI fixes for DF-42/43/44 (chat banners, 409-as-failure, Explore version node) were NOT
  re-driven in a browser this run — they landed after run 17 and remain unverified by dogfood.
- DF-33/34/35 (privacy/export) and the Muster consumer path were not re-tested; only their install-side
  neighbours (release path, port hygiene) were touched.

## Artifacts

- Board rows: **DF-OFF-BY-ONE-45** (gate deferral invisible), **46** (cause-blind kill reason),
  **47** (gate skip logged as tick error), **48** (README toolchain below go.mod minimum),
  **49** (board validator FAIL at HEAD), **INSTALL-OFF-BY-ONE-2026-10-16** (install leg PASS).
- `docs/dogfood/diagnostics.md` §19 — how the gate/bounds surfaces compose and the right way to drive
  them; `.coding-hermes/dogfood-log.md` — run 18 entry; `skills/off-by-one-usage/SKILL.md` v1.12.0 —
  pitfalls 37–39 for consumers.
- Scratch evidence on the control host: `/home/kara/oby-dogfood-1016/{A,B,C,perf}/evidence.txt`,
  `install-payload.sh`, and the pre-write board snapshots in `…/snap/`.

## Phase 2 (same tick) — the fleet fresh-system QA battery, and a cap-tolerance defect (DF-51)

The tick's install leg was also driven through the fleet harness the lane prompt names:
`bunker-qa.sh launch /home/kara/off-by-one` → agent `913faf73` on bunker-mvp (TTL 4h), battery against
**HEAD 31f49319** (this run's own commit), then `collect --evidence
/home/kara/oby-dogfood-1016/qa/evidence.jsonl` (20 cells, agent auto-destroyed).

Green: repo-head/sync, toolchain-bootstrap (Go + node + make bootstrapped into `$HOME`), **fresh-install**,
**ci-pass — native suite PASS**, upgrade (previous release tree → HEAD reinstall clean), chaos-errorpath
(clean error on missing config, rc=2). Not applicable: docker-deploy / chaos-shutdown (no compose file),
chaos-corruption (no db/state files in the repo). Note `ui-probe` graded "no web UI detected" although the
repo serves an embedded SPA — a harness-detection gap, reported here rather than filed (QA lane owns the
probe).

**Red, and reproduced: `chaos-resource` rc=1 — `TestApplyMemLimitMB_RoundTrip` FAIL in
`internal/sandbox`.** The cell ran the suite under a 3 GiB RLIMIT_AS. Local control proves it is
environment-driven, not product-driven:

```
$ go test ./internal/sandbox/ -run TestApplyMemLimitMB_RoundTrip -count=1     → ok 0.004s
$ bash -c 'ulimit -v 3145728; go test ./internal/sandbox/ -run TestApplyMemLimitMB_RoundTrip -count=1'
    memcap_test.go:190: roundtrip helper failed: exit status 1
        ApplyMemLimitMB(4096): sandbox: setrlimit RLIMIT_AS=4096 MiB: operation not permitted   → rc=1
```

The helper asks for 4096 MiB while an inherited hard cap of 3072 MiB is in force; setrlimit above an
inherited hard limit is EPERM. Filed as **DF-OFF-BY-ONE-51** (fix direction: derive the probe from the
ambient hard limit and skip with a reason). Worth a row because containers, CI runners, this fleet's QA
battery and the Hermes terminal sandbox all carry ambient AS caps, so the documented `go test ./...`
turns red for an environment reason in exactly the shapes this project targets.

**Deliberately NOT filed:** the battery's `ci-act` cell exited rc=1 with a truncated act log. The
harness's own rule is that "a failed act leg is never graded OK; hosted CI is authoritative", and the
full log lived on the now-destroyed agent — so the leg is recorded here as **unattributed**, not as a
defect claim.

**Sibling thread (same tick, same board):** another executor of this tick landed a follow-up commit
(192e398b) filing **DF-OFF-BY-ONE-50** — export pushes not atomic (a stranded commit silently skipped
forever) — and re-proving the REVIEW-OB-014 import precondition live. Both threads are credited to run
18; DF-50 and DF-51 are distinct findings.

