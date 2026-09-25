# Verdict: DF-OFF-BY-ONE-20

**Task:** q= search token semantics unstated in api-reference
**Evaluated:** 2026-09-25T18:16:26.730133
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: go: downloading github.com/joho/godotenv v1.5.1
- ✓ **tier2**
  - COMPLETE
  ✓ api-reference.md documents q= tokenization semantics and the truncation direction: docs/api-reference.md:135 explicitly documents both required elements. Tokenization semantics: 'the whole value is wrapped as a single FTS5 phrase (internal/graph/search.go, quote-escaping at the top of Store.Search), so the query matches one adjacent run of whole tokens — there is no automatic token ANDing ... A leading or trailing * is treated as literal text, not a wildcard operator.' Truncation direction: 'no left truncation, and no suffix wildcard: ... q=pep66 / q=pep66* return 0 even though pep668 exists.' Code reference verified accurate: internal/graph/search.go:44 `ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"``. Empirically confirmed with sqlite3 FTS5: quoted "pep668"→1 hit, "pep66"→0, "pep66*"→0, "pep 668"→adjacent match — doc claims match actual behavior. Committed in ba4d637 'docs(api): q= is a single FTS5 phrase ... (DF-OFF-BY-ONE-20)'; file is 26635 bytes / 29 headings of real content, not a stub. [resolution 0.29; api-reference.md]
docs/api-reference.md:135 accurately documents q= FTS5 phrase tokenization semantics and the truncation direction (no left truncation, no suffix wildcard), verified against internal/graph/search.go:44 and empirical FTS5 behavior.

## Summary

Judge Result: DF-OFF-BY-ONE-20

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: go: downloading github.com/joho/godotenv v1.5.1

Stage tier2: PASS
  COMPLETE
  ✓ api-reference.md documents q= tokenization semantics and the truncation direction: docs/api-reference.md:135 explicitly documents both required elements. Tokenization semantics: 'the whole value is wrapped as a single FTS5 phrase (internal/graph/search.go, quote-escaping at the top of Store.Search), so the query matches one adjacent run of whole tokens — there is no automatic token ANDing ... A leading or trailing * is treated as literal text, not a wildcard operator.' Truncation direction: 'no left truncation, and no suffix wildcard: ... q=pep66 / q=pep66* return 0 even though pep668 exists.' Code reference verified accurate: internal/graph/search.go:44 `ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"``. Empirically confirmed with sqlite3 FTS5: quoted "pep668"→1 hit, "pep66"→0, "pep66*"→0, "pep 668"→adjacent match — doc claims match actual behavior. Committed in ba4d637 'docs(api): q= is a single FTS5 phrase ... (DF-OFF-BY-ONE-20)'; file is 26635 bytes / 29 headings of real content, not a stub. [resolution 0.29; api-reference.md]
docs/api-reference.md:135 accurately documents q= FTS5 phrase tokenization semantics and the truncation direction (no left truncation, no suffix wildcard), verified against internal/graph/search.go:44 and empirical FTS5 behavior.

Overall: PASS ✓
