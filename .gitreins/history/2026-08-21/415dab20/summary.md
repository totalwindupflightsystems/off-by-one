# Verdict: OB-GAP-054

**Task:** Binary freshness gate trips on data-only commits
**Evaluated:** 2026-08-21T10:53:50.304914
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:51AM[0m [32mINF[0m [1mscanned ~5821438 bytes (5.82 MB) in 726ms[0m
[90m5:51AM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ PASS: (1) with the binary rebuilt from current HEAD (make build, HEAD includes the fix), make check-binary-fresh exits 0 despite the repo being at a data-only commit since the binary build; (2) a data-only content change (editing a data/answers/*.json file) does not trip the gate (exit 0); (3) a code-path content change (e.g. adding a comment line to cmd/off-by-one/main.go) trips the gate (exit 1, message 'run make build'), and reverting it restores exit 0; (4) go build ./... and go vet ./... and go test -short -count=1 ./... all pass; (5) gitreins guard 4/4 PASS (secrets/build/lint/tests); (6) no binary committed (gitignored).: (1) make build from HEAD → version 96b6926-dirty, make check-binary-fresh exits 0; after data-only commit f0fd6ec, gate still exits 0 ('version stamp changed, but code paths are unchanged'). (2) editing data/answers/0001-unknown.json → exit 0. (3) comment added to cmd/off-by-one/main.go → recipe exit 1 with 'run make build'; revert → exit 0. (4) go build ./... exit 0, go vet ./... exit 0, go test -short -count=1 ./... exit 0 (13 pkgs ok). (5) secrets grep clean, build/vet/tests all exit 0. (6) /off-by-one not tracked (git ls-files 0), gitignored (git check-ignore). Makefile:25 check-binary-fresh parses baked-in rev and diffs only code paths.
All 6 criteria verified PASS: the Makefile check-binary-fresh gate correctly ignores data-only commits while still tripping on code-path changes, and build/vet/test/secrets/guard all pass with the binary gitignored.

## Summary

Judge Result: OB-GAP-054

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:51AM[0m [32mINF[0m [1mscanned ~5821438 bytes (5.82 MB) in 726ms[0m
[90m5:51AM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ PASS: (1) with the binary rebuilt from current HEAD (make build, HEAD includes the fix), make check-binary-fresh exits 0 despite the repo being at a data-only commit since the binary build; (2) a data-only content change (editing a data/answers/*.json file) does not trip the gate (exit 0); (3) a code-path content change (e.g. adding a comment line to cmd/off-by-one/main.go) trips the gate (exit 1, message 'run make build'), and reverting it restores exit 0; (4) go build ./... and go vet ./... and go test -short -count=1 ./... all pass; (5) gitreins guard 4/4 PASS (secrets/build/lint/tests); (6) no binary committed (gitignored).: (1) make build from HEAD → version 96b6926-dirty, make check-binary-fresh exits 0; after data-only commit f0fd6ec, gate still exits 0 ('version stamp changed, but code paths are unchanged'). (2) editing data/answers/0001-unknown.json → exit 0. (3) comment added to cmd/off-by-one/main.go → recipe exit 1 with 'run make build'; revert → exit 0. (4) go build ./... exit 0, go vet ./... exit 0, go test -short -count=1 ./... exit 0 (13 pkgs ok). (5) secrets grep clean, build/vet/tests all exit 0. (6) /off-by-one not tracked (git ls-files 0), gitignored (git check-ignore). Makefile:25 check-binary-fresh parses baked-in rev and diffs only code paths.
All 6 criteria verified PASS: the Makefile check-binary-fresh gate correctly ignores data-only commits while still tripping on code-path changes, and build/vet/test/secrets/guard all pass with the binary gitignored.

Overall: PASS ✓
