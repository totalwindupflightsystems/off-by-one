# Verdict: OB-GAP-093

**Task:** Redact internal names from class-id-derived site slugs/titles
**Evaluated:** 2026-10-13T17:41:36.661183
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ grep -ri dexdat site/ returns 0 hits after regeneration; slug deterministic + collision-checked; class still published; regression test in scripts/tests/: Ran `python3 scripts/generate-static-site.py` (exit 0, 'index.html: 2649 classes / 4277 answers'); `grep -ri dexdat site/ | wc -l` = 0 after regeneration, and regeneration is idempotent (git diff site/ empty). Deterministic: public_stem() is a pure function of the stem (scripts/generate-static-site.py:50-65) and test_deterministic + TestEndToEndDeterminism (main() twice -> identical tree) pass. Collision-checked: build_slug_map() raises SystemExit when two distinct raw ids map to one slug (scripts/generate-static-site.py:68-82); test_distinct_raw_ids_colliding_abort passes. Class still published: site/classes/2638-project-core-bunker-qa-compose-up-fail-fresh-agent.html exists, appears in site/sitemap.xml (1 URL) and data/INDEX.md:2557; title/og:title/h1/canonical all redacted to '<project>'/'project' token. Regression test: scripts/tests/generate_static_site_redaction_test.py — `python3 -m pytest ... -v` => '15 passed, 11 subtests passed' (exit 0); `make site-redaction-selftest` => 'Ran 15 tests ... OK'. Guard NAME_PATTERN case-sensitive grep over site/ returns 0 files (exit 1 = no match).


## Summary

Judge Result: OB-GAP-093

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ grep -ri dexdat site/ returns 0 hits after regeneration; slug deterministic + collision-checked; class still published; regression test in scripts/tests/: Ran `python3 scripts/generate-static-site.py` (exit 0, 'index.html: 2649 classes / 4277 answers'); `grep -ri dexdat site/ | wc -l` = 0 after regeneration, and regeneration is idempotent (git diff site/ empty). Deterministic: public_stem() is a pure function of the stem (scripts/generate-static-site.py:50-65) and test_deterministic + TestEndToEndDeterminism (main() twice -> identical tree) pass. Collision-checked: build_slug_map() raises SystemExit when two distinct raw ids map to one slug (scripts/generate-static-site.py:68-82); test_distinct_raw_ids_colliding_abort passes. Class still published: site/classes/2638-project-core-bunker-qa-compose-up-fail-fresh-agent.html exists, appears in site/sitemap.xml (1 URL) and data/INDEX.md:2557; title/og:title/h1/canonical all redacted to '<project>'/'project' token. Regression test: scripts/tests/generate_static_site_redaction_test.py — `python3 -m pytest ... -v` => '15 passed, 11 subtests passed' (exit 0); `make site-redaction-selftest` => 'Ran 15 tests ... OK'. Guard NAME_PATTERN case-sensitive grep over site/ returns 0 files (exit 1 = no match).


Overall: PASS ✓
