// Chat integration: *solver.Executor implements web.AgentRunner so the
// /ws/chat WebSocket handler can treat the solver as the chat agent.
//
// A chat turn behaves like a one-shot solve: the user's message is
// wrapped in a queue-style ingest.Entry under the synthetic "chat-turn"
// problem class and handed to Solve. The resulting solution markdown is
// streamed back as agent ChatMessages.
//
// Chat solutions are deliberately NOT persisted via Commit: chat output
// is ephemeral and unverified, and Commit marks answers verified, which
// would pollute the answer corpus with conversational replies.
package solver

import (
	"context"
	"fmt"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/web"
)

// Compile-time assertion: *Executor satisfies web.AgentRunner.
var _ web.AgentRunner = (*Executor)(nil)

// ChatProblemClass is the synthetic problem class used for chat turns.
// It keeps chat-originated solves distinguishable from real queued
// submissions in logs and pi-agent output.
const ChatProblemClass = "chat-turn"

// Run implements web.AgentRunner. The user's message becomes the
// Description of a one-shot ingest.Entry; Solve runs the usual
// sandbox + pi-agent cycle against it; the solution markdown is sent
// on outCh as a single agent message. Any solve failure is returned
// so the handler can surface its standard error message.
func (e *Executor) Run(ctx context.Context, userMessage string, outCh chan<- web.ChatMessage) error {
	entry := &ingest.Entry{
		ID:           fmt.Sprintf("chat-%d", time.Now().UnixNano()),
		ProblemClass: ChatProblemClass,
		Environment:  "chat",
		Description:  userMessage,
		Context:      map[string]any{"source": "chat", "interface": "ws"},
	}
	sol, err := e.Solve(ctx, entry)
	if err != nil {
		return err
	}
	msg := web.ChatMessage{Type: "agent", Message: sol.SolutionMarkdown}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case outCh <- msg:
		return nil
	}
}
