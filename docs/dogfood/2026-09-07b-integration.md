# Off-by-One Dogfood Integration — 2026-09-07 (evening run): Export/Import Git Round Trip

**Run type:** deep dogfood, export/import corpus-sharing workflow (never exercised by the
Aug 10 / Aug 30 / Sep 1 / Sep 4 / Sep 7-morning runs, which only hit the
`501 not_configured` path on a default-start server).

**Environment:** fully scratch — fresh `./off-by-one seed -db /tmp/dogfood-ob1/ob1.db`
(14s, 1385 classes / 1470 answers), server started with:

```bash
OFF_BY_ONE_EXPORT_DIR=/tmp/dogfood-ob1/export \
OFF_BY_ONE_IMPORT_DIR=/tmp/dogfood-ob1/import \
OFF_BY_ONE_DB=/tmp/dogfood-ob1/ob1.db \
OFF_BY_ONE_PORT=18901 ./off-by-one
```

Repo DB / live fleet daemon on :8766 untouched. Verdict this run: **PROMISING-BUT-ROUGH**
(import leg works end-to-end; export leg is dead-on-arrival at the API layer).

## What the docs promise

`docs/integration.md` §Import/Export Corpora:

- **Export** — `POST /api/v1/export` with `{target_repo, answer_ids, branch, commit_message}`
  pushes verified answers as flat files into a git repo.
- **Import** — `POST /api/v1/import` with `{source_repo, branch, conflict_strategy}` pulls a
  community answer repo back into the graph.

That is the corpus-sharing value prop: verified answers flow lab → git → other labs.

## Leg 1 — EXPORT: BROKEN (P0, DF-OFF-BY-ONE-6)

Ran the documented example against a real git repo on disk:

```bash
curl -s -X POST http://127.0.0.1:18901/api/v1/export \
  -H "Content-Type: application/json" \
  -d '{"target_repo": "/tmp/dogfood-ob1/export-repo", "answer_ids": [43], "branch": "main"}'
```

Response — **every time, for every answer id tried**:

```
HTTP 500
{"error":"export_failed","message":"export item class=0 answer=43: get problem class: graph: not found"}
```

**Root cause (read from source, not guessed):**

- `internal/api/handlers.go:786-789` — the handler builds
  `items[i] = export.ExportItem{AnswerID: id}` and **never sets ClassID**.
- `internal/export/git.go:255-259` — `writeItem` calls
  `store.GetProblemClass(ctx, item.ClassID)`; class 0 cannot exist → not found.
- The engine itself is correct: `internal/export/git_test.go` always passes
  `ClassID: pc.ID` and those tests are green. The break is **only in the HTTP wiring**.
- `internal/api/handlers_test.go` covers 501/400 for export — the success path has no
  handler-level test. Classic three-layer check failure: L1 compiles, L2 runs, L3
  (a real user doing the documented workflow) is broken.

**Also (DF-OFF-BY-ONE-8):** `commit_message` is accepted in `exportRequest` but no code
path forwards it to the engine — silently dropped. The 500 text leaks internal struct
state (`class=0 answer=43`) instead of telling the user their request shape is the problem.

## Leg 2 — IMPORT: WORKS end-to-end

Authored a community answer exactly as the format docs describe
(`pre-solve-answers/{class}/{env}/{version}/{solution.md,evidence.md,signatures.json}`)
in a scratch git repo, then:

```bash
curl -s -X POST http://127.0.0.1:18901/api/v1/import \
  -H "Content-Type: application/json" \
  -d '{"source_repo": "/tmp/dogfood-ob1/import-repo", "branch": "main", "conflict_strategy": "skip"}'
# → HTTP 200 {"added":1,"updated":0,"skipped":0,"conflicted":0}

curl -s -X POST http://127.0.0.1:18901/api/v1/problems/discover \
  -H "Content-Type: application/json" \
  -d '{"problem_class": "dogfood-import-roundtrip-2026-09-07"}'
# → {"found":true,"answer":{...full solution + evidence + signatures...}}
```

Time-to-first-success on the import leg: **~90 seconds** from server-up to discoverable
answer. Re-importing the same repo correctly dedups: `{"added":0,"skipped":1}`.

### IMPORT PITFALL (P1, DF-OFF-BY-ONE-7): bogus source silently "succeeds"

Second import in the same server lifetime, pointing at a repo that does not exist:

```bash
curl -s -X POST http://127.0.0.1:18901/api/v1/import \
  -d '{"source_repo": "/tmp/does-not-exist-repo", "branch": "main"}'
# → HTTP 200 {"added":0,"updated":0,"skipped":1,"conflicted":0}
```

Identical shape to the benign dedup response. Mechanism: `prepareClone`
(`internal/import/git.go:196`) sees the previous import's clone still sitting in
`ImportLocalDir/.git`, and runs `fetch`/`pull` against **the old origin** — the requested
`source_repo` is never validated. A multi-import server silently re-scans whatever it
imported last. A *first* import to a dead URL does 500 correctly, so this only bites the
long-lived-server case the feature exists for. Fix direction (for the foreman):
per-source-repo clone dirs (hash of URL) or verify `remote.origin.url` matches the
request before fetch; return a 4xx/5xx when the source can't be cloned.

## Other real-use observations

- **`q=` search matches answer bodies, not just titles** — `q=raft` returns
  `docs-org-rename-url-drift` because its answer text mentions raft. Useful, undocumented.
- **env/lang list filters match STORED values** — `q=raft&env=linux` → 0 results because
  the corpus is overwhelmingly `env: "docker"`; the README submit example uses
  `"environment": "linux"`, teaching vocabulary that filters reject (DF-OFF-BY-ONE-9).
  My imported answer with `env=linux` *does* filter correctly, proving the filter itself
  works.
- **Port collisions on this host are a real hazard** — 8766 (fleet daemon) and 18767
  (`crier` service) are both taken; bind failure kills the process with
  `address already in use` after startup logs look healthy.
- **Solver warning is loud and useful on a fresh instance:**
  `WARNING: DEEPSEEK_API_KEY is empty or placeholder — all solves will fail with 401`.
- Cron loop on a busy host logs `cron: tick error: cron: system is not idle` — expected,
  and honest.

## The right way to use import today (until DF-OFF-BY-ONE-6/7 are fixed)

1. Start the server with `OFF_BY_ONE_IMPORT_DIR` set to an **empty, per-source** directory.
2. Import one source repo per server lifetime, or wipe the import dir between sources
   (the stale-clone bug makes cross-source imports unsafe).
3. Author answers as
   `pre-solve-answers/{class-title}/{env}/{version}/solution.md` (+ `evidence.md`,
   optional `signatures.json`) in the format shown in `internal/import/git.go`
   header comment.
4. `POST /api/v1/import` → verify with `POST /api/v1/problems/discover`.
5. Do NOT bother with `POST /api/v1/export` — it 500s on every request as of HEAD
   `ad63507`. Track DF-OFF-BY-ONE-6 for the fix.
