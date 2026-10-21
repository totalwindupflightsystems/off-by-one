# Verdict: QA-OFF-BY-ONE-28

**Task:** CI step pip install markdown is not runner-portable (PEP 668)
**Evaluated:** 2026-10-21T23:33:11.556132
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ The workflow's markdown install runs on PEP 668 runners: no bare 'pip install markdown' remains in .github/workflows, the site-redaction self-test still runs and passes, YAML parses, go build ./... passes, and the commit references QA-OFF-BY-ONE-28.: Commit b4997e5d changed .github/workflows/ci.yml:197 from `pip install markdown` to `python3 -m pip install --break-system-packages markdown` (PEP 668-safe). grep -rnE "(^|[^-])pip install markdown" .github/workflows/ returns no matches (exit 1); only remaining pip line is the --break-system-packages form. Self-test still invoked unchanged at ci.yml:198 `make site-redaction-selftest`; running it gave exit_code 0 with output "Ran 15 tests in 0.005s / OK". YAML parses: `python3 -c yaml.safe_load('.github/workflows/ci.yml')` exit 0, jobs = [build, transport-retry-selftest, deploy-gate-selftest, binary-seed-probe, corpus-hygiene, golangci-lint]. `go build ./...` exit_code 0, no output. Commit message subject: "fix(ci): make markdown install PEP 668-portable. Addresses QA-OFF-BY-ONE-28." (grep -c QA-OFF-BY-ONE-28 = 1).
The bare pip install was replaced with `python3 -m pip install --break-system-packages markdown`, the redaction self-test passes (15 tests OK), YAML parses, go build succeeds, and the commit references QA-OFF-BY-ONE-28.

## Summary

Judge Result: QA-OFF-BY-ONE-28

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ The workflow's markdown install runs on PEP 668 runners: no bare 'pip install markdown' remains in .github/workflows, the site-redaction self-test still runs and passes, YAML parses, go build ./... passes, and the commit references QA-OFF-BY-ONE-28.: Commit b4997e5d changed .github/workflows/ci.yml:197 from `pip install markdown` to `python3 -m pip install --break-system-packages markdown` (PEP 668-safe). grep -rnE "(^|[^-])pip install markdown" .github/workflows/ returns no matches (exit 1); only remaining pip line is the --break-system-packages form. Self-test still invoked unchanged at ci.yml:198 `make site-redaction-selftest`; running it gave exit_code 0 with output "Ran 15 tests in 0.005s / OK". YAML parses: `python3 -c yaml.safe_load('.github/workflows/ci.yml')` exit 0, jobs = [build, transport-retry-selftest, deploy-gate-selftest, binary-seed-probe, corpus-hygiene, golangci-lint]. `go build ./...` exit_code 0, no output. Commit message subject: "fix(ci): make markdown install PEP 668-portable. Addresses QA-OFF-BY-ONE-28." (grep -c QA-OFF-BY-ONE-28 = 1).
The bare pip install was replaced with `python3 -m pip install --break-system-packages markdown`, the redaction self-test passes (15 tests OK), YAML parses, go build succeeds, and the commit references QA-OFF-BY-ONE-28.

Overall: PASS ✓
