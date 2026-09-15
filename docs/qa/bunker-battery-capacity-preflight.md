# QA battery harness — capacity preflight (QA-OFF-BY-ONE-9)

The fresh-system QA battery is driven by `~/.hermes/scripts/bunker-qa.sh` (a
fleet script that lives OUTSIDE this repo and is not tracked here). This note
records the harness change made for board row **QA-OFF-BY-ONE-9** so the fix is
reviewable from inside the project that filed the finding.

## Defect

`run_battery()` preflighted only two *deterministic* failure classes —
server-missing-from-`~/.bunker/config.yaml` and ssh-unreachable — and then
called `spawn_agent()`, whose 3-attempt retry loop treated every spawn failure
as transient. A full agent pool is not transient: `bunker spawn` answers
`port range allocation: no free port ranges available` (port-pool class) or
`capacity full: C/C agents` (agent-ceiling class) no matter how many times it is
retried. The observable result was the retry ladder QA-OFF-BY-ONE-8/10/11 filed:

```
[run_battery] FAIL — spawn failed on bunker-las-02: spawn attempt 2 failed:
bunker: spawn agent: internal: port range allocation: no free port ranges available
```

Three wasted spawn attempts + ~10s of backoff per cycle, and one actionable
finding reported as congestion.

## Fix (bunker-qa.sh)

1. Two pure helpers above `build_remote_script()`:
   - `parse_bunker_capacity <srv> <status-output>` — extracts `USED MAX` from the
     `Agents: N/MAX` line, scoping to the `── <srv> ──` section when the output
     holds several sections (and refusing to borrow another host's numbers when
     none matches). Prints nothing when the text is unparseable.
   - `bunker_capacity <srv>` — runs `bunker status --server <srv>`; if that yields
     no usable pair, falls back to `bunker list --server <srv> --status all`
     (`Total: N agents`) for the used count. At most TWO cheap CLI probes; no
     spawn, no polling. Returns `USED MAX`, `USED -` (ceiling unreadable) or
     nothing (unknown).
2. A capacity block in `run_battery()`, immediately after the existing
   config/ssh preflight and BEFORE `spawn_agent()`:
   - `used >= max` → one `run_battery` FAIL evidence row (detail carries `N/MAX`
     and `no spawn attempted`), `PREFLIGHT FAIL: bunker capacity exhausted on
     <server> (N/MAX agents)` on stderr, `exit 2` — the same
     record-one-row-and-stop contract as the two preflights above.
   - ceiling unreadable / probe failed → **warn and proceed**. Unknown is not
     full; refusing on a probe hiccup would wedge the QA lane for every project
     on that host.
   - Capacity available → no output at all (the path is unchanged).
3. `BUNKER_QA_SKIP_PREFLIGHT=1` bypasses the capacity probe only (the
   config/ssh preflights stay in force).

The capacity source of truth is the CLI because bunkerd speaks connect/gRPC —
raw HTTP probes against `/api/v1/*` return 404 without proving anything.

## Verification

Parser fixtures (13/13, `parse_bunker_capacity` driven with live and synthetic
status text): live `bunker-las-02` → `4 8`; multi-section text scoped per host;
no matching host → unknown (never another host's numbers); whitespace-tolerant
`Agents:0 / 12`; empty/garbage → unknown; `list` fallback → `3 -`.

End-to-end (21/21) — the real `run_battery` driven through a stub `bunker` on
`PATH` that records every invocation, so "no spawn attempted" is provable:

| case | pool | result |
|---|---|---|
| A | stub `Agents: 8/8` (full) | `rc=2`, `PREFLIGHT FAIL … (8/8 agents)`, exactly ONE evidence row, **0 spawn invocations**, no retry-ladder text |
| B | stub `Agents: 2/8` | no PREFLIGHT FAIL; reaches `spawn attempt 1` — original path intact |
| C | no `Agents` line, `Total: 3 agents` | warns (`ceiling unreadable … 3 agents live`), proceeds |
| D2 | both probes fail | warns (`capacity unknown`), proceeds |
| E | full + `BUNKER_QA_SKIP_PREFLIGHT=1` | bypass logged, proceeds |
| F | LIVE fleet (`status`/`list` proxied to the real CLI) | live probe `3 8` → proceeds unchanged |

RED proof: the same full-pool case against the pre-fix script exits `rc=1`
after **3 spawn attempts** with evidence
`spawn failed on bunker-las-02: spawn attempt 1 failed: … spawn attempt 2 failed: …`
— the filed symptom — versus `rc=2` / `0` spawn attempts after the fix.

Repro (no real agent is spawned; the stub answers `status`/`list` and fails
`spawn` on purpose):

```bash
mkdir -p /tmp/qa9/stub && cd /tmp/qa9/stub   # stub logs "$*" per call, prints a canned status
BUNKER_STUB_MODE=full \
  PATH=/tmp/qa9/stub:$PATH BUNKER_QA_EVIDENCE=/tmp/qa9/ev.jsonl \
  bash ~/.hermes/scripts/bunker-qa.sh run /home/kara/off-by-one ; echo "rc=$?"

# live pass path (capacity IS available — the probe hits the real CLI)
PATH=/tmp/qa9/stub:$PATH BUNKER_STUB_MODE=passthru BUNKER_QA_EVIDENCE=/tmp/qa9/ev2.jsonl \
  bash ~/.hermes/scripts/bunker-qa.sh run /home/kara/off-by-one
```
