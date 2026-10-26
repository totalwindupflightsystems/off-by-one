# Off-by-One dogfood run — 2026-10-26 current HEAD

Verdict: PROMISING-BUT-ROUGH for the delivery/install verification surface; the core catalog workflow works at HEAD 27898c45, but the generic install battery is not attributable to this Go product and the install leg exposed harness/port collisions.

Promise under test: a user can build the Go binary, seed a fresh SQLite catalog, start the HTTP service, discover a verified answer, inspect stats/search/taxonomy, and receive honest errors for unknown classes and unavailable solving.

Target identity: /home/kara/off-by-one, HEAD 27898c4564c8e2a7dea07c962daf85e45ce18f58, branch master, origin/master matched at the start and the worktree was clean. The stand-in /home/kara/.hermes/stand-in/dogfood/off-by-one resolves to this checkout's .coding-hermes/board. Existing work at HEAD includes 27605bb0 (transactional seed) and 27898c45 (latest board-only commit).

Real-use path

1. `make build` succeeded. `./off-by-one --version` returned `off-by-one v0.2.0-66-g27898c45`.
2. Fresh isolated database seed: `./off-by-one seed -db /tmp/.../cold.db` returned rc=0 and `files=2900; classes=2900 created; answers=4529 created; edges=13080`, wall 4.91s (real 4.91, user 3.85, sys 0.46).
3. Idempotent repeat against the same database returned rc=0, `classes=0 created / 2900 existing; answers=0 created / 4529 skipped; edges=0`, wall 0.54s.
4. Scratch server on port 18848 with the isolated database started successfully. `/health` returned `status=ok`, version `v0.2.0-66-g27898c45`. `/stats` returned 2,900 problems, 4,529 answers, 4,529 verified answers, queue depth 0, hit rate 1, solver_available false.
5. `POST /api/v1/problems/discover` for `so-nil-pointer-deref` returned `found:true` and a complete verified answer. An unknown class returned HTTP 404. A submit with invalid cadence returned HTTP 503 solver_unavailable because the scratch instance had no bwrap/pi-agent, with an actionable message directing the user to discover existing answers. Search and paged taxonomy also returned data.
6. The scratch server was stopped and confirmed stopped. No product database or credentials were touched.

Performance

The current seed path is no longer the historical PERF-OFF-BY-ONE-1 case: current HEAD measured 4.91s cold and 0.54s idempotent warm on ext4 scratch storage for 2,900 files and 4,529 answers. The transaction opens at internal/seed/seed.go:146 and commits at :161. This is below a user-noticeable threshold for a one-time local catalog setup; no new PERF row is warranted. The old PERF-OFF-BY-ONE-1 row is correctly historical evidence for the released pre-transaction loader, not a current regression.

Ephemeral bunker install leg

`bunker-qa.sh launch /home/kara/off-by-one` ran on bunker-mvp with agent 268bbe39 at the exact target HEAD, then `collect` returned `COLLECTED ... rows=20` and the agent was destroyed. This is not a silent pass.

Evidence: `/tmp/bunker-qa-evidence-off-by-one-20261026-1548-df.jsonl` (23 JSONL rows including collect; 20 product/battery rows) and `/tmp/bunker-qa-evidence-off-by-one-20261026-1548-df.jsonl.chaos-disconnect.log` (1,386 bytes).

Relevant cells: repo-head OK; native ci-pass OK; upgrade OK; chaos-resource PASS; chaos-disconnect INFO with a real network-cut log; ci-act UNVERIFIED because the agent lacked `catthehacker/ubuntu:act-latest`; fresh-install FAIL is harness-shaped (`install rc=0 but no artifact found (toolsd in PATH...)`) and does not name this repo's `make build`/`./off-by-one` artifact; ui-probe INFO because the generic probe attempted the shared default port 8766 and got address already in use; chaos-corruption INFO/UNVERIFIED because its start probe selected `oby-memcap`, not the daemon. The agent was destroyed by collect and the subsequent list showed the run-owned agent absent.

The fresh-install, ui-probe and chaos-corruption results are therefore partial battery evidence, not product defects. A direct product install arm on a fresh agent would be needed to close that proof; this tick does not call the battery green.

Value judgment

The core catalog value is real and directly usable: build, seed, serve, discover and inspect stats all worked, and keyless operation fails closed with a clear solver-unavailable response rather than queueing work that cannot run. The current seed optimization materially improves first-use cost. Usability remains rough at the install-verification boundary because the generic QA battery targets the wrong artifact name and default port, and its failed local-act leg is environmental rather than a product verdict.

No new board row was filed: the observed product behavior is either working, already covered by existing rows, or harness evidence. The pending QA-OFF-BY-ONE-RELENG-1/2 rows already cover the releng collection class; this run's bunker collection completed, but the battery's product-specific gaps should be handled by the QA/harness owner rather than duplicated here.

Artifacts left in the repo: this integration report, the updated `skills/off-by-one-usage/SKILL.md` v1.15.0 with current-HEAD seed timing and battery-attribution guidance, and the append-only dogfood log entry. No source code, board rows, or scheduler state was changed.

ch:trace
run: off-by-one-dogfood-2026-10-26-15-48-08
repo-head: 27898c4564c8e2a7dea07c962daf85e45ce18f58
witness: live:local-build+scratch-api+bunker-qa
verdict: PROMISING-BUT-ROUGH
install: PARTIAL battery; collected agent 268bbe39, fresh-install cell not attributable
performance: current seed 4.91s cold / 0.54s warm; no new PERF row
