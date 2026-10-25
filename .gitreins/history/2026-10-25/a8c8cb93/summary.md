# Verdict: INSTALL-OFF-BY-ONE-1

**Task:** Verify existing fresh-machine install evidence
**Evaluated:** 2026-10-25T05:11:17.656939
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Existing evidence file contains fresh-install=OK, ci-pass=OK, upgrade=OK, chaos-resource=PASS; no implementation change required.: Evidence file /tmp/bunker-qa-evidence-20261020T105356Z-822880.jsonl (cited at docs/dogfood/2026-10-20-deploy-truth-integration.md:160 and board row .coding-hermes/board/tasks.jsonl:240). Ran the board row's own acceptance command: `test -f /tmp/bunker-qa-evidence-20261020T105356Z-822880.jsonl && jq -e 'select(.cell=="fresh-install" and .status=="OK"), select(.cell=="ci-pass" and .status=="OK"), select(.cell=="upgrade" and .status=="OK"), select(.cell=="chaos-resource" and .status=="PASS")' ...` -> exit=0, printing all four rows: line 7 fresh-install OK, line 13 ci-pass OK ("native suite PASS"), line 15 upgrade OK ("previous release v0.1.0 -> HEAD reinstall clean"), line 21 chaos-resource PASS ("suite survives 3G memory cap"). No implementation change required: the task diff only adds the .gitreins/tasks.yaml entry, and the originating commit a2784353 (run 19) touched only docs/board/skill ("No code changed"). Independently corroborated the chaos-resource PASS: DF-51 fix a7ff40e6 added the ambient-cap skip; `bash -c 'ulimit -v 3145728; go test ./internal/sandbox/ -run TestApplyMemLimitMB_RoundTrip -count=1 -v'` -> "--- SKIP ... ambient RLIMIT_AS hard cap 3072 MiB < probe 4096 MiB / PASS / ok ... 0.003s", and the full suite under the same 3 GiB cap (`ulimit -v 3145728; go test ./... -short -count=1 -p 1`) -> exit=0, all 13 packages ok. (The in-repo docs/dogfood/evidence/*.jsonl files are different runs: 2026-10-20-battery-second-run has ci-pass=FAIL and 2026-10-23-battery has chaos-resource=INFO; the cited run-19 file is the one satisfying all four cells.)
The cited run-19 bunker evidence file contains all four required cells at the required statuses (verified by running the board row's jq acceptance command, exit=0), and no implementation change was made.

## Summary

Judge Result: INSTALL-OFF-BY-ONE-1

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Existing evidence file contains fresh-install=OK, ci-pass=OK, upgrade=OK, chaos-resource=PASS; no implementation change required.: Evidence file /tmp/bunker-qa-evidence-20261020T105356Z-822880.jsonl (cited at docs/dogfood/2026-10-20-deploy-truth-integration.md:160 and board row .coding-hermes/board/tasks.jsonl:240). Ran the board row's own acceptance command: `test -f /tmp/bunker-qa-evidence-20261020T105356Z-822880.jsonl && jq -e 'select(.cell=="fresh-install" and .status=="OK"), select(.cell=="ci-pass" and .status=="OK"), select(.cell=="upgrade" and .status=="OK"), select(.cell=="chaos-resource" and .status=="PASS")' ...` -> exit=0, printing all four rows: line 7 fresh-install OK, line 13 ci-pass OK ("native suite PASS"), line 15 upgrade OK ("previous release v0.1.0 -> HEAD reinstall clean"), line 21 chaos-resource PASS ("suite survives 3G memory cap"). No implementation change required: the task diff only adds the .gitreins/tasks.yaml entry, and the originating commit a2784353 (run 19) touched only docs/board/skill ("No code changed"). Independently corroborated the chaos-resource PASS: DF-51 fix a7ff40e6 added the ambient-cap skip; `bash -c 'ulimit -v 3145728; go test ./internal/sandbox/ -run TestApplyMemLimitMB_RoundTrip -count=1 -v'` -> "--- SKIP ... ambient RLIMIT_AS hard cap 3072 MiB < probe 4096 MiB / PASS / ok ... 0.003s", and the full suite under the same 3 GiB cap (`ulimit -v 3145728; go test ./... -short -count=1 -p 1`) -> exit=0, all 13 packages ok. (The in-repo docs/dogfood/evidence/*.jsonl files are different runs: 2026-10-20-battery-second-run has ci-pass=FAIL and 2026-10-23-battery has chaos-resource=INFO; the cited run-19 file is the one satisfying all four cells.)
The cited run-19 bunker evidence file contains all four required cells at the required statuses (verified by running the board row's jq acceptance command, exit=0), and no implementation change was made.

Overall: PASS ✓
