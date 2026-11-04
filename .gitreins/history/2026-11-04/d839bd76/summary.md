# Verdict: RELEASE-OB-018

**Task:** Deploy v0.3.0 build: rebuild the live binary with Go 1.26.10 and restart off-by-one.service
**Evaluated:** 2026-11-04T18:06:35.731171
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ ./off-by-one --version prints a stamp containing v0.3.0 and go version -m ./off-by-one reports Go go1.26.10 with vcs.modified=false; curl -s http://127.0.0.1:8766/health returns a version containing v0.3.0; curl -s -D- http://127.0.0.1:8766/metrics/prom returns HTTP 200 with Content-Type text/plain; version=0.0.4 and the body contains obo_http_up, obo_cron_loop_running, obo_solves_total, obo_queue_pending and obo_solve_duration_seconds; the JSON surface curl -s http://127.0.0.1:8766/metrics | jq -e .skipped_host_pressure still succeeds; make check-binary-fresh exits 0: All clauses verified live. `./off-by-one --version` -> `off-by-one v0.3.0-4-g4de41a27` (contains v0.3.0). `go version -m ./off-by-one` -> `./off-by-one: go1.26.10` plus `build vcs=git`, `vcs.revision=4de41a271a961d10c227624f98d32e5de1e4d8db`, `vcs.modified=false`. `curl -s http://127.0.0.1:8766/health` -> `{"status":"ok","uptime":"29s","version":"v0.3.0-4-g4de41a27"}`. `curl -s -D- http://127.0.0.1:8766/metrics/prom` -> `HTTP/1.1 200 OK` with `Content-Type: text/plain; version=0.0.4; charset=utf-8; escaping=underscores`; body contains `obo_http_up 1`, `obo_cron_loop_running 1`, `obo_queue_pending 40`, `obo_solves_total`, and `obo_solve_duration_seconds_bucket/...` (all 5 metric families confirmed via grep -oE '^obo_(http_up|cron_loop_running|solves_total|queue_pending|solve_duration_seconds)'). `curl -s http://127.0.0.1:8766/metrics | jq -e .skipped_host_pressure` -> `1`, jq exit 0. `make check-binary-fresh` -> `./off-by-one is up to date with source (version stamp changed, but code paths are unchanged)`, EXIT=0. Service is live: `systemctl status off-by-one.service` -> `Active: active (running) since Wed 2026-11-04 13:05:49 -05`, Main PID 2521243 running `/home/kara/off-by-one/off-by-one --port 8766 ...`, log shows `solver ready` and `cron loop started`.
Live deployment fully verified: v0.3.0 binary built with go1.26.10 (vcs.modified=false), service running, health/prom/JSON metrics endpoints all correct, and make check-binary-fresh exits 0.

## Summary

Judge Result: RELEASE-OB-018

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ ./off-by-one --version prints a stamp containing v0.3.0 and go version -m ./off-by-one reports Go go1.26.10 with vcs.modified=false; curl -s http://127.0.0.1:8766/health returns a version containing v0.3.0; curl -s -D- http://127.0.0.1:8766/metrics/prom returns HTTP 200 with Content-Type text/plain; version=0.0.4 and the body contains obo_http_up, obo_cron_loop_running, obo_solves_total, obo_queue_pending and obo_solve_duration_seconds; the JSON surface curl -s http://127.0.0.1:8766/metrics | jq -e .skipped_host_pressure still succeeds; make check-binary-fresh exits 0: All clauses verified live. `./off-by-one --version` -> `off-by-one v0.3.0-4-g4de41a27` (contains v0.3.0). `go version -m ./off-by-one` -> `./off-by-one: go1.26.10` plus `build vcs=git`, `vcs.revision=4de41a271a961d10c227624f98d32e5de1e4d8db`, `vcs.modified=false`. `curl -s http://127.0.0.1:8766/health` -> `{"status":"ok","uptime":"29s","version":"v0.3.0-4-g4de41a27"}`. `curl -s -D- http://127.0.0.1:8766/metrics/prom` -> `HTTP/1.1 200 OK` with `Content-Type: text/plain; version=0.0.4; charset=utf-8; escaping=underscores`; body contains `obo_http_up 1`, `obo_cron_loop_running 1`, `obo_queue_pending 40`, `obo_solves_total`, and `obo_solve_duration_seconds_bucket/...` (all 5 metric families confirmed via grep -oE '^obo_(http_up|cron_loop_running|solves_total|queue_pending|solve_duration_seconds)'). `curl -s http://127.0.0.1:8766/metrics | jq -e .skipped_host_pressure` -> `1`, jq exit 0. `make check-binary-fresh` -> `./off-by-one is up to date with source (version stamp changed, but code paths are unchanged)`, EXIT=0. Service is live: `systemctl status off-by-one.service` -> `Active: active (running) since Wed 2026-11-04 13:05:49 -05`, Main PID 2521243 running `/home/kara/off-by-one/off-by-one --port 8766 ...`, log shows `solver ready` and `cron loop started`.
Live deployment fully verified: v0.3.0 binary built with go1.26.10 (vcs.modified=false), service running, health/prom/JSON metrics endpoints all correct, and make check-binary-fresh exits 0.

Overall: PASS ✓
