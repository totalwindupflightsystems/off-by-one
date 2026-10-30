# QA-OFF-BY-ONE-20-R3 verification

- Tested repository commit: `f0fd1474b29666e4de17be357a0d1672a6ad344c`
- Row: `QA-OFF-BY-ONE-20-R3`
- Classification: owner/shared-infrastructure verification; no source worker dispatched.

## Acceptance evidence

1. Shared harness inspection: `bash -n /home/kara/.hermes/scripts/bunker-qa.sh` passed. The required `OFF_BY_ONE_PORT`, `ui_probe`, or scratch-port override is absent. The harness currently uses a fixed `UI_PORTS` list and launches the Go arm without a per-run free-port override. Criterion 1: NOT MET.
2. Foreign-listener witness: `ss -ltnp | grep ':8766'` showed `off-by-one` PID `485475` bound to `127.0.0.1:8766`; `/health` returned `{"status":"ok","uptime":"16h48m41s","version":"v0.2.0-146-g8606af2c"}`. This is the expected contention premise, but no fresh bunker battery was run. Criterion 2: NOT MET.
3. Live product UI control: `GET /` on `127.0.0.1:8766` returned HTTP 200, `text/html; charset=utf-8`, 5260 bytes, and the SPA shell contained `home search submit explore system export import`. This proves the product surface, not the unrun harness arm. Criterion 3: NOT MET because no fresh `ui-probe.log` exists proving the battery avoided the bind error.

## Local evidence

- `git remote`: `git@github.com:totalwindupflightsystems/off-by-one.git`
- Local and `origin/master` were both `f0fd1474b29666e4de17be357a0d1672a6ad344c` before this evidence-only closeout.
- Latest exact-head CI before this closeout was successful for the corpus sync and pages runs; no recent failure was reported by `gh run list --repo totalwindupflightsystems/off-by-one --limit 3 --json status,conclusion,displayTitle,headBranch,createdAt`.

## Disposition

Keep the board row pending. The missing work is in the shared deployed harness `/home/kara/.hermes/scripts/bunker-qa.sh`, outside this repository and owner-controlled. Do not fabricate a code commit or a fresh-system battery result from the live product control.
