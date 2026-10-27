# README-8 — README health sweep re-evaluation

- UTC timestamp: 2026-10-27T02:50:13Z
- HEAD: 59232a1694a34030df22aa37d10f9c2b92fd770a (chore(gitreins): record README-5 judge artifact)
- Checkout: /home/kara/off-by-one (master)

## 1. Prerequisite row dispositions (boardctl)

`boardctl -C /home/kara/off-by-one show <row>` — exact relevant fields:

| Row | status | worker_status | completed_at | commit_hash | judge/guard |
|-----|--------|---------------|--------------|-------------|-------------|
| README-1-R2 | complete | completed | 2026-10-26T22:48:00Z | d6c52c269909e8771856c38faf4c48ab9daf8339 | guard Tier 1 DEGRADED PASS; CI: latest 3 runs successful |
| README-5 | complete | complete | 2026-10-27T02:10:00Z | e28f7689 | Tier 2 judge PASS (verdict ef1fed45); CI success |
| README-6 | complete | complete | 2026-10-27T01:03:32Z | 7a56435e | guard PASS; judge PASS 6a404326 |
| README-7 | complete | complete | 2026-10-27T01:31:21Z | 04164be2b735730da55f5e0dd3979b08b95d9dc0 | guard PASS; tier2 judge PASS 83526723 |

All four rows show `complete` — no superseding disposition needed.

## 2. Acceptance command reruns against this checkout

### README-1-R2 — CI job-count claim

Command: `grep -nE '^  [A-Za-z0-9_-]+:' .github/workflows/ci.yml`

Exact stdout (exit 0):

```
4:  push:
7:  pull_request:
11:  build:
86:  transport-retry-selftest:
104:  deploy-gate-selftest:
143:  binary-seed-probe:
180:  corpus-hygiene:
204:  golangci-lint:
```

Six job keys: build, transport-retry-selftest, deploy-gate-selftest, binary-seed-probe, corpus-hygiene, golangci-lint (lines 4 and 7 are trigger keys, not jobs).

README.md:549 (current) states: "six jobs in `.github/workflows/ci.yml` (`build`, `transport-retry-selftest`, `deploy-gate-selftest`, `binary-seed-probe`, `corpus-hygiene`, `golangci-lint`)". Matches all six job keys. PASS.

### README-5 — stale v0.1.x release qualifier

Command: `git grep -n -E 'v0\.1\.x' -- README.md`

Exact output: no matches; exit 1 (grep "no lines selected"). PASS.

Command: `git describe --tags --always`

Exact stdout (exit 0):

```
v0.2.0-88-g59232a16
```

Command: `git tag --sort=-creatordate | head -1`

Exact stdout (exit 0):

```
v0.2.0
```

The stale qualifier is gone; the replacement wording is version-agnostic and remains accurate against the current revision (v0.2.0-based). PASS.

### README-6 — Docker prerequisite for Muster integration tests

Command: `go test -count=1 ./internal/muster`

Exact stdout (exit 0):

```
ok  	github.com/totalwindupflightsystems/off-by-one/internal/muster	0.043s
```

Command: `git ls-files '*muster*' '*docker*'`

Exit 0. Muster-related entries: `internal/muster/bridge.go`, `internal/muster/bridge_test.go`, `internal/muster/e2e_test.go`, `muster-config.yaml`, `scripts/connect-muster.sh`. All `*docker*` matches are corpus answer/site data files (data/answers/*.json, site/classes/*.html). No Dockerfile and no docker-compose file exists in the repository.

README.md:395 region (Prerequisites) now lists Go 1.26.9+, optional bubblewrap, and optional pi-agent — the "Docker (for Muster integration tests)" line is removed. PASS.

### README-7 — repository traceability doctrine

Command: `test -f docs/traceability-doctrine.md`

Exit 0 (no output). File present: 61 lines, defining the repository's traceability contract (Purpose, Scope, Artifacts, Workflow, Exceptions). PASS.

## 3. Conclusion

All four prerequisite rows are complete, and every acceptance command reruns cleanly against the current checkout. **README-8: PASS.**
