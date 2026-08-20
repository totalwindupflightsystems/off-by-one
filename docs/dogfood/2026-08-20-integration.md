# Off-by-One Dogfood — 2026-08-20 Integration Report (field-test run #2)

**Verdict: ✅ SHIPPABLE** — see `.coding-hermes/dogfood-log.md`.

This is the record of a REAL user run against the live lab on 2026-08-19/20:
an agent (this one) hit a genuine problem, submitted it through the documented
API, watched the idle-cycle solver pick it up inside a bwrap sandbox, and
re-discovered the pre-verified answer. Every step below was executed against
`http://127.0.0.1:8766` (live lab) and a scratch readonly instance on `:8891`.

---

## 1. The real problem (the agent-user scenario)

The host this run executed on has a genuine, common Python toolchain trap:

```
python3       → 3.11.15
pip           → python3.14 (different interpreter, earlier in PATH)
PEP 668       → externally-managed-environment: pip install fails system-wide
```

`pip install <pkg>` fails with `error: externally-managed-environment`, and the
interpreter mismatch makes `python3 -m pip` behave differently from `pip`. This
is a real class of problem agents hit constantly on Debian/Ubuntu hosts.

## 2. The exact workflow (all live, all documented)

### 2.1 Pre-flight — is the class already solved? (avoid duplicate work)

```bash
curl -s "http://localhost:8766/api/v1/problems?q=pep668&limit=5"
# {"problems":null,"total":0}     ← class does not exist (also: see Finding OB-GAP-046)
```

`POST /api/v1/problems/discover` for `python-pip-interpreter-mismatch-pep668`
→ `{"error":"not_found"}` → genuinely new class, submit.

### 2.2 Submit (documented request shape, cadence required)

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/submit -H 'Content-Type: application/json' -d '{
  "problem_class": "python-pip-interpreter-mismatch-pep668",
  "environment": "linux", "language": "python", "version": "3.11.15",
  "description": "...", "error_message": "error: externally-managed-environment",
  "stack_trace": "× This environment is externally managed ...",
  "context": {"observed": "python3=3.11.15, pip->python3.14, PEP 668=yes"},
  "cadence": "post-debug",
  "required_tools": ["python3", "pip"]
}'
# {"submission_id":"sub_e6bf1e","status":"queued","position":10,
#  "estimated_time":"5m0s","existing_solutions":0}
```

### 2.3 Watch it move through the pipeline (the lab working as promised)

| When (UTC) | Observation |
|---|---|
| 04:06:40 | submit → `queued`, position 10, `estimated_time:"5m0s"` |
| 04:12:43 | fleet submission `sub_607de1` (go-sharded-atomic-counter-false-sharing) starts; sandbox dir `/tmp/off-by-one-sandbox-sub_607de1` appears with `problem.json` |
| 04:17:43 | `sub_607de1` failed at exactly 300s — the known bwrap-cap fleet pattern (NOT a regression; `signal: killed` class) |
| 04:18 | next fleet submission `sub_b6862e` (js-merkle-patricia-trie-proof-verification) solves |
| 04:24:43 | **`sub_e6bf1e` → `in_progress` / `solver_running`** — cron loop (`-cron-interval 60s`) picked mine up |
| 04:25:52 | **`sub_e6bf1e` → `complete` in 69 seconds** |

While solving, `ps` shows `bwrap → pi-agent solve --problem-file /workspace/problem.json
--model deepseek-v4-flash` — argv carries NO API keys (OB-GAP-006/015 security
fixes hold). Sandbox dir is cleaned up after completion.

### 2.4 Discover the cached answer (the payoff)

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/discover \
  -H 'Content-Type: application/json' -d '{"problem_class":"python-pip-interpreter-mismatch-pep668"}'
# → HTTP 200 {"found":true,"answer":{...}}
```

The returned answer (id 1347, env=linux/lang=python/version=3.11.15) is a real,
verified solution: it correctly diagnosed BOTH overlapping causes (interpreter
mismatch `pip`→3.14 vs `python3`→3.11; PEP 668 externally-managed guard),
prescribed the venv-from-explicit-interpreter fix, and the solve ran actual
verification inside the sandbox (reproduced `error: externally-managed-environment`,
confirmed the version mismatch, created venvs, `pip install testtools` succeeded
with no PEP 668 error). Evidence + model signature included
(`openrouter/deepseek/deepseek-v4-flash-0731`).

After the solve: `GET /api/v1/stats` → 1172 problems / 1347 answers (mine landed),
`GET /api/v1/problems/python-pip-interpreter-mismatch-pep668/answers` returns the
answer, and `?q=pep668` finds the class. **End-to-end loop: submit → idle-cycle
solve → discover — all live, all documented.**

> Note on statuses: the detail endpoint shows `status:"verified"` for the new
> class; the FTS search list shows `status:"pending"` — see Finding OB-GAP-050.

## 3. Re-verification of run #1 findings (2026-08-10 dogfood) — ALL FIXED

| Run #1 finding | Then | Now (live-verified 2026-08-20) |
|---|---|---|
| OB-GAP-020 readonly discover 403 | only discover endpoint blocked in catalog mode | **200 found:true** on scratch `--readonly` instance (:8891) |
| OB-GAP-021 README counts stale | 812/948 vs live | README has no literal counts; points to `data/COUNTS.md` (auto-stamped) |
| OB-GAP-022 example class 404 | `go-nil-pointer-deref` didn't exist | `so-nil-pointer-deref` discover → 200, deep answer (id 84) |
| OB-GAP-023 tuple semantics | docs said "scores by" | docs state exact-match rule |
| OB-GAP-024 detail status empty | `"status":""` | `"status":"verified"` |
| OB-GAP-025 corpus junk | test/canary classes in export | 0 junk patterns in `data/INDEX.md` |
| OB-GAP-034 silent queue | submissions sat forever w/o solver | submit → **503 solver_unavailable** when no solver |

Also re-verified: `seed` honors `OFF_BY_ONE_DB` + `-db`/`-dir` (OB-GAP-039/044);
readonly submit → 403 `"catalog is read-only — submissions go through the
upstream lab"`; readonly chat → 403 `"AI agent disabled in read-only catalog
mode"`; export/import → 501 with config-guidance messages.

## 4. Errors hit this run (and the right way)

| Error | Cause | Right way |
|---|---|---|
| `TypeError: 'NoneType' object is not iterable` in my own client when parsing search | zero-match search returns `{"problems":null,"total":0}` — **null not `[]`** | Tracked as OB-GAP-046; until fixed, guard with `or []` |
| `{"related":null}` on `GET /api/v1/problems/so-nil-pointer-deref/related` | same null-vs-`[]` family | OB-GAP-046 |
| `seed: read corpus dir data/answers: no such file or directory` + **exit 0** + empty 4KB DB | ran `seed` from outside the repo root; corpus path is CWD-relative and failure is non-fatal | Run from repo root or pass `-dir <repo>/data`; tracked as OB-GAP-048 |
| Port 8890 already in use (SearXNG on this host) | environment collision, not a project bug | pick a free port; health check after start |
| `-dir` pointing at `data/answers` (not `data`) | flag takes the dir that CONTAINS `answers/` | `seed -dir ./data` |

## 5. What a NEW user needs that isn't documented (small stuff)

- `seed` failures are silent (exit 0) — see OB-GAP-048.
- `stats.avg_solve_time` is always `""` and submit's `estimated_time` is a fixed
  "5m0s" default that disappears once the item reaches position 0 — see
  OB-GAP-047. Don't build scheduling logic on it.
- The live lab's queue window (`GET /api/v1/queue?limit=100`) is dominated by
  the `off-by-one-self-test` class (per-class window) — the authoritative
  picture is the DB; don't conclude "nothing is happening" from the window.

## 6. Bottom line for a maintainer (1 hour of your time — do these first)

1. Return `[]` for empty collections (`problems`, `related`) — agent clients
   crash on null (OB-GAP-046).
2. Either compute `avg_solve_time` from completed submissions or remove it
   (OB-GAP-047).
3. Make `seed` exit non-zero when the corpus dir is unreadable (OB-GAP-048).
4. Add a tick-gate that greps `skills/` + `docs/dogfood/` for completed gap IDs
   so knowledge artifacts stop advertising fixed bugs (OB-GAP-049).
