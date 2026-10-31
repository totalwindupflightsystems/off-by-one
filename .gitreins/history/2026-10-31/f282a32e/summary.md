# Verdict: QA-OFF-BY-ONE-20-R3

**Task:** bunker-qa ui-probe scratch-port arm (port contention)
**Evaluated:** 2026-10-31T21:24:27.712815
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ PASS when: (1) grep -nE 'OFF_BY_ONE_PORT|ui_probe|scratch' /home/kara/.hermes/scripts/bunker-qa.sh shows the ui-probe go-native leg starting the tested server on a free loopback port override when the declared default port is already bound; (2) bash -n /home/kara/.hermes/scripts/bunker-qa.sh exits 0 and the rendered qa-run.sh syntax-checks clean; (3) dry-run with a foreign listener holding 127.0.0.1:8766: the arm starts the off-by-one server on a scratch port and the accepted run log contains no 'bind: address already in use' line while the pre-edit arm reproduces the bind-death INFO grade. Board row QA-OFF-BY-ONE-20-R3 criteria (2)/(3) (fresh-system battery witness) remain open and are out of this task scope.: Sub-conditions (1) and (2) hold: grep -nE 'OFF_BY_ONE_PORT|ui_probe|scratch' /home/kara/.hermes/scripts/bunker-qa.sh shows the arm (2728 ui_probe_scratch_start(), 2737 `setsid timeout 45 env OFF_BY_ONE_PORT=$UI_SCRATCH_PORT`, 2757-2766 scratch port picked when UI_OCCUPIED != ' ', 2789 pre-start call, 2822 recovery after 'address already in use'), and bash -n exits 0 with the rendered qa-run.sh (lines 1495-1589) syntax-checking clean. Sub-condition (3) FAILS: the arm is unreachable for the tested repo. The go-native ui-probe leg is gated at bunker-qa.sh:2672 on `elif [ -f go.mod ] && ui_go_http_source`, and ui_go_http_source greps for 'net/http' in *.go; off-by-one's HTTP server lives in cmd/off-by-one/main.go importing internal/api + internal/web, so `grep -rqF -e 'net/http' --include='*.go' .` in /home/kara/off-by-one returns no match (exit 1). The whole go-native leg — including the new scratch-port arm — never runs for off-by-one; the cell falls through to the N/A branch at 2864-2866, so no dry-run can show the arm starting the server on a scratch port. Additionally the detected start command is `go run ./cmd/oby-memcap` (alphabetically first cmd/ main), which is a memcap wrapper that prints 'usage: oby-memcap <command> [args...]' and exits 2 (verified by running /tmp/oby-memcap-test), so even if the leg ran the first UI_B would not be a server. The claimed end-to-end dry-run witness therefore cannot be reproduced.
The scratch-port code exists and syntax-checks, but the go-native ui-probe leg is gated behind a net/http grep that off-by-one fails, making the new arm unreachable and the required dry-run witness unattainable.

## Summary

Judge Result: QA-OFF-BY-ONE-20-R3

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ PASS when: (1) grep -nE 'OFF_BY_ONE_PORT|ui_probe|scratch' /home/kara/.hermes/scripts/bunker-qa.sh shows the ui-probe go-native leg starting the tested server on a free loopback port override when the declared default port is already bound; (2) bash -n /home/kara/.hermes/scripts/bunker-qa.sh exits 0 and the rendered qa-run.sh syntax-checks clean; (3) dry-run with a foreign listener holding 127.0.0.1:8766: the arm starts the off-by-one server on a scratch port and the accepted run log contains no 'bind: address already in use' line while the pre-edit arm reproduces the bind-death INFO grade. Board row QA-OFF-BY-ONE-20-R3 criteria (2)/(3) (fresh-system battery witness) remain open and are out of this task scope.: Sub-conditions (1) and (2) hold: grep -nE 'OFF_BY_ONE_PORT|ui_probe|scratch' /home/kara/.hermes/scripts/bunker-qa.sh shows the arm (2728 ui_probe_scratch_start(), 2737 `setsid timeout 45 env OFF_BY_ONE_PORT=$UI_SCRATCH_PORT`, 2757-2766 scratch port picked when UI_OCCUPIED != ' ', 2789 pre-start call, 2822 recovery after 'address already in use'), and bash -n exits 0 with the rendered qa-run.sh (lines 1495-1589) syntax-checking clean. Sub-condition (3) FAILS: the arm is unreachable for the tested repo. The go-native ui-probe leg is gated at bunker-qa.sh:2672 on `elif [ -f go.mod ] && ui_go_http_source`, and ui_go_http_source greps for 'net/http' in *.go; off-by-one's HTTP server lives in cmd/off-by-one/main.go importing internal/api + internal/web, so `grep -rqF -e 'net/http' --include='*.go' .` in /home/kara/off-by-one returns no match (exit 1). The whole go-native leg — including the new scratch-port arm — never runs for off-by-one; the cell falls through to the N/A branch at 2864-2866, so no dry-run can show the arm starting the server on a scratch port. Additionally the detected start command is `go run ./cmd/oby-memcap` (alphabetically first cmd/ main), which is a memcap wrapper that prints 'usage: oby-memcap <command> [args...]' and exits 2 (verified by running /tmp/oby-memcap-test), so even if the leg ran the first UI_B would not be a server. The claimed end-to-end dry-run witness therefore cannot be reproduced.
The scratch-port code exists and syntax-checks, but the go-native ui-probe leg is gated behind a net/http grep that off-by-one fails, making the new arm unreachable and the required dry-run witness unattainable.

Overall: FAIL ✗
