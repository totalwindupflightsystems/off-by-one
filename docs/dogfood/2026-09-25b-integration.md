# Dogfood integration — the community answer-sharing engine (export/import), 2026-09-25 (second run)

**Angle (first exercise in 12 runs):** the git export/import pair — Core Loop
steps 6–7, "community answers" — under CONTENT CHANGE, not the round-trip of
identical content that 09-07 and 09-19 already proved.

**Setup (all scratch, repo DB untouched):**

- producer: `off-by-one -db /tmp/dof-exp/producer/o1p.db -port 18801 -export-dir /tmp/dof-exp/export-work -load-threshold -1` (seeded 38s: 2254 classes / 2346 answers)
- consumer: `-db /tmp/dof-exp/consumer/o1-fresh.db -port 18802 -import-dir /tmp/dof-exp/import-work -load-threshold -1` — **no seed, 0 classes / 0 answers**
- community remote: `git init --bare /tmp/dof-exp/community-repo/answers.git` (branch `main` set via `symbolic-ref`)

## The working recipe (verified end-to-end)

1. **Export** (producer → community repo):
   `POST /api/v1/export {"target_repo":"/…/answers.git","answer_ids":[2,3,4],"branch":"main","commit_message":"…"}`
   → `{"commit_sha":"a950ae0","files_changed":9}` in <1s. Layout is exactly spec §8.1:
   `pre-solve-answers/{class}/{env}/{version}/{solution.md,evidence.md,signatures.json}`.
   Preconditions (from the 09-19 recipe, re-confirmed): bare remote whose URL
   equals `target_repo`, branch exists on it, `-export-dir` set at start (else 501).
2. **Empty-node bootstrap import** (consumer):
   `POST /api/v1/import {"source_repo":"/…/answers.git","branch":"main"}`
   → `{"added":3,"updated":0,"skipped":0,"conflicted":0}`; 156ms cold
   (fresh clone included), ~90ms warm (fetch+walk+diff).
   Immediately after: `POST /api/v1/problems/discover {"problem_class":"go-string-reverse"}`
   → `found:true`, full solution + evidence + signatures (12ms). Classes not
   imported 404 cleanly. Re-import dedups (`skipped:3`).
3. **Error paths worth knowing:**
   - nonexistent `source_repo` → 409 `source_repo_mismatch` naming the existing clone's origin (DF-7 fix live)
   - bad branch → 500 with BOTH git error strings (checkout + fallback)
   - empty `answer_ids` → 400 `answer_ids must be non-empty`
   - endpoints 501 `not_configured` without `-export-dir`/`-import-dir`
   - re-export of an already-exported answer with unchanged DB content → new commit is a no-op diff (files_changed only for genuinely new answers)

## What broke (the findings; full repro in diagnostics §13)

- **P1 DF-OFF-BY-ONE-22:** producer re-export silently clobbers a community
  edit (commit 719cec8, `grep Update v2` 1 → 0; handler forces `Push:true`,
  engine has no upstream diff). The edit is unrecoverable.
- **P1 DF-OFF-BY-ONE-23:** `extractSection` cuts at the first `---` INSIDE the
  solution body (corpus answers commonly end with one) → appended community
  text is parsed away → import reports `skipped: identical content`. Proven
  byte-level (parsed 830 bytes == stored 830 bytes while the FILE grew by 55).
- **P2 DF-OFF-BY-ONE-24:** `conflict_strategy` (skip|replace|manual) is accepted
  by the API, advertised in the OpenAPI spec, and silently dropped at
  handlers.go:970; the engine has no conflict/replace/manual path.
- **P2 DF-OFF-BY-ONE-25 (docs):** the empty-node bootstrap flow (no seed →
  import → discover) is undocumented, and it is the cheapest way to join a
  community answers repo.

## Performance (Step 2b numbers; nothing user-noticeable — no PERF row)

| operation | warm | cold |
|---|---|---|
| import (3 answers, existing clone) | 87–101ms (n=5) | 156ms (fresh clone) |
| import after a 200-answer push (196 answers) | 73ms | — |
| export 3 answers | 89–107ms (n=3) | — |
| export 200 answers (600 files, 1 commit) | 933ms | — |
| discover (imported class) | 12–18ms | — |

Scale probe scaled linearly (3→200 answers, 30x data, ~9x time — dominated by
the single git commit, not per-answer work). Verdict: fast enough everywhere;
the real defects are semantic.
