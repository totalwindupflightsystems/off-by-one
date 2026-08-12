// Package muster provides the bridge between Off-by-One and the Muster MCP
// server. Muster reads the OpenAPI spec at /openapi.json, auto-generates
// MCP tools (submit_problem, discover_solution, list_problems,
// get_queue_status, export_to_git, import_from_git, get_taxonomy,
// get_stats, get_related, list_queue), and relays calls between AI agents
// and the Off-by-One REST API.
//
// The Bridge validates that the spec is Muster-compatible, provides health
// check (is Muster connected? are tools live?), and logs tool calls for
// debugging.
package muster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Bridge validates the Off-by-One OpenAPI spec for Muster compatibility and
// monitors the Muster MCP connection. It is instantiated by the main binary
// at startup and can be queried for health status.
type Bridge struct {
	baseURL string // Off-by-One server URL (e.g. http://localhost:8766)
	client  *http.Client

	mu         sync.Mutex
	connected  bool
	lastCheck  time.Time
	toolCalls  int64      // total tool calls logged
	callLog    []ToolCall // ring buffer of recent calls
	maxCallLog int
}

// ToolCall is a logged Muster MCP tool invocation, stored in the bridge's
// ring buffer for debugging.
type ToolCall struct {
	Tool      string    `json:"tool"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	Status    int       `json:"status"`
}

// HealthResult is the structured output of Bridge.HealthCheck.
type HealthResult struct {
	ServerUp  bool     `json:"server_up"`
	SpecValid bool     `json:"spec_valid"`
	MusterUp  bool     `json:"muster_up"`
	Tools     []string `json:"tools,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// NewBridge creates a Bridge targeting the given Off-by-One server URL.
// The default timeout for HTTP probes is 5 seconds.
func NewBridge(baseURL string) *Bridge {
	return &Bridge{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		maxCallLog: 100,
	}
}

// musterToolNames is the full MCP tool surface Muster auto-generates from
// the OpenAPI spec. Keep in lockstep with pkg/api/openapi.yaml operationIds
// and the tools list in muster-config.yaml (see also the ten-tool check in
// ValidateSpec).
var musterToolNames = []string{
	"submit_problem", "discover_solution", "list_problems",
	"get_queue_status", "export_to_git", "import_from_git",
	"get_taxonomy", "get_stats", "get_related", "list_queue",
}

// ValidateSpec fetches the OpenAPI spec from the server and checks that it
// has the minimum structure Muster needs to generate MCP tools:
//   - Every path operation has an operationId
//   - POST operations have a requestBody with JSON content
//   - All ten Muster tools (operationIds) are present
//
// Returns nil if valid, or a descriptive error listing all issues found.
func (b *Bridge) ValidateSpec(ctx context.Context) error {
	spec, err := b.fetchSpec(ctx)
	if err != nil {
		return fmt.Errorf("fetch spec: %w", err)
	}
	return ValidateSpecDoc(spec)
}

// ValidateSpecDoc checks a parsed OpenAPI spec for Muster compatibility.
// Exported for unit testing without a running server.
func ValidateSpecDoc(raw map[string]any) error {
	var issues []string

	paths, ok := raw["paths"].(map[string]any)
	if !ok {
		return fmt.Errorf("spec has no 'paths' object")
	}

	if len(paths) == 0 {
		return fmt.Errorf("spec has no paths defined")
	}

	for path, pathVal := range paths {
		pathItem, ok := pathVal.(map[string]any)
		if !ok {
			issues = append(issues, fmt.Sprintf("path %s: not an object", path))
			continue
		}

		for method, opVal := range pathItem {
			op, ok := opVal.(map[string]any)
			if !ok {
				continue // might be a "parameters" or "$ref" key
			}

			// Only check HTTP methods.
			switch strings.ToUpper(method) {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
			default:
				continue
			}

			// operationId is required.
			opID, _ := op["operationId"].(string)
			if opID == "" {
				issues = append(issues,
					fmt.Sprintf("%s %s: missing operationId", method, path))
			}

			// POST/PUT must have a requestBody with application/json.
			if strings.ToUpper(method) == "POST" || strings.ToUpper(method) == "PUT" {
				rb, hasRB := op["requestBody"].(map[string]any)
				if !hasRB {
					issues = append(issues,
						fmt.Sprintf("%s %s: missing requestBody", method, path))
				} else {
					content, hasContent := rb["content"].(map[string]any)
					if !hasContent {
						issues = append(issues,
							fmt.Sprintf("%s %s: requestBody missing content", method, path))
					} else if _, hasJSON := content["application/json"]; !hasJSON {
						issues = append(issues,
							fmt.Sprintf("%s %s: requestBody missing application/json", method, path))
					}
				}
			}
		}
	}

	// Check for the ten Muster tools (operationIds). All must be present
	// in the spec so Muster can auto-generate the full MCP tool surface.
	requiredTools := map[string]bool{
		"submitProblem":    false,
		"discoverSolution": false,
		"listProblems":     false,
		"getQueueStatus":   false,
		"exportToGit":      false,
		"importFromGit":    false,
		"getTaxonomy":      false,
		"getStats":         false,
		"getRelated":       false,
		"listQueue":        false,
	}
	for _, pathVal := range paths {
		pathItem, ok := pathVal.(map[string]any)
		if !ok {
			continue
		}
		for _, opVal := range pathItem {
			op, ok := opVal.(map[string]any)
			if !ok {
				continue
			}
			opID, _ := op["operationId"].(string)
			if _, want := requiredTools[opID]; want {
				requiredTools[opID] = true
			}
		}
	}
	for tool, found := range requiredTools {
		if !found {
			issues = append(issues, fmt.Sprintf("missing required operationId: %s", tool))
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("spec validation failed:\n  - %s", strings.Join(issues, "\n  - "))
	}
	return nil
}

// fetchSpec downloads the OpenAPI spec from the server's /openapi.json endpoint.
func (b *Bridge) fetchSpec(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/openapi.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openapi.json returned %d", resp.StatusCode)
	}

	var spec map[string]any
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&spec); err != nil {
		// The server may serve YAML, not JSON. Re-fetch as YAML.
		return nil, fmt.Errorf("decode openapi.json (not valid JSON — server may serve YAML): %w", err)
	}
	return spec, nil
}

// HealthCheck probes both the Off-by-One server and (optionally) the Muster
// MCP server. The Muster server is optional — when not running, only
// serverUp and specValid are reported.
func (b *Bridge) HealthCheck(ctx context.Context) *HealthResult {
	result := &HealthResult{}

	// 1. Check Off-by-One server.
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/health", nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	resp, err := b.client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("server unreachable: %v", err)
		return result
	}
	resp.Body.Close()
	result.ServerUp = resp.StatusCode == 200

	// 2. Validate the spec.
	if result.ServerUp {
		if err := b.ValidateSpec(ctx); err != nil {
			result.Error = err.Error()
		} else {
			result.SpecValid = true
		}
	}

	// 3. Check if Muster is running (best-effort — Muster may be on a
	// different port or not running at all). We look for a Muster
	// health endpoint on port 8767.
	musterURL := strings.Replace(b.baseURL, "8766", "8767", 1)
	if musterURL == b.baseURL {
		// Can't derive Muster URL — skip.
		result.MusterUp = false
	} else {
		mReq, _ := http.NewRequestWithContext(ctx, "GET", musterURL+"/health", nil)
		mResp, mErr := b.client.Do(mReq)
		if mErr == nil {
			mResp.Body.Close()
			result.MusterUp = mResp.StatusCode == 200
		}
	}

	if result.MusterUp {
		result.Tools = musterToolNames
	}

	return result
}

// MarkConnected updates the connection status. Called by the main binary
// after the connect-muster script confirms Muster is up.
func (b *Bridge) MarkConnected(connected bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = connected
	b.lastCheck = time.Now()
}

// IsConnected returns whether Muster has been confirmed connected.
func (b *Bridge) IsConnected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connected
}

// LogToolCall records a tool invocation in the ring buffer. Used for
// debugging MCP tool calls from the AI agent side.
func (b *Bridge) LogToolCall(call ToolCall) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.toolCalls++
	if len(b.callLog) >= b.maxCallLog {
		b.callLog = b.callLog[1:]
	}
	b.callLog = append(b.callLog, call)
}

// RecentCalls returns up to n recent tool calls from the ring buffer.
func (b *Bridge) RecentCalls(n int) []ToolCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || n > len(b.callLog) {
		n = len(b.callLog)
	}
	start := len(b.callLog) - n
	out := make([]ToolCall, n)
	copy(out, b.callLog[start:])
	return out
}

// TotalCalls returns the total number of tool calls logged since startup.
func (b *Bridge) TotalCalls() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.toolCalls
}
