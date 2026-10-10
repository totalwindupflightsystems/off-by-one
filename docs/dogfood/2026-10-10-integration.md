# Dogfood Integration Report — 2026-10-10 (run #14)

**Angle:** the flagship loop end-to-end as a first-time user — submit a NEW
problem class to the live service and watch the idle-cycle solver produce a
verifiable answer. Prior runs proved submit queueing (2026-10-02) and the
import/export, Muster-consumer, public-deployment, and install surfaces. No
prior run had watched a solve land.

**Promise under test:** "During idle cycles, the lab reproduces the problem in
a sandbox, solves it via Pi Agent, and caches the answer ... when any agent
later hits that problem class, it discovers the pre-verified answer."

## What I did (real use, live service on :8766)

1. Submitted a genuinely new problem class the corpus had never seen:
   `df10-rust-cargo-workspace-feature-unification-20261009` (rust/linux/stable,
   cadence post-debug). Response: `sub_3c5516`, queued position 1, ETA 3m33s.
2. Polled `GET /api/v1/queue/sub_3c5516` every 45s: pending → in_progress
   (`stage: solver_running`) → `complete` (`stage: done`). Wall time
   queued→done ≈ 6m35s (started 00:00:49Z, completed 00:07:24Z).
3. Discovered the new class: `found: true`, answer id 4400, `status: verified`,
   full solution present.
4. **Graded the answer as a user would:** it is excellent. The solve installed
   Rust inside the sandbox, empirically built both failing and fixed workspace
   layouts, proved `resolver = "2"` fixes the build-dep channel but NOT the
   normal-dep channel (a nuance most human answers get wrong), and delivered
   copy-pasteable reproduction + diagnosis commands (`cargo tree -e features -i`).
   Evidence block records model (openrouter/deepseek-v4.1-flash) and timestamp.
5. Resubmitted the same tuple: HTTP 409 `deduplicated`, `existing_solutions: 1`
   — but with an empty `submission_id` (finding DF-37).
6. Probed the README's own example tuple on the live service and on a fresh
   bunker install: `so-nil-pointer-deref` + `version: "1.26.1"` returns
   `found: false` with EMPTY `version_warnings`, while dropping version returns
   `found: true`. The corpus answer for that class carries `version: ""`
   (verified answer id 1601). Finding DF-36 (P1).

## Errors hit and fixes (the trail)

| What happened | What it taught |
|---|---|
| `/tmp/serve.log: Permission denied` on bunker agent | On bunker hosts /tmp is shared across that host's agents and mode-restricted for the agent user — write logs under `$HOME`. (Pre-existing pitfall, confirmed again.) |
| Discover `found:false` on the docs' example | Not an install failure: tuple mismatch. The API gives no signal (version_warnings empty) that a version mismatch was the reason. |
| Board ids DF-33..35 consumed by a sibling lane mid-tick | Re-grepped max id immediately before append; renumbered my rows to 36-38 rather than minting duplicates. My first append was removed and refiled cleanly (git diff shows exactly 4 added lines). |

## Measured numbers (perf, per coding-hermes-perf)

- Discover (headline read op, live service, warm): **10.0 ms ± 2.0 ms**
  (hyperfine, 20 runs, 3 warmups). Sub-ms on the fresh bunker install. No user-
  perceptible slowness — no PERF row.
- Submit → queue: <100 ms. Queue status poll: <10 ms.
- Solve wall time (the one real wait): **6m35s** vs quoted ETA **3m33s** →
  DF-38 (P3). This wait is inherent to solving; the finding is the ETA gap,
  not the duration.
- Install leg (fresh bunker, from-source path verbatim): clone 5s →
  `make build` 41s → `seed` 14s (2577 classes / 2673 answers / 11356 edges) →
  serve + `/health` ok. Total ≈ 60s of user-visible work.

## Verdict

✅ **SHIPPABLE.** The flagship promise — submit a new problem, get a verified,
genuinely good answer back, discover it later — held end-to-end for the first
time in 14 runs, on the live service, with no intervention. The product works.
Open friction is docs-contract drift (DF-36 P1, DF-37 P2) and an honest-ETA
gap (DF-38 P3).
