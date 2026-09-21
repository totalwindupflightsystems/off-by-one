# Verdict: RELEASE-OB-001

**Task:** CHANGELOG.md [0.1.0] heading is a prose lie — no release tag or GitHub Release object exists
**Evaluated:** 2026-09-21T08:54:21.799211
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.384s
- ✗ **tier2**
  - INCOMPLETE
  ✗ CHANGELOG.md carries no promoted [x.y.z] heading claiming a shipped release; version history lives under an Unreleased section naming the future first cut, and  /  no longer contradict the file: Clause 1 (CHANGELOG.md) PASSES: CHANGELOG.md:3 is '## [Unreleased]'; `grep -nE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md` exits 1 (no promoted [x.y.z] heading); lines 10-12 name the future first cut ('The first tagged cut is pending the release tooling work tracked in the project board (RELEASE-OB-002). Nothing in this section should be read as version 0.1.0 being published.'). Premise independently verified: `git tag -l` empty, `git ls-remote --tags origin` empty, `gh release list` empty. Commit c18cc9a correctly replaced '## [0.1.0] — 2026-07-24' with '## [Unreleased]'. HOWEVER clause 2 ('and  /  no longer contradict the file' — two file refs stripped by the sanitizer, confirmed via yaml.safe_load) FAILS: the public landing page docs/index.html:177 still renders `<div class="badge">🚀 v0.1.0 — Public Beta</div>` and links to CHANGELOG.md at docs/index.html:346, directly contradicting CHANGELOG.md:12. Other unfixed release-claiming surfaces: ob1-sitrep-prd.html:60 ('Public Beta v0.1.0') and specs/ui-spec.md:29,678 ('v0.1.0'). `git show --stat c18cc9a` confirms the commit changed ONLY CHANGELOG.md (1 file changed, 9 insertions, 2 deletions), so no contradicting surface was corrected. README.md is clean (no v0.1.0/Public Beta/release claim; its only 0.1.0 mention at line 341 is the accurate 0.1.0-dev build-stamp note). Because the criterion is conjunctive and the 'no longer contradict' clause is unsatisfied, the criterion FAILS.
CHANGELOG.md was correctly demoted to an [Unreleased] section naming the future first cut, but the second clause of the criterion is unmet: docs/index.html:177 still advertises 'v0.1.0 — Public Beta' (and links to the CHANGELOG), contradicting the file, and the commit touched only CHANGELOG.md.

## Summary

Judge Result: RELEASE-OB-001

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.384s

Stage tier2: FAIL
  INCOMPLETE
  ✗ CHANGELOG.md carries no promoted [x.y.z] heading claiming a shipped release; version history lives under an Unreleased section naming the future first cut, and  /  no longer contradict the file: Clause 1 (CHANGELOG.md) PASSES: CHANGELOG.md:3 is '## [Unreleased]'; `grep -nE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md` exits 1 (no promoted [x.y.z] heading); lines 10-12 name the future first cut ('The first tagged cut is pending the release tooling work tracked in the project board (RELEASE-OB-002). Nothing in this section should be read as version 0.1.0 being published.'). Premise independently verified: `git tag -l` empty, `git ls-remote --tags origin` empty, `gh release list` empty. Commit c18cc9a correctly replaced '## [0.1.0] — 2026-07-24' with '## [Unreleased]'. HOWEVER clause 2 ('and  /  no longer contradict the file' — two file refs stripped by the sanitizer, confirmed via yaml.safe_load) FAILS: the public landing page docs/index.html:177 still renders `<div class="badge">🚀 v0.1.0 — Public Beta</div>` and links to CHANGELOG.md at docs/index.html:346, directly contradicting CHANGELOG.md:12. Other unfixed release-claiming surfaces: ob1-sitrep-prd.html:60 ('Public Beta v0.1.0') and specs/ui-spec.md:29,678 ('v0.1.0'). `git show --stat c18cc9a` confirms the commit changed ONLY CHANGELOG.md (1 file changed, 9 insertions, 2 deletions), so no contradicting surface was corrected. README.md is clean (no v0.1.0/Public Beta/release claim; its only 0.1.0 mention at line 341 is the accurate 0.1.0-dev build-stamp note). Because the criterion is conjunctive and the 'no longer contradict' clause is unsatisfied, the criterion FAILS.
CHANGELOG.md was correctly demoted to an [Unreleased] section naming the future first cut, but the second clause of the criterion is unmet: docs/index.html:177 still advertises 'v0.1.0 — Public Beta' (and links to the CHANGELOG), contradicting the file, and the commit touched only CHANGELOG.md.

Overall: FAIL ✗
