# 2026-09-19 — Real-use run (dogfood tick off-by-one-dogfood-2026-09-19-20-45-38)

## Promise under test
"A user can run a pre-solve lab: submit problems, discover pre-verified answers
by problem class, browse/search/filter the corpus, and share verified answers
with other labs via git export/import."

## What I did (as a user, not a tester)
- Built from HEAD c5b245c (`go build ./cmd/off-by-one`), seeded a scratch DB
  (`./off-by-one seed -db /tmp/dogfood-ob/data/db.sqlite`, 14s: 1910 classes,
  1998 answers, 7722 edges), served on :18903.
- Discover: known class → `found:true` with the full solution + evidence +
  signatures. Unknown class → clean 404 JSON.
- Submit: new problem → `queued`, position 1. Same tuple again → HTTP 409
  `status:"deduplicated"` with the SAME submission id (docs match behavior).
  Bad cadence → 400 enumerating accepted values. Missing problem_class → 400.
- Browse/search: `q=nil` full-text matches titles; `env=` / `lang=` filters now
  actually filter (live OB-GAP-080 fix check: unfiltered total 1910 → lang=go
  1326 → lang=python 246 → env=linux 26 → env=docker 1104 — filter hits the
  DB, not the payload). `limit=500` and `limit=101` clamp to the documented
  max 100 (live OB-GAP-081 fix check). Note: list rows carry only
  id/title/answer_count/status — class slug is in `title`, not a `class` key.
- Export/Import round trip (the leg that was dead on 09-07): WORKS.
  Setup that the API requires (undocumented): the export target needs a real
  git remote — `target_repo` must equal the clone's origin URL, the clone
  workspace must exist under OFF_BY_ONE_EXPORT_DIR with that origin, and the
  remote branch must already exist. With a bare remote at
  /tmp/dogfood-ob/answers-remote.git: POST /api/v1/export → 200
  `{commit_sha:0fdd5a6, files_changed:6}`; re-import of the same repo →
  `{added:0, updated:1}` (dedup works, no silent skip of fresh data —
  DF-OFF-BY-ONE-7 class of bug is gone; bogus source now 500s with the exact
  git error instead of 200ing).
- Import error paths: nonexistent source_repo → 500 `import_failed` with the
  verbatim git stderr (good signal, though a 4xx would be more correct).
- Web UI serves (200, 5KB SPA); /openapi.json served with sha256 stamp.

## Findings → board
- DF-OFF-BY-ONE-10 (P1): GET /api/v1/queue (list) returns
  `{"entries":null,"total":0}` while two pending submissions exist and
  GET /api/v1/queue/{id} returns them. The polling user never sees their job
  in the list.
- DF-OFF-BY-ONE-11 (P2): fresh-machine install — no Go bootstrap recipe in
  Quick Start; private-repo clone needs a credential a fresh box won't have.

## The right way (for the next user)
1. `git clone <repo> && cd off-by-one && cp .env.example .env`
2. `make build` (NOT bare `go build` — the version-stamp check rejects it)
3. `./off-by-one seed [-db path]` then `./off-by-one serve [-db path]`
4. For corpus sharing: create a BARE git remote, `git clone` it into
   $OFF_BY_ONE_EXPORT_DIR, then pass `target_repo` = the remote's URL with
   `answer_ids` from GET .../answers. Export commits land on the remote's
   branch; any lab can import from the same URL.
