package solver

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// jsConcat joins adjacent single-quoted string literals so the contract
// phrases can be asserted as they read, not as the source happens to wrap
// them (`'... the maximum ' + 'number of ...'`). Re-wrapping a line in
// the wrapper must not make this test fail.
var jsConcat = regexp.MustCompile(`'\s*\+\s*'`)

// TestPiAgentWrapperDeclaresResourceBounds pins DF-OFF-BY-ONE-31
// acceptance (c): the solve contract the shipped wrapper hands to
// pi-agent (scripts/pi-agent) requires simulation-style solutions to
// declare explicit bounds and to aggregate incrementally instead of
// retaining a record per event.
//
// The prompt is built by the wrapper, not by Go code, so a regression
// there would silently drop the contract while every Go test stayed
// green. This test reads the shipped file and asserts the marker phrases
// so the reminder cannot be edited away unnoticed. It is deliberately a
// substring check on the source (not an execution of the wrapper): the
// wrapper needs a real pi binary and provider keys, neither of which a
// unit test can supply.
func TestPiAgentWrapperDeclaresResourceBounds(t *testing.T) {
	wrapperPath := filepath.Join("..", "..", "scripts", "pi-agent")
	raw, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("read %s: %v", wrapperPath, err)
	}
	body := jsConcat.ReplaceAllString(string(raw), "")

	for _, want := range []string{
		"## Resource bounds (required for simulation-style problems)",
		"declare explicit bounds",
		"maximum number of simulated events/packets/seconds",
		"aggregate the result incrementally",
		"rather than retaining a record per event",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scripts/pi-agent solve prompt is missing the resource-bounds contract phrase %q", want)
		}
	}
}
