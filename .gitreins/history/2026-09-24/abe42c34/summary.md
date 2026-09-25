# Verdict: OB-DF1318

**Task:** Unique tmp paths + connect-muster.sh honors SERVER_URL port
**Evaluated:** 2026-09-24T22:59:30.870248
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ README.md Go-tarball recipe uses a mktemp-based download path with zero /tmp/go.tgz occurrences; scripts/connect-muster.sh derives the server port from OFF_BY_ONE_URL (passes --port when starting a local server), skips local server start for non-local URLs, and reads the muster health probe from MUSTER_HEALTH_URL with no hardcoded 8767 outside the documented default: README.md:307 GO_TGZ="$(mktemp)" used by curl -o "$GO_TGZ" and tar -xzf "$GO_TGZ"; `grep -n '/tmp/go.tgz' README.md` returned 0 matches (exit 1) — remaining /tmp/go.tgz hits are only historical dogfood docs describing the fixed bug. scripts/connect-muster.sh:14 SERVER_URL="${OFF_BY_ONE_URL:-http://localhost:8766}"; :18-21 PORT=8766 then regex `:([0-9]+)(/|$)` overrides from URL; :60 `nohup off-by-one --db "$PROJECT_DIR/off-by-one.db" --port "$PORT"`; :24-27 IS_LOCAL case (localhost/127.0.0.1/::1) and :54-68 branch; :109-110 skips local Muster start when IS_LOCAL=false; :29 MUSTER_HEALTH_URL="${MUSTER_HEALTH_URL:-http://localhost:8767}" (documented default) and :134-137 probe `curl -sf -m 3 "$MUSTER_HEALTH_URL/health"`. Empirical: `bash -n` SYNTAX_OK; OFF_BY_ONE_URL=http://remote.example.com:9999 --dry-run printed 'is a remote host — not starting a local server'; OFF_BY_ONE_URL=http://localhost:9999 --dry-run printed 'would run: off-by-one --db ... --port 9999'. `grep -rn 8767 scripts/ README.md docs/` shows the sole script occurrence is the documented default at line 29. [resolution 0.04; README.md, scripts/connect-muster.sh]
All elements verified: README uses a mktemp download path with zero /tmp/go.tgz, and connect-muster.sh derives the port from OFF_BY_ONE_URL, passes --port for local starts, skips local starts for remote URLs, and probes health via MUSTER_HEALTH_URL with 8767 only as the documented default.

## Summary

Judge Result: OB-DF1318

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ README.md Go-tarball recipe uses a mktemp-based download path with zero /tmp/go.tgz occurrences; scripts/connect-muster.sh derives the server port from OFF_BY_ONE_URL (passes --port when starting a local server), skips local server start for non-local URLs, and reads the muster health probe from MUSTER_HEALTH_URL with no hardcoded 8767 outside the documented default: README.md:307 GO_TGZ="$(mktemp)" used by curl -o "$GO_TGZ" and tar -xzf "$GO_TGZ"; `grep -n '/tmp/go.tgz' README.md` returned 0 matches (exit 1) — remaining /tmp/go.tgz hits are only historical dogfood docs describing the fixed bug. scripts/connect-muster.sh:14 SERVER_URL="${OFF_BY_ONE_URL:-http://localhost:8766}"; :18-21 PORT=8766 then regex `:([0-9]+)(/|$)` overrides from URL; :60 `nohup off-by-one --db "$PROJECT_DIR/off-by-one.db" --port "$PORT"`; :24-27 IS_LOCAL case (localhost/127.0.0.1/::1) and :54-68 branch; :109-110 skips local Muster start when IS_LOCAL=false; :29 MUSTER_HEALTH_URL="${MUSTER_HEALTH_URL:-http://localhost:8767}" (documented default) and :134-137 probe `curl -sf -m 3 "$MUSTER_HEALTH_URL/health"`. Empirical: `bash -n` SYNTAX_OK; OFF_BY_ONE_URL=http://remote.example.com:9999 --dry-run printed 'is a remote host — not starting a local server'; OFF_BY_ONE_URL=http://localhost:9999 --dry-run printed 'would run: off-by-one --db ... --port 9999'. `grep -rn 8767 scripts/ README.md docs/` shows the sole script occurrence is the documented default at line 29. [resolution 0.04; README.md, scripts/connect-muster.sh]
All elements verified: README uses a mktemp download path with zero /tmp/go.tgz, and connect-muster.sh derives the port from OFF_BY_ONE_URL, passes --port for local starts, skips local starts for remote URLs, and probes health via MUSTER_HEALTH_URL with 8767 only as the documented default.

Overall: PASS ✓
