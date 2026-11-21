# Verdict: OB-OPS-008

**Task:** Corpus catch-up export: sanctioned exporter output replaces 3 hygiene-failing hand-written files (+2574 answers)
**Evaluated:** 2026-11-21T08:48:52.846532
**Result:** ✗ FAIL

## Criteria

- — **Run: git show --stat d7837da9 -- . 'data/answers' shows the corpus sync. Verify on the committed tree: (1) make check-corpus-hygiene exits 0 — no corpus JSON contains an absolute /home/<user> path or a bare IPv4 literal; (2) data/answers.jsonl has 7450 lines and data/answers/ has 3249 *.json files; (3) the three classes auger-worktree-corpus-regen-abs-paths, auger-killed-pytest-leaks-live-test-namespace and gitreins-precommit-hook-clean-env-drops-exports are present as files whose ids are >= 3336 (regenerated, not the hand-written originals); (4) seeding the committed corpus into an empty SQLite DB with ./off-by-one --db /tmp/x.db seed -dir data creates 3242 problem classes, and running the same seed a second time creates 0 new classes and 0 new answers (idempotent); (5) board row OB-OPS-008 has status complete and carries commit_hash d7837da9. Failure condition: any corpus file retains a /home/ path or IPv4 literal, answers/answers.jsonl counts differ from (2), hygiene gate is red, seed is non-idempotent, or OB-OPS-008 is not complete.**
  - Not examined — evaluation ended before this criterion was reached (no judgement was made about it)

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
- ✗ **tier2**
  - INCOMPLETE
  — Run: git show --stat d7837da9 -- . 'data/answers' shows the corpus sync. Verify on the committed tree: (1) make check-corpus-hygiene exits 0 — no corpus JSON contains an absolute /home/<user> path or a bare IPv4 literal; (2) data/answers.jsonl has 7450 lines and data/answers/ has 3249 *.json files; (3) the three classes auger-worktree-corpus-regen-abs-paths, auger-killed-pytest-leaks-live-test-namespace and gitreins-precommit-hook-clean-env-drops-exports are present as files whose ids are >= 3336 (regenerated, not the hand-written originals); (4) seeding the committed corpus into an empty SQLite DB with ./off-by-one --db /tmp/x.db seed -dir data creates 3242 problem classes, and running the same seed a second time creates 0 new classes and 0 new answers (idempotent); (5) board row OB-OPS-008 has status complete and carries commit_hash d7837da9. Failure condition: any corpus file retains a /home/ path or IPv4 literal, answers/answers.jsonl counts differ from (2), hygiene gate is red, seed is non-idempotent, or OB-OPS-008 is not complete.: Not examined — evaluation ended before this criterion was reached (no judgement was made about it)
  ✗ Coverage FAILED — not passing: Run: git show --stat d7837da9 -- . 'data/answers' shows the corpus sync. Verify on the committed tree: (1) make check-corpus-hygiene exits 0 — no corpus JSON contains an absolute /home/<user> path or a bare IPv4 literal; (2) data/answers.jsonl has 7450 lines and data/answers/ has 3249 *.json files; (3) the three classes auger-worktree-corpus-regen-abs-paths, auger-killed-pytest-leaks-live-test-namespace and gitreins-precommit-hook-clean-env-drops-exports are present as files whose ids are >= 3336 (regenerated, not the hand-written originals); (4) seeding the committed corpus into an empty SQLite DB with ./off-by-one --db /tmp/x.db seed -dir data creates 3242 problem classes, and running the same seed a second time creates 0 new classes and 0 new answers (idempotent); (5) board row OB-OPS-008 has status complete and carries commit_hash d7837da9. Failure condition: any corpus file retains a /home/ path or IPv4 literal, answers/answers.jsonl counts differ from (2), hygiene gate is red, seed is non-idempotent, or OB-OPS-008 is not complete.
Cap exceeded: Iteration cap (200) reached (200.5 used). Increase max_iterations or split criteria. Partial verdict — 0 of 1 criteria graded, 1 ungraded. The run spent 354,626 in / 4,504 out over 200.5 iterations.

## Summary

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check

Stage tier2: FAIL
  INCOMPLETE
  — Run: git show --stat d7837da9 -- . 'data/answers' shows the corpus sync. Verify on the committed tree: (1) make check-corpus-hygiene exits 0 — no corpus JSON contains an absolute /home/<user> path or a bare IPv4 literal; (2) data/answers.jsonl has 7450 lines and data/answers/ has 3249 *.json files; (3) the three classes auger-worktree-corpus-regen-abs-paths, auger-killed-pytest-leaks-live-test-namespace and gitreins-precommit-hook-clean-env-drops-exports are present as files whose ids are >= 3336 (regenerated, not the hand-written originals); (4) seeding the committed corpus into an empty SQLite DB with ./off-by-one --db /tmp/x.db seed -dir data creates 3242 problem classes, and running the same seed a second time creates 0 new classes and 0 new answers (idempotent); (5) board row OB-OPS-008 has status complete and carries commit_hash d7837da9. Failure condition: any corpus file retains a /home/ path or IPv4 literal, answers/answers.jsonl counts differ from (2), hygiene gate is red, seed is non-idempotent, or OB-OPS-008 is not complete.: Not examined — evaluation ended before this criterion was reached (no judgement was made about it)
  ✗ Coverage FAILED — not passing: Run: git show --stat d7837da9 -- . 'data/answers' shows the corpus sync. Verify on the committed tree: (1) make check-corpus-hygiene exits 0 — no corpus JSON contains an absolute /home/<user> path or a bare IPv4 literal; (2) data/answers.jsonl has 7450 lines and data/answers/ has 3249 *.json files; (3) the three classes auger-worktree-corpus-regen-abs-paths, auger-killed-pytest-leaks-live-test-namespace and gitreins-precommit-hook-clean-env-drops-exports are present as files whose ids are >= 3336 (regenerated, not the hand-written originals); (4) seeding the committed corpus into an empty SQLite DB with ./off-by-one --db /tmp/x.db seed -dir data creates 3242 problem classes, and running the same seed a second time creates 0 new classes and 0 new answers (idempotent); (5) board row OB-OPS-008 has status complete and carries commit_hash d7837da9. Failure condition: any corpus file retains a /home/ path or IPv4 literal, answers/answers.jsonl counts differ from (2), hygiene gate is red, seed is non-idempotent, or OB-OPS-008 is not complete.
Cap exceeded: Iteration cap (200) reached (200.5 used). Increase max_iterations or split criteria. Partial verdict — 0 of 1 criteria graded, 1 ungraded. The run spent 354,626 in / 4,504 out over 200.5 iterations.

Overall: FAIL ✗
