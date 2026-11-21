# PM evidence run — DF-OFF-BY-ONE-58 (seed drops corpus created_at)

Run type: PM verification of an already-landed fix. NOT a gitreins judge verdict.
Date: 2026-11-21 07:06-05:00 (12:06 UTC)
Repo: /home/kara/off-by-one @ master (clean at verification time)

## Command set (all re-run this tick, on the merged tree)

    D=/home/kara/.hermes/cache/scratch/df58
    ./off-by-one seed -db $D/fresh.db -dir ./data
    sqlite3 $D/fresh.db "select count(distinct created_at) from problem_classes"
    sqlite3 $D/fresh.db "select created_at from problem_classes where id=89"
    sqlite3 $D/fresh.db "select id,title,created_at from problem_classes where created_at like '2026-07-25 01:10:08%'"
    sqlite3 $D/fresh.db "select a.id,a.created_at from answer_nodes a join problem_classes c on c.id=a.class_id where c.created_at like '2026-07-25 01:10:08%' order by a.id"
    ./off-by-one seed -db $D/fresh.db -dir ./data      # re-seed, idempotence arm

## Measured output

    seed complete: files=3249; classes=3242 created / 7 existing; answers=4876 created / 2574 skipped; edges=15016 created
    real 0m8.713s

    arm1 distinct created_at            = 3242      (AC: > 1)                     PASS
    arm2 problem_classes.id=89          = 2026-07-25 01:10:08 (corpus value)      PASS
    arm3 class 89 answers               = 125 -> 2026-07-25 01:10:08
                                          126 -> 2026-10-01 07:20:47              PASS (by property)
    arm4 re-seed                        = classes=0 created / 3249 existing;
                                          3242 distinct stamps still; id89 unchanged  PASS

## Disclosure: stale id in the criterion text

The AC said `select created_at from answer_nodes where class_id=112` returns the two
corpus dates. Class 112 in a fresh DB is `garbled-circuit-evaluator` (created_at
2026-07-26 00:22:30) and its answers are 2026-07-26 00:22:30 / 2026-10-01 07:20:47 —
also corpus dates, not the seed clock, but not the dates the criterion pinned. The
class whose corpus created_at is 2026-07-25 01:10:08 is id=89
(`serde-json-validator-jsonschema`) and its two answer nodes are 125/126 carrying
2026-07-25 01:10:08 and 2026-10-01 07:20:47 — exactly the pair the criterion describes.
So the criterion's numeric ids were stale (id assignment is insertion-order dependent);
the property it asserts holds. The criterion is rewritten to key on the class title and
date pair instead of numeric ids so it is reproducible from any fresh seed.

## What the bug was, and that it is fixed in code

internal/seed/seed.go:207 now calls UpsertProblemClassAt(..., cf.CreatedAt) and :263
CreateAnswerNodeAt(..., ans.CreatedAt); internal/graph/store.go:261/:410 insert an
explicit created_at. Fix commit dc96707f (2026-10-23, ancestor of master).
Before the fix every seeded row fell back to the schema DEFAULT datetime('now') — the
10-23 report measured 60 distinct stamps for 2856 classes, all inside the 59s seed run.
Now: 3242 classes carry 3242 distinct corpus dates.
