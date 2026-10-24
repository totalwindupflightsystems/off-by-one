# Verdict: DF-OFF-BY-ONE-60

**Task:** Keep README release install pin current
**Evaluated:** 2026-10-24T16:29:35.667390
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ README release install snippet uses the latest GitHub release tag and its version equality probe passes.: README.md:418 now reads `V=v0.2.0` inside the 'Install from a release (no toolchain needed)' block (README.md:413-431). Latest release tag is v0.2.0: `git tag --sort=-creatordate` -> v0.2.0, v0.1.1, v0.1.0; `git describe --tags --abbrev=0` -> v0.2.0; `git ls-remote --tags origin` -> v0.2.0 is the newest remote tag. Equality probe run fresh: `test "$(grep -m1 '^V=' README.md | cut -d= -f2)" = "$(git describe --tags --abbrev=0)"` printed 'README pin: V=v0.2.0 / latest tag: v0.2.0 / PROBE_PASS' (exit 0). Change landed in commit 82ffd2c8 'docs: keep release install pin current' (README.md 1 insertion, 1 deletion: V=v0.1.1 -> V=v0.2.0), merged as 18ad8e65. No stale `V=v0.1.1` remains in README.md or docs/*.md (grep returns only README.md:418 V=v0.2.0). Docs-only change, so no Go test suite applies; the version equality probe output above is the decisive evidence. (docs/dogfood/diagnostics.md:1178 and skills/off-by-one-usage/SKILL.md:596 still narrate the old v0.1.1 pin, but those are historical dogfood findings, not the install snippet.)
README.md:418 pins V=v0.2.0, matching the latest release tag v0.2.0, and the version equality probe passes (PROBE_PASS, exit 0).

## Summary

Judge Result: DF-OFF-BY-ONE-60

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ README release install snippet uses the latest GitHub release tag and its version equality probe passes.: README.md:418 now reads `V=v0.2.0` inside the 'Install from a release (no toolchain needed)' block (README.md:413-431). Latest release tag is v0.2.0: `git tag --sort=-creatordate` -> v0.2.0, v0.1.1, v0.1.0; `git describe --tags --abbrev=0` -> v0.2.0; `git ls-remote --tags origin` -> v0.2.0 is the newest remote tag. Equality probe run fresh: `test "$(grep -m1 '^V=' README.md | cut -d= -f2)" = "$(git describe --tags --abbrev=0)"` printed 'README pin: V=v0.2.0 / latest tag: v0.2.0 / PROBE_PASS' (exit 0). Change landed in commit 82ffd2c8 'docs: keep release install pin current' (README.md 1 insertion, 1 deletion: V=v0.1.1 -> V=v0.2.0), merged as 18ad8e65. No stale `V=v0.1.1` remains in README.md or docs/*.md (grep returns only README.md:418 V=v0.2.0). Docs-only change, so no Go test suite applies; the version equality probe output above is the decisive evidence. (docs/dogfood/diagnostics.md:1178 and skills/off-by-one-usage/SKILL.md:596 still narrate the old v0.1.1 pin, but those are historical dogfood findings, not the install snippet.)
README.md:418 pins V=v0.2.0, matching the latest release tag v0.2.0, and the version equality probe passes (PROBE_PASS, exit 0).

Overall: PASS ✓
