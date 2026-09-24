# Off-by-One Dogfood Integration — 2026-09-24 (run #10)

**Angle:** the surface no prior run touched — the *consumer side of the Muster MCP bridge*
(README core loop step 1: "Agents push problems via Muster API/MCP/CLI"). Runs #1–9 drove the
REST API with curl/python; this run acted as the **agent that Muster exists for**: connected a
real MCP-capable client (MusterFlow) to the live OpenAPI spec, drove the lab through the
generated typed CLI and raw MCP `tools/call`, and re-ran the ephemeral-bunker install leg
verifying the bridge files exist on a fresh clone.

**Target:** live fleet instance on :8766 (HEAD 474a86e, 2247 classes / 2442 answers /
hit_rate 0.988) — read-mostly probes plus two scratch submissions.

## What was done (as a real consumer)

1. `musterflow connect http://localhost:8766/openapi.json --base-url http://localhost:8766`
   → 15 endpoints auto-generated into a typed CLI (`musterflow Off-by-One problems list-problems`,
   `discovery discover-solution ...`) and an HTTP MCP endpoint (`/mcp`, 15 tools).
2. Full read surface over the generated CLI: stats, taxonomy tree, class detail, answers,
   discover, related, queue list + status.
3. Real write flow over MCP: `tools/call listProblems q="pep 668"` → found the PEP 668 class
   that solved a real host problem on 09-22 (answer 1347); `submitProblem` → `queued,
   position 1, sub_ea3d97` (CLI) and `sub_b0b4bc` (MCP) — both later visible in `list-queue`.
4. Error paths: discover 404 message, submit with bad cadence (CLI enumerates allowed values
   in `--help` because the OpenAPI spec carries them).
5. Bunker install leg (below) re-verified the bridge files on a fresh clone.

## What worked (evidence)

- **OpenAPI → tools chain is real.** One command produced a usable CLI and a working MCP
  endpoint; `initialize` + `tools/list` + `tools/call discoverSolution` all succeeded; the
  returned answer carried the full solution text (verified against the direct REST response).
- Discover through the whole generated chain: **21.3 ms ± 0.9 ms warm** (hyperfine, 20 runs)
  vs 1.2 ms direct REST. Nothing a user would feel.
- Queue writes round-trip: both probes appear in `list-queue` (total 4286), `get-queue-status`
  shows honest per-submission state, and `estimated_time` is derived from the rolling
  AvgSolveTime (3m29s; the old fixed "30s" is gone).
- MusterFlow `--help` inherits the spec's `allowed: pre-phase, end-of-day, post-debug` — the
  2026-08-30 "error messages omit allowed values" friction is fixed at the spec level.
- Install leg green verbatim again (see below); bridge files (`muster-config.yaml`,
  `scripts/connect-muster.sh`) exist in the fresh clone.

## What broke / friction (rows filed)

| # | Finding | Severity |
|---|---------|----------|
| DF-OFF-BY-ONE-17 | The Muster path the README promises is a dead end for a new user: Quick Start never mentions it, the `muster` binary is unbuildable (`go install github.com/wojons/muster@latest` — module is private, no published tag), and `connect-muster.sh` prints "Muster binary not found" then **exits 0** reporting "Integration Complete". The bridge's consumer side has never been installable from scratch. | P2 |
| DF-OFF-BY-ONE-18 | `connect-muster.sh` hardcodes :8766 (kills a foreign daemon on a port-collision host, the README's own documented scenario) and health-checks Muster on :8767, a port nothing in `muster-config.yaml` pins. `OFF_BY_ONE_URL` override exists but is undocumented. | P2 |
| DF-OFF-BY-ONE-19 | MusterFlow table output truncates every wide field to ~120 chars — answers/queue entries unreadable; `--output json` is the workaround and is not hinted in the truncated table. (MusterFlow-side row DF-038.) | P3 |
| DF-OFF-BY-ONE-20 | `q=` search semantics unstated: `pep 668` needs the full AND token match (3 hits incl. FTS noise), `pep668` gives exactly 1; right-truncation works, left does not. One sentence in api-reference.md would remove the guesswork. | P3 |

## Install leg (bunker las-bunker-03, agent f0557da1, PROVEN)

- Public HTTPS clone 3s → HEAD 17a481c (same commit as control host).
- Bare Debian agent: **no Go toolchain** (README assumes it; manual tarball, same as runs
  09-19/22/23 — DF-OFF-BY-ONE-11 remains open on this).
- `make build` **71s** · `./off-by-one seed` **33s** (2138 classes / 2229 answers / 9020 edges)
  · serve on :18766 · `/health` 200 · discover `found:true` 10ms.
- `./scripts/connect-muster.sh --dry-run OFF_BY_ONE_URL=:18766`: steps 1–3 green; step 4
  correctly reports muster binary not installed — and this is where the DF-17 dead end lives.
- Agent destroyed (`bunker destroy` exit 0; list clean — f0557da1 gone, remaining agents are
  other lanes').

## Perf (Step 2b verdict: nothing to file)

Headline operation (discover via the full generated chain) 21.3ms warm / 0.19s first call
cold incl. process start; direct REST 1.2–30ms; install 71s build + 33s seed. No user-noticeable
slow path in the consumed surface → no PERF row (per skill: a win nobody can feel is not a
finding).

## Verdict

**SHIPPABLE** for the lab itself (run #10 = 3rd consecutive SHIPPABLE; open items DF-12 public
catalog, DF-11 fresh-machine toolchain, DF-16 chat silence carry forward). The new angle's
verdict is harsher: the **Muster consumer path is documentation-real but install-unreal**
(DF-17/18) — everything on the wire works once a Muster binary exists, but nothing in the
repo can produce one for a fresh user.
