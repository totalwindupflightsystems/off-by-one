# Dogfood 2026-09-23 — Solve Pipeline, Web-UI Chat, Release-Binary Install

**Tick:** off-by-one-dogfood-2026-09-23-10-59-22 · HEAD 239cbdb · verdict ✅ SHIPPABLE (1 open P1)

**This run's angle:** surfaces none of the 7 prior runs touched — (1) the solve
pipeline submit→queue→bwrap sandbox→Pi Agent→stored answer, (2) the web UI's
`/ws/chat`, (3) the v0.1.1 release binary as a fresh-machine consumer artifact.

## Promise under test

*"A user submits a real problem and gets a verified answer; anyone on a fresh
box can consume the lab without building from source; the web UI is a usable
chat front-end."*

**Held** for the solve pipeline (two real problems solved, 24s / 58s, correct
solutions stored and browsable) and the release-binary install path (58s
end-to-end, sha256 verified, discover green). **Failed** for those users: the
flagship `POST /api/v1/problems/discover` 404s any solved class whose title
merely contains the word "dogfood" or "canary" (DF-OFF-BY-ONE-15).

## What worked (evidence, not vibes)

- Scratch instance from the released v0.1.1-style workflow: seed 2094 classes /
  2184 answers / 8784 edges (host build, 45s; bunker release binary 46s).
- Submit `ob1-dogfood-fib-off-by-one-index` → queued 23ms → cron picked it up
  (20s interval) → real pi-agent in bwrap → complete in 24s. Solution text is
  correct: root-cause (`i <= n` returns F(n+1)), exact fix, executed
  verification block, table test 0..12.
- Submit `ob1-ctx-canary-deploy-oom` → complete in 58s — a genuine goroutine
  leak writeup ("400 MiB retained, 401 goroutines"). Solver quality is real.
- Dedup works: re-submit same tuple → 409 `existing_solutions:1`.
- Web UI serves; `/ws/chat` upgrade 101, system hello immediate, full solution
  frame at t=74s (with no interim feedback — see DF-16).
- Fresh-machine consumer path (first proof): release binary + SHA256SUMS
  download+verify 6s → corpus shallow-clone 3s → seed 46s → serve → discover
  found:true, 9ms warm ×3. Agent destroyed, bunker list clean.

## The P1 (DF-OFF-BY-ONE-15) in one paragraph

`internal/graph/placeholder.go` guards a list of probe-class regexes
(`self-test`, `dogfood`, `canary`, `field-test`, `ds-007`, …). They are
UNANCHORED — `(?i)dogfood` matches any substring of any title. The same
family is applied in SQL (`graph.NotPlaceholderClassSQL`) to the queue list.
Result: a class titled `ob1-dogfood-fib-off-by-one-index` submits fine, solves,
shows up in browse (`GET /problems`, `q=`, class detail, answers) — and then
discover returns 404 and the queue list hides it. Submit, solve, store: all
work. The one endpoint whose whole job is "hand me the pre-verified answer"
refuses to serve it. Control class discovers fine in 10ms. A user hits this
with zero warning; nothing in the API response hints at the cause.

## Numbers (Step 2b)

| operation | warm | cold / first | verdict |
|---|---|---|---|
| submit | 23ms | — | fine |
| discover (control class) | 9-12ms | 12ms first | fine |
| q= search | 32ms | — | fine |
| seed 2094 classes (host build) | — | 45s | fine |
| **release-binary install, fresh Debian** | — | **58s total** | fine — and fast |
| real solve (pi-agent + bwrap) | — | 24s / 58s | fine |
| WS chat answer | — | 74s silence | UX finding (DF-16) |

Nothing here is slow enough for a PERF row — discover/search are single-digit
milliseconds and the fresh install is under a minute. The only user-noticeable
wait is the chat silence, filed as a UX row (DF-16), not a perf row.

## Right way to run this surface (for the next agent)

```bash
# scratch instance with the REAL solver (bwrap + pi-agent + keys present)
./off-by-one seed -db /tmp/dogfood-ob-2026-09-23/lab.db
export DEEPSEEK_API_KEY=... OPENROUTER_API_KEY=...
./off-by-one --port 18766 --db /tmp/dogfood-ob-2026-09-23/lab.db \
  --load-threshold -1 --cron-interval 20s --solve-timeout 300s
# then: POST /api/v1/problems/submit  → poll GET /api/v1/queue/sub_xxx
```

Watch out: a class slug containing `dogfood`/`canary`/`field-test` will solve
but then 404 on discover until DF-OFF-BY-ONE-15 is fixed. Use slugs free of
probe-words when probing discover.