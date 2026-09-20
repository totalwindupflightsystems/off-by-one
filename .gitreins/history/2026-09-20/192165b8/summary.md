# Verdict: DF-OFF-BY-ONE-11

**Task:** README: fresh-machine install bootstrap (Go toolchain + clone access)
**Evaluated:** 2026-09-20T13:56:58.732918
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.228s
- ✗ **tier2**
  - INCOMPLETE
  ✗ README Quick Start prerequisites carry a Go tarball bootstrap one-liner path that a bare non-sudo user can follow and an explicit note that cloning the repo requires access (private-repo clone is not anonymous), matching the bunker-bare-Debian evidence; existing green path unchanged: The required README content is NOT in the graded tree. HEAD=8df11f19e81c (master): `grep -n 'tarball\|go1\.26\.8\|toolchain\|go.dev/dl' README.md` -> exit 1 (0 matches) and `grep -n 'requires repository access\|no credential\|HTTPS token' README.md` -> exit 1 (0 matches). README.md:289-294 Prerequisites is still the bare '- Go 1.25+' with no tarball bootstrap, and README.md:298-301 Quick Start is still the bare '# Clone / git clone git@github.com:...' with no access note. The work exists only on an UNMERGED branch: `git branch -a --contains c2eff7c` -> '+ wt/DF-OFF-BY-ONE-11' (only) and `git merge-base --is-ancestor c2eff7c HEAD` -> NOT ancestor. Commit c2eff7c 'docs(readme): fresh-machine install bootstrap — Go tarball + clone access (DF-OFF-BY-ONE-11)' does contain exactly the required text (curl -fsSL https://go.dev/dl/go1.26.8.linux-amd64.tar.gz -o /tmp/go.tgz; mkdir -p ~/toolchain && tar -C ~/toolchain -xzf /tmp/go.tgz; export PATH="$HOME/toolchain/go/bin:$PATH"; go version; plus '# Clone — requires repository access (SSH key or HTTPS token for github.com:totalwindupflightsystems/off-by-one). A fresh box with no credential for that repo cannot clone it.'), and `git diff HEAD wt/DF-OFF-BY-ONE-11 -- README.md` shows those hunks as branch-only. The only graded worktree change for this task is .gitreins/tasks.yaml flipping status in_progress -> complete (worktree.patch 0818cc8e) — no README change landed on master. The 'existing green path unchanged' half is trivially satisfied (README.md:296-320 Quick Start is byte-identical to pre-task), but the primary requirement is absent. Test suite for completeness: `go test ./... -short -count=1 -p 1 -timeout 180s` -> EXIT=0, all 13 packages 'ok' (no test_command in .gitreins/config.yaml; guards.tests=false, go.tests=true).
The Go tarball bootstrap and clone-access note exist only on the unmerged branch wt/DF-OFF-BY-ONE-11 (commit c2eff7c); the graded master tree's README still carries the bare 'Go 1.25+' prerequisite and bare clone step, so the criterion fails.

## Summary

Judge Result: DF-OFF-BY-ONE-11

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.228s

Stage tier2: FAIL
  INCOMPLETE
  ✗ README Quick Start prerequisites carry a Go tarball bootstrap one-liner path that a bare non-sudo user can follow and an explicit note that cloning the repo requires access (private-repo clone is not anonymous), matching the bunker-bare-Debian evidence; existing green path unchanged: The required README content is NOT in the graded tree. HEAD=8df11f19e81c (master): `grep -n 'tarball\|go1\.26\.8\|toolchain\|go.dev/dl' README.md` -> exit 1 (0 matches) and `grep -n 'requires repository access\|no credential\|HTTPS token' README.md` -> exit 1 (0 matches). README.md:289-294 Prerequisites is still the bare '- Go 1.25+' with no tarball bootstrap, and README.md:298-301 Quick Start is still the bare '# Clone / git clone git@github.com:...' with no access note. The work exists only on an UNMERGED branch: `git branch -a --contains c2eff7c` -> '+ wt/DF-OFF-BY-ONE-11' (only) and `git merge-base --is-ancestor c2eff7c HEAD` -> NOT ancestor. Commit c2eff7c 'docs(readme): fresh-machine install bootstrap — Go tarball + clone access (DF-OFF-BY-ONE-11)' does contain exactly the required text (curl -fsSL https://go.dev/dl/go1.26.8.linux-amd64.tar.gz -o /tmp/go.tgz; mkdir -p ~/toolchain && tar -C ~/toolchain -xzf /tmp/go.tgz; export PATH="$HOME/toolchain/go/bin:$PATH"; go version; plus '# Clone — requires repository access (SSH key or HTTPS token for github.com:totalwindupflightsystems/off-by-one). A fresh box with no credential for that repo cannot clone it.'), and `git diff HEAD wt/DF-OFF-BY-ONE-11 -- README.md` shows those hunks as branch-only. The only graded worktree change for this task is .gitreins/tasks.yaml flipping status in_progress -> complete (worktree.patch 0818cc8e) — no README change landed on master. The 'existing green path unchanged' half is trivially satisfied (README.md:296-320 Quick Start is byte-identical to pre-task), but the primary requirement is absent. Test suite for completeness: `go test ./... -short -count=1 -p 1 -timeout 180s` -> EXIT=0, all 13 packages 'ok' (no test_command in .gitreins/config.yaml; guards.tests=false, go.tests=true).
The Go tarball bootstrap and clone-access note exist only on the unmerged branch wt/DF-OFF-BY-ONE-11 (commit c2eff7c); the graded master tree's README still carries the bare 'Go 1.25+' prerequisite and bare clone step, so the criterion fails.

Overall: FAIL ✗
