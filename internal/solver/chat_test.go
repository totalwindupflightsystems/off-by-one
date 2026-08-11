package solver

import (
	"context"
	"strings"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/web"
)

// Compile-time assertion: *Executor must satisfy web.AgentRunner so
// main.go can wire it straight into web.NewChatHandler.
var _ web.AgentRunner = (*Executor)(nil)

// TestExecutor_Run_EmitsChatMessage proves a chat turn flows through
// Solve and the solution markdown is emitted as an agent ChatMessage
// on outCh.
func TestExecutor_Run_EmitsChatMessage(t *testing.T) {
	ex, _, _ := newTestSolver(t, "")

	outCh := make(chan web.ChatMessage, 1)
	if err := ex.Run(context.Background(), "how do I fix a docker permission error?", outCh); err != nil {
		t.Fatalf("Run: %v", err)
	}

	select {
	case msg := <-outCh:
		if msg.Type != "agent" {
			t.Errorf("message type: got %q, want agent", msg.Type)
		}
		// The default fake pi-agent writes "# Solution for <class>"
		// into solution.md — the streamed message must carry it.
		if !strings.Contains(msg.Message, "Solution for "+ChatProblemClass) {
			t.Errorf("message should contain solution markdown for %q, got %q", ChatProblemClass, msg.Message)
		}
	default:
		t.Fatal("no ChatMessage emitted on outCh")
	}
}

// TestExecutor_Run_SolveError proves a solve failure is returned as an
// error (the handler turns it into the standard error message) and no
// agent message is emitted.
func TestExecutor_Run_SolveError(t *testing.T) {
	ex, _, _ := newTestSolver(t, "")
	// A nonexistent pi-agent makes Solve fail with ErrPiAgentNotFound
	// before the runner is ever invoked.
	ex.cfg.PiAgentPath = "/nonexistent/pi-agent"

	outCh := make(chan web.ChatMessage, 1)
	err := ex.Run(context.Background(), "anything", outCh)
	if err == nil {
		t.Fatal("Run should return an error when Solve fails")
	}
	select {
	case msg := <-outCh:
		t.Fatalf("no message expected on solve failure, got %+v", msg)
	default:
	}
}
