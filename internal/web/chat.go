// Package web — AI Agent Chat WebSocket handler.
//
// This file implements the /ws/chat endpoint described in WI-013 and
// system spec §4.2 (AI Agent Chat). The handler:
//
//  1. Upgrades the HTTP connection to a WebSocket.
//  2. Reads chat messages from the client (JSON: {message: "..."}).
//  3. Relays each message to an AgentRunner (the Pi Agent abstraction).
//  4. Streams the agent's response back to the client.
//
// The AgentRunner interface allows the server to talk to Pi Agent via
// bwrap in production while using a fake in tests. The runner has
// access to the graph Store so it can search for answers while chatting,
// and to the ingest Queue so it can suggest submitting a new problem.
//
// # Connection lifecycle (DF-OFF-BY-ONE-2)
//
// The chat loop is split into a read pump and a single main loop:
//
//   - readPump owns every c.Read call and uses NO read deadline. A client
//     that is silently waiting for a slow answer is indistinguishable from
//     a dead one at the frame level, so a read deadline must never be used
//     while a turn is in flight. Any read error (close frame, TCP reset,
//     network failure) cancels the shared context, which stops the running
//     agent turn — a genuine disconnect still cancels the runner.
//   - The main loop owns EVERY write to the connection (greeting, agent
//     messages, errors). A WebSocket conn supports one concurrent writer,
//     so funnelling all writes through one goroutine keeps that invariant.
//   - readTimeout now applies only to a connection that is idle with NO
//     turn in flight. While a turn runs the connection stays open for as
//     long as the solver needs (up to its own OB1_BWRAP_TIMEOUT).
//   - A ping loop covers liveness during long turns: a half-open TCP
//     connection (no FIN) is detected by a failed ping, which cancels the
//     in-flight turn instead of letting it run to the solver timeout.
//   - One turn at a time: a message that arrives while a turn is in flight
//     is rejected with {type:"error"} rather than queued or run
//     concurrently (concurrent turns would race on the single writer).
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
)

// minPingInterval floors the keepalive ping interval so tiny read
// timeouts (used in tests) cannot turn into a ping storm.
const minPingInterval = time.Second

// ChatMessage is the JSON shape exchanged over the WebSocket. The
// client sends {type:"user", message:"..."}; the server responds with
// {type:"agent", message:"...", actions:[...]}. The "actions" field
// carries structured suggestions the UI can render as buttons (e.g.
// "Submit as new problem", "View answer #42").
type ChatMessage struct {
	Type    string       `json:"type"`              // "user" | "agent" | "error" | "system"
	Message string       `json:"message,omitempty"` // the text content
	Actions []ChatAction `json:"actions,omitempty"` // optional action suggestions
	Context *ChatContext `json:"context,omitempty"` // optional structured context
}

// ChatAction is a structured suggestion the UI can render.
type ChatAction struct {
	Type  string `json:"type"`  // "submit" | "view_answer" | "search"
	Label string `json:"label"` // human-readable button text
	Data  any    `json:"data"`  // type-specific payload
}

// ChatContext carries optional structured data alongside a message —
// e.g., search results the agent found while answering.
type ChatContext struct {
	SearchHits []graph.SearchHit `json:"search_hits,omitempty"`
}

// AgentRunner abstracts the Pi Agent (or any LLM-backed agent) so the
// chat handler can be unit-tested without spawning bwrap. The runner
// receives a user message and a context that gives it access to the
// graph and queue.
//
// Run should send the agent's response to outCh. If the agent wants to
// suggest actions (submit, view answer), it sends them in the Actions
// field. Run blocks until the conversation turn is complete or ctx is
// cancelled. Returning an error sends an error message to the client.
type AgentRunner interface {
	Run(ctx context.Context, userMessage string, outCh chan<- ChatMessage) error
}

// AgentContext bundles the graph store and queue so the runner can
// search for answers and submit problems while chatting. Production
// runners use this to implement the "agent can search + suggest submit"
// behavior from the spec.
type AgentContext struct {
	Store *graph.Store
	Queue *ingest.Queue
}

// ChatHandler holds the WebSocket handler state.
type ChatHandler struct {
	runner AgentRunner
	// readTimeout caps how long we wait for a client message while NO
	// turn is in flight (an idle connection). It is never applied while
	// a turn is running — see the package comment.
	readTimeout time.Duration
	// writeTimeout caps how long we wait to send a message.
	writeTimeout time.Duration
}

// NewChatHandler builds a ChatHandler with the given runner. Pass nil
// to disable the chat (messages will get an "offline" response).
func NewChatHandler(runner AgentRunner) *ChatHandler {
	return &ChatHandler{
		runner:       runner,
		readTimeout:  30 * time.Second,
		writeTimeout: 10 * time.Second,
	}
}

// chatFrame is one raw frame read by the read pump.
type chatFrame struct {
	msgType websocket.MessageType
	data    []byte
}

// ServeHTTP upgrades to WebSocket and runs the chat loop.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only accept GET (the WebSocket upgrade method).
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow same-origin only. In production behind a reverse
		// proxy this is correct; for local dev the browser and
		// server are on the same origin.
		InsecureSkipVerify: false,
	})
	if err != nil {
		// websocket.Accept already wrote an error response.
		return
	}
	defer func() { _ = c.CloseNow() }()

	// ctx is cancelled when the client disconnects (the read pump sees
	// the read error), when a keepalive ping fails, or when this handler
	// returns. The read pump cancels it directly because neither
	// r.Context() (only cancels at handler return) nor a deferred cancel
	// (the handler must not return while a turn runs) is enough on its
	// own.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// The read pump owns every c.Read call: no read deadline, one read
	// error channel, and every read error cancels ctx.
	msgCh := make(chan chatFrame)
	errCh := make(chan error, 1)
	go readPump(ctx, cancel, c, msgCh, errCh)

	// Keepalive pings detect a dead connection while a long turn runs.
	go h.pingLoop(ctx, cancel, c)

	// Send a greeting so the client knows the connection is live. This
	// (and every later write) happens on this goroutine only.
	if err := h.send(ctx, c, ChatMessage{
		Type:    "system",
		Message: "Connected to Off-by-One AI Agent. Ask me about debugging problems.",
	}); err != nil {
		return
	}

	// idle is armed only while no turn is in flight: it closes a
	// connection whose client has gone away between turns, but never
	// interrupts a client that is waiting for an answer.
	idle := time.NewTimer(h.readTimeout)
	defer idle.Stop()

	// turnOut / turnDone are non-nil only while a turn is in flight.
	// A nil channel blocks forever in select, which is exactly what we
	// want for the "no turn running" case.
	var turnOut chan ChatMessage
	var turnDone chan struct{}

	for {
		var idleCh <-chan time.Time
		if turnDone == nil {
			idleCh = idle.C
		}

		select {
		case f := <-msgCh:
			if turnDone != nil {
				// One turn at a time (see package comment). Reject
				// rather than queue: the client gets an explicit
				// signal instead of a silently reordered answer.
				if f.msgType == websocket.MessageText {
					_ = h.send(ctx, c, ChatMessage{
						Type:    "error",
						Message: "A turn is already in progress. Wait for the current answer before asking again.",
					})
				}
				continue
			}
			// Client is alive: restart the idle window.
			resetTimer(idle, h.readTimeout)

			if f.msgType != websocket.MessageText {
				continue
			}

			var userMsg ChatMessage
			if err := json.Unmarshal(f.data, &userMsg); err != nil {
				_ = h.send(ctx, c, ChatMessage{Type: "error", Message: "invalid message format"})
				continue
			}

			// Start the turn. Disarm the idle timer: while a turn is in
			// flight the connection stays open for as long as the solver
			// needs (its own timeout is the only bound).
			stopTimer(idle)
			turnOut = make(chan ChatMessage)
			turnDone = make(chan struct{})
			um, out, done := userMsg, turnOut, turnDone
			go func() {
				h.runTurn(ctx, um, out)
				close(done)
			}()

		case msg := <-turnOut:
			// Agent output for the in-flight turn. All writes to the
			// WebSocket happen here (single writer).
			if err := h.send(ctx, c, msg); err != nil {
				// The client is gone or the write timed out; stop the
				// turn instead of letting it burn the solver budget.
				cancel()
				return
			}

		case <-turnDone:
			// Turn finished; back to the idle state.
			turnOut = nil
			turnDone = nil
			resetTimer(idle, h.readTimeout)

		case err := <-errCh:
			// Client disconnected (readPump already cancelled ctx).
			_ = err
			return

		case <-idleCh:
			// No message and no turn for readTimeout: the client is idle,
			// close the connection.
			log.Printf("chat: closing idle connection (no message for %s)", h.readTimeout)
			return

		case <-ctx.Done():
			return
		}
	}
}

// readPump owns every c.Read call for the connection. It deliberately
// uses NO read deadline: a client waiting for a slow agent answer is
// indistinguishable from a dead one at the frame level. Liveness is
// covered by pingLoop instead. Any read error means the client is gone
// (or ctx was cancelled), so the pump cancels the shared context —
// stopping an in-flight agent turn — and reports the error.
func readPump(
	ctx context.Context,
	cancel context.CancelFunc,
	c *websocket.Conn,
	msgCh chan<- chatFrame,
	errCh chan<- error,
) {
	for {
		msgType, data, err := c.Read(ctx)
		if err != nil {
			cancel()
			select {
			case errCh <- err:
			default:
			}
			return
		}
		select {
		case msgCh <- chatFrame{msgType: msgType, data: data}:
		case <-ctx.Done():
			return
		}
	}
}

// pingLoop pings the client periodically so a half-open TCP connection
// (client gone without a FIN — closed laptop, NAT rebind) is detected
// while a long turn is in flight. A failed ping cancels the shared
// context, which stops the running turn instead of leaving it to run to
// the solver timeout.
//
// coder/websocket's Ping does not read from the connection itself; it
// waits for a Reader call to consume the pong. The read pump is always
// blocked in Read while it is not delivering a frame, so the pong is
// always consumed.
func (h *ChatHandler) pingLoop(ctx context.Context, cancel context.CancelFunc, c *websocket.Conn) {
	interval := h.readTimeout / 2
	if interval < minPingInterval {
		interval = minPingInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, pcancel := context.WithTimeout(ctx, h.writeTimeout)
			err := c.Ping(pingCtx)
			pcancel()
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("chat: client not responding to ping: %v", err)
				}
				cancel()
				return
			}
		}
	}
}

// runTurn executes one agent turn and forwards the runner's messages to
// out. It never writes to the WebSocket itself: ServeHTTP's main loop
// owns every write, so only one goroutine ever writes to the connection.
func (h *ChatHandler) runTurn(ctx context.Context, userMsg ChatMessage, out chan<- ChatMessage) {
	if h.runner == nil {
		select {
		case out <- ChatMessage{
			Type:    "agent",
			Message: "AI Agent is offline in this build. Configure an AgentRunner to enable chat.",
		}:
		case <-ctx.Done():
		}
		return
	}

	// Relay to the agent runner. The runner sends responses on runnerOut.
	runnerOut := make(chan ChatMessage, 8)
	var runnerErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runnerErr = h.runner.Run(ctx, userMsg.Message, runnerOut)
		close(runnerOut)
	}()

	// Forward agent responses to out (and thus to the WebSocket).
	for msg := range runnerOut {
		select {
		case out <- msg:
		case <-ctx.Done():
			// Client gone — stop forwarding; the runner observes the
			// cancelled context itself.
			return
		}
	}

	wg.Wait()

	if runnerErr != nil && !errors.Is(runnerErr, context.Canceled) {
		log.Printf("chat: agent runner error: %v", runnerErr)
		select {
		case out <- ChatMessage{
			Type:    "error",
			Message: "Agent encountered an error. Please try again.",
		}:
		case <-ctx.Done():
		}
	}
}

// send writes a ChatMessage as JSON to the WebSocket with a write deadline.
func (h *ChatHandler) send(ctx context.Context, c *websocket.Conn, msg ChatMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal chat message: %w", err)
	}
	writeCtx, cancel := context.WithTimeout(ctx, h.writeTimeout)
	defer cancel()
	return c.Write(writeCtx, websocket.MessageText, data)
}

// resetTimer restarts an idle timer, draining a value that already fired.
func resetTimer(t *time.Timer, d time.Duration) {
	stopTimer(t)
	t.Reset(d)
}

// stopTimer stops an idle timer, draining a value that already fired.
func stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}
