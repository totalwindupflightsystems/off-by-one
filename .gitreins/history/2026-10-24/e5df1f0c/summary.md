# Verdict: DF-OFF-BY-ONE-60

**Task:** Keep README release install pin current
**Evaluated:** 2026-10-24T16:21:41.493616
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ README release install snippet uses the latest GitHub release tag and its version equality probe passes.: README.md:418 still reads `V=v0.1.1` in the 'Install from a release (no toolchain needed)' block (README.md:413-431). The latest release tag is v0.2.0 (`git tag --sort=-creatordate` -> v0.2.0, v0.1.1, v0.1.0; `git describe --tags --abbrev=0` -> v0.2.0). Running the task's own equality probe: `test "$(grep -m1 '^V=' README.md | cut -d= -f2)" = "v0.2.0"` -> prints `V=v0.1.1` then `PROBE_FAIL` (exit 1). README.md is untouched by this task (`git diff HEAD -- README.md` is empty; the only working-tree changes are .coding-hermes/board/tasks.jsonl and .gitreins/tasks.yaml, which merely flip the task status to complete). No guard/probe script was added either — grep for 'version equality|equality probe|latest.*release.*tag' matches only .gitreins/tasks.yaml:1432-1435. The defect documented in docs/dogfood/2026-10-23-release-artifact-integration.md:80 and .coding-hermes/board/tasks.jsonl:263 (snippet installs an artifact two releases behind v0.2.0) remains unfixed.
README.md:418 still pins V=v0.1.1 while the latest release tag is v0.2.0, so the version equality probe fails and the task was marked complete without any change.

## Summary

Judge Result: DF-OFF-BY-ONE-60

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ README release install snippet uses the latest GitHub release tag and its version equality probe passes.: README.md:418 still reads `V=v0.1.1` in the 'Install from a release (no toolchain needed)' block (README.md:413-431). The latest release tag is v0.2.0 (`git tag --sort=-creatordate` -> v0.2.0, v0.1.1, v0.1.0; `git describe --tags --abbrev=0` -> v0.2.0). Running the task's own equality probe: `test "$(grep -m1 '^V=' README.md | cut -d= -f2)" = "v0.2.0"` -> prints `V=v0.1.1` then `PROBE_FAIL` (exit 1). README.md is untouched by this task (`git diff HEAD -- README.md` is empty; the only working-tree changes are .coding-hermes/board/tasks.jsonl and .gitreins/tasks.yaml, which merely flip the task status to complete). No guard/probe script was added either — grep for 'version equality|equality probe|latest.*release.*tag' matches only .gitreins/tasks.yaml:1432-1435. The defect documented in docs/dogfood/2026-10-23-release-artifact-integration.md:80 and .coding-hermes/board/tasks.jsonl:263 (snippet installs an artifact two releases behind v0.2.0) remains unfixed.
README.md:418 still pins V=v0.1.1 while the latest release tag is v0.2.0, so the version equality probe fails and the task was marked complete without any change.

Overall: FAIL ✗
