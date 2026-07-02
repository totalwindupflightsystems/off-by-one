#!/usr/bin/env bash
#
# connect-muster.sh — Start Off-by-One + Muster MCP bridge
#
# Usage:
#   ./scripts/connect-muster.sh              # Start everything
#   ./scripts/connect-muster.sh --dry-run    # Check prerequisites without starting
#   ./scripts/connect-muster.sh --check-only # Only verify Off-by-One is running
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVER_URL="${OFF_BY_ONE_URL:-http://localhost:8766}"
MUSTER_CONFIG="$PROJECT_DIR/muster-config.yaml"
DRY_RUN=false
CHECK_ONLY=false

# Parse args.
for arg in "$@"; do
    case "$arg" in
        --dry-run)     DRY_RUN=true ;;
        --check-only)  CHECK_ONLY=true ;;
        *) echo "Unknown flag: $arg" >&2; exit 1 ;;
    esac
done

echo "=== Off-by-One Muster Integration ==="
echo "Server URL: $SERVER_URL"
echo ""

# --- Step 1: Check if Off-by-One is running ---
echo "[1/4] Checking if Off-by-One server is running at $SERVER_URL ..."
HEALTH_RESPONSE=$(curl -sf -m 3 "$SERVER_URL/health" 2>/dev/null || echo "")
if [ -z "$HEALTH_RESPONSE" ]; then
    if [ "$CHECK_ONLY" = true ]; then
        echo "  ✗ Off-by-One server is NOT running"
        exit 1
    fi
    echo "  → Starting Off-by-One server ..."
    if [ "$DRY_RUN" = true ]; then
        echo "  (dry-run: would run: off-by-one --db $PROJECT_DIR/off-by-one.db)"
    else
        cd "$PROJECT_DIR"
        nohup off-by-one --db "$PROJECT_DIR/off-by-one.db" \
            > /tmp/off-by-one.log 2>&1 &
        echo $! > /tmp/off-by-one.pid
        echo "  ✓ Started (PID $(cat /tmp/off-by-one.pid))"
        sleep 2
    fi
else
    echo "  ✓ Off-by-One server is running"
fi

if [ "$CHECK_ONLY" = true ]; then
    exit 0
fi

# --- Step 2: Verify OpenAPI spec ---
echo ""
echo "[2/4] Verifying OpenAPI spec at $SERVER_URL/openapi.json ..."
SPEC_RESPONSE=$(curl -sf -m 5 "$SERVER_URL/openapi.json" 2>/dev/null || echo "")
if [ -z "$SPEC_RESPONSE" ]; then
    echo "  ✗ /openapi.json returned empty response"
    exit 1
fi
if ! echo "$SPEC_RESPONSE" | grep -q "operationId"; then
    echo "  ✗ Spec does not contain any operationId fields"
    exit 1
fi
echo "  ✓ OpenAPI spec is valid and contains operationId fields"

# --- Step 3: Check for required tools ---
echo ""
echo "[3/4] Checking for required Muster MCP tools ..."
for tool_id in submitProblem discoverSolution listProblems getQueueStatus; do
    if echo "$SPEC_RESPONSE" | grep -q "$tool_id"; then
        echo "  ✓ Found: $tool_id"
    else
        echo "  ✗ Missing: $tool_id"
        exit 1
    fi
done

# --- Step 4: Start Muster ---
echo ""
echo "[4/4] Starting Muster MCP server ..."
if ! command -v muster &>/dev/null; then
    if [ "$DRY_RUN" = true ]; then
        echo "  (dry-run: muster not installed — would need 'go install github.com/wojons/muster@latest')"
    else
        echo "  ⚠ Muster binary not found in PATH"
        echo "    Install with: go install github.com/wojons/muster@latest"
        echo "    Or use an existing Muster deployment pointing at: $SERVER_URL"
        echo ""
        echo "  The Off-by-One server is running and its OpenAPI spec is valid."
        echo "  Any MCP-compatible client can connect to it."
        exit 0
    fi
else
    if [ "$DRY_RUN" = true ]; then
        echo "  (dry-run: would run: muster serve --host $SERVER_URL --config $MUSTER_CONFIG)"
    else
        nohup muster serve --host "$SERVER_URL" --config "$MUSTER_CONFIG" \
            > /tmp/muster-mcp.log 2>&1 &
        echo $! > /tmp/muster-mcp.pid
        echo "  ✓ Muster started (PID $(cat /tmp/muster-mcp.pid))"
        sleep 2

        # Verify Muster is up.
        if curl -sf -m 3 "http://localhost:8767/health" >/dev/null 2>&1; then
            echo "  ✓ Muster health check passed"
        else
            echo "  ⚠ Muster health check failed (may still be starting up)"
        fi
    fi
fi

echo ""
echo "=== Integration Complete ==="
echo "Off-by-One API: $SERVER_URL"
echo "OpenAPI spec:   $SERVER_URL/openapi.json"
echo "MCP tools:      submit_problem, discover_solution, list_problems, get_queue_status"
