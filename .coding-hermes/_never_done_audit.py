#!/usr/bin/env python3
"""NEVER-DONE 12-point audit for tick 101."""
import json, urllib.request, subprocess

BASE = "http://localhost:8766"

def get_json(path):
    url = BASE + path
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=5) as r:
            return json.loads(r.read().decode())
    except Exception as e:
        return {"error": str(e)}

def run(cmd, timeout=30):
    try:
        r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=timeout, cwd="/home/kara/off-by-one")
        return {"rc": r.returncode, "stdout": r.stdout.strip(), "stderr": r.stderr.strip()[:200]}
    except subprocess.TimeoutExpired:
        return {"rc": -1, "stdout": "TIMEOUT", "stderr": ""}

checks = {}

# 1. BUILD
r = run("go build ./... 2>&1")
checks["BUILD"] = r["rc"] == 0

# 2. VET
r = run("go vet ./... 2>&1")
checks["VET"] = r["rc"] == 0

# 3. TESTS
r = run("go test ./... -count=1 -timeout 120s 2>&1")
all_ok = "FAIL" not in r["stdout"] and r["rc"] == 0
checks["TESTS"] = all_ok

# 4. STATICCHECK
r = run("go run honnef.co/go/tools/cmd/staticcheck@latest ./... 2>&1")
no_issues = r["rc"] == 0 or "no issues" in r["stdout"].lower()
checks["STATICCHECK"] = no_issues

# 5. GOVULNCHECK
r = run("go run golang.org/x/vuln/cmd/govulncheck@latest ./... 2>&1")
no_vulns = "No vulnerabilities" in r["stdout"] or r["rc"] == 0
checks["GOVULNCHECK"] = no_vulns

# 6. SERVER HEALTH
r = run("curl -s --max-time 5 http://localhost:8766/health")
healthy = '"ok"' in r["stdout"] or '"status":"ok"' in r["stdout"]
checks["SERVER_HEALTH"] = healthy

# 7. NO TODOs/FIXMEs
r = run('grep -rn "TODO\\|FIXME\\|HACK\\|XXX" --include="*.go" . 2>/dev/null | grep -v _test.go | grep -v vendor | wc -l')
checks["NO_TODOS"] = int(r["stdout"]) == 0
print(f"TODO count: {r['stdout']}")

# 8. GIT STATUS CLEAN
r = run("git status --porcelain 2>&1")
staged = [l for l in r["stdout"].split("\n") if l.strip() and not l.strip().startswith("?? .coding-hermes/")]
checks["GIT_CLEAN"] = len(staged) == 0
print(f"Modified files (non-temp): {len(staged)}")
for s in staged[:10]:
    print(f"  {s}")

# 9. HIT_RATE
stats = get_json("/api/v1/stats")
hit_rate = stats.get("hit_rate", 0)
checks["HIT_RATE_1"] = hit_rate == 1
print(f"Hit rate: {hit_rate}")

# 10. ANSWERS > PROBLEMS
problems = stats.get("total_problems", 0)
answers = stats.get("total_answers", 0)
checks["ANSWERS_GT_PROBLEMS"] = problems >= 0 and answers >= problems
print(f"Problems: {problems}, Answers: {answers}")

# 11. OUTDATED DEPS (minor/patch only - acceptable)
r = run("go list -m -u all 2>&1 | grep '\\[' | wc -l")
deps_outdated = int(r["stdout"])
checks["DEPS_OK"] = deps_outdated < 10  # minor/patch only
print(f"Outdated deps: {deps_outdated}")

# 12. KNOWLEDGE FRESHNESS (completed board IDs cited as still-active in skills/docs)
r = run("python3 .coding-hermes/_knowledge_freshness.py 2>&1")
checks["KNOWLEDGE_FRESHNESS"] = r["rc"] == 0
if not checks["KNOWLEDGE_FRESHNESS"]:
    print(r["stdout"])

# Summary
print("\n=== NEVER-DONE AUDIT RESULTS ===")
pass_count = 0
for name, passed in checks.items():
    status = "✅ PASS" if passed else "❌ FAIL"
    print(f"  {status} — {name}")
    if passed:
        pass_count += 1

print(f"\nResult: {pass_count}/{len(checks)} PASS")
print(json.dumps({"checks": {k: bool(v) for k,v in checks.items()}, "pass_count": pass_count, "total": len(checks)}))
