# Verdict: DOC-13

**Task:** Regenerate site/index.html from current corpus and fix title-slot bug
**Evaluated:** 2026-11-24T23:29:15.453070
**Result:** ✓ PASS

## Criteria

- ✓ **site/index.html title/meta/og/band literals equal a same-tree corpus census (files in data/answers/*.json and sum of answers[]); generate-static-site.py title slot emits total_answers; counts-parity assert added inside existing corpus-jsonl CI job (no new top-level job)**
  - Census: `ls data/answers/*.json | wc -l` = 3249 and `jq -s '[.[] | .answers | length] | add' data/answers/*.json` = 7450. site/index.html:6 title=7450 answers, :7 meta description=3249 classes/7450 answers, :9 og:description=3249 classes/7450 answers, :131 band=3249 classes/7450 answers (tiles :101/:102 also 7450/3249; 5 occurrences of each number total). Generator: scripts/generate-static-site.py:352 <title> now uses {total_answers} (old '{total_classes} verified' gone — grep exit 1), with total_answers = sum(len(c["answers"])) at :281. CI: ci.yml:211 existing corpus-jsonl job contains the DOC-13 parity step at :223-242; next top-level job readme-release-pin starts at :248 — no new top-level job.
- ✓ **derived: Regeneration reproduces the committed site/index.html numbers (title slot emits total_answers from the same-tree corpus; committed file not hand-edited)**
  - Ran `python3 scripts/generate-static-site.py` end-to-end against the same tree; diff of regenerated vs committed index.html shows ONLY line 132 footer date differing (2026-11-23 committed vs 2026-11-21 regenerated; corpus_date() is mtime-based per generate-static-site.py:410-419) — every count slot identical; site/sitemap.xml byte-identical. Working tree restored to delivered state afterwards (`git checkout -- site/` → 0 dirty files under site/).
- ✓ **derived: The parity assert is non-vacuous — each sed extraction yields a non-empty value from the current site/index.html, and a corrupted slot would fail the job**
  - Executed the ci.yml:223-242 step verbatim: CLASSES='3249' ANSWERS='7450' TITLE='7450' DESC_C='3249' DESC_A='7450' OG='7450' BAND='7450' → PARITY PASS with all five extractions non-empty. Each guard is `[ "$X" = "$N" ] || fail ...; exit 1`, so a stale or unmatchable slot (empty extraction) yields '' != census and fails the step; echo line 'parity OK: site advertises 3249 problem classes / 7450 verified answers' would print on green.

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ site/index.html title/meta/og/band literals equal a same-tree corpus census (files in data/answers/*.json and sum of answers[]); generate-static-site.py title slot emits total_answers; counts-parity assert added inside existing corpus-jsonl CI job (no new top-level job): Census: `ls data/answers/*.json | wc -l` = 3249 and `jq -s '[.[] | .answers | length] | add' data/answers/*.json` = 7450. site/index.html:6 title=7450 answers, :7 meta description=3249 classes/7450 answers, :9 og:description=3249 classes/7450 answers, :131 band=3249 classes/7450 answers (tiles :101/:102 also 7450/3249; 5 occurrences of each number total). Generator: scripts/generate-static-site.py:352 <title> now uses {total_answers} (old '{total_classes} verified' gone — grep exit 1), with total_answers = sum(len(c["answers"])) at :281. CI: ci.yml:211 existing corpus-jsonl job contains the DOC-13 parity step at :223-242; next top-level job readme-release-pin starts at :248 — no new top-level job.
  ✓ derived: Regeneration reproduces the committed site/index.html numbers (title slot emits total_answers from the same-tree corpus; committed file not hand-edited): Ran `python3 scripts/generate-static-site.py` end-to-end against the same tree; diff of regenerated vs committed index.html shows ONLY line 132 footer date differing (2026-11-23 committed vs 2026-11-21 regenerated; corpus_date() is mtime-based per generate-static-site.py:410-419) — every count slot identical; site/sitemap.xml byte-identical. Working tree restored to delivered state afterwards (`git checkout -- site/` → 0 dirty files under site/).
  ✓ derived: The parity assert is non-vacuous — each sed extraction yields a non-empty value from the current site/index.html, and a corrupted slot would fail the job: Executed the ci.yml:223-242 step verbatim: CLASSES='3249' ANSWERS='7450' TITLE='7450' DESC_C='3249' DESC_A='7450' OG='7450' BAND='7450' → PARITY PASS with all five extractions non-empty. Each guard is `[ "$X" = "$N" ] || fail ...; exit 1`, so a stale or unmatchable slot (empty extraction) yields '' != census and fails the step; echo line 'parity OK: site advertises 3249 problem classes / 7450 verified answers' would print on green.
site/index.html title/meta/og/band literals exactly match the same-tree census (3249 class files, 7450 summed answers), the generator's title slot now emits total_answers, the counts-parity assert runs green and non-vacuously inside the existing corpus-jsonl CI job, and regeneration from the same tree reproduces every count literal (only the mtime-based footer date differs). [uncovered failure paths: failure mode 1 (Footer 'generated' date is mtime-dependent (corpus_date() uses os.path.getmtime), so regenerating on a fresh clone reproduces counts but not the committed date literal): detected_by is empty — uncovered failure path]
- ✓ **fix_verification**
  - fix_verification: 0 finding(s) verified

## Summary

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ site/index.html title/meta/og/band literals equal a same-tree corpus census (files in data/answers/*.json and sum of answers[]); generate-static-site.py title slot emits total_answers; counts-parity assert added inside existing corpus-jsonl CI job (no new top-level job): Census: `ls data/answers/*.json | wc -l` = 3249 and `jq -s '[.[] | .answers | length] | add' data/answers/*.json` = 7450. site/index.html:6 title=7450 answers, :7 meta description=3249 classes/7450 answers, :9 og:description=3249 classes/7450 answers, :131 band=3249 classes/7450 answers (tiles :101/:102 also 7450/3249; 5 occurrences of each number total). Generator: scripts/generate-static-site.py:352 <title> now uses {total_answers} (old '{total_classes} verified' gone — grep exit 1), with total_answers = sum(len(c["answers"])) at :281. CI: ci.yml:211 existing corpus-jsonl job contains the DOC-13 parity step at :223-242; next top-level job readme-release-pin starts at :248 — no new top-level job.
  ✓ derived: Regeneration reproduces the committed site/index.html numbers (title slot emits total_answers from the same-tree corpus; committed file not hand-edited): Ran `python3 scripts/generate-static-site.py` end-to-end against the same tree; diff of regenerated vs committed index.html shows ONLY line 132 footer date differing (2026-11-23 committed vs 2026-11-21 regenerated; corpus_date() is mtime-based per generate-static-site.py:410-419) — every count slot identical; site/sitemap.xml byte-identical. Working tree restored to delivered state afterwards (`git checkout -- site/` → 0 dirty files under site/).
  ✓ derived: The parity assert is non-vacuous — each sed extraction yields a non-empty value from the current site/index.html, and a corrupted slot would fail the job: Executed the ci.yml:223-242 step verbatim: CLASSES='3249' ANSWERS='7450' TITLE='7450' DESC_C='3249' DESC_A='7450' OG='7450' BAND='7450' → PARITY PASS with all five extractions non-empty. Each guard is `[ "$X" = "$N" ] || fail ...; exit 1`, so a stale or unmatchable slot (empty extraction) yields '' != census and fails the step; echo line 'parity OK: site advertises 3249 problem classes / 7450 verified answers' would print on green.
site/index.html title/meta/og/band literals exactly match the same-tree census (3249 class files, 7450 summed answers), the generator's title slot now emits total_answers, the counts-parity assert runs green and non-vacuously inside the existing corpus-jsonl CI job, and regeneration from the same tree reproduces every count literal (only the mtime-based footer date differs). [uncovered failure paths: failure mode 1 (Footer 'generated' date is mtime-dependent (corpus_date() uses os.path.getmtime), so regenerating on a fresh clone reproduces counts but not the committed date literal): detected_by is empty — uncovered failure path]

Stage fix_verification: PASS
  fix_verification: 0 finding(s) verified

Overall: PASS ✓
