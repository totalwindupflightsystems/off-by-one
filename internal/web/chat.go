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
	// readTimeout caps how long we wait for a client message.
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

	// Use a cancellable context so that when the client disconnects
	// (c.Read returns an error), we can cancel any in-flight runner.
	// r.Context() only cancels when this handler returns — but the
	// handler can't return until the runner finishes, so we need our
	// own cancellation trigger.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Send a greeting so the client knows the connection is live.
	if err := h.send(ctx, c, ChatMessage{
		Type:    "system",
		Message: "Connected to Off-by-One AI Agent. Ask me about debugging problems.",
	}); err != nil {
		return
	}

	for {
		// Read a message from the client.
		readCtx, readCancel := context.WithTimeout(ctx, h.readTimeout)
		msgType, data, err := c.Read(readCtx)
		readCancel()
		if err != nil {
			// Normal closure or timeout — just exit. The
			// deferred cancel will stop any in-flight runner.
			return
		}
		if msgType != websocket.MessageText {
			continue
		}

		var userMsg ChatMessage
		if err := json.Unmarshal(data, &userMsg); err != nil {
			_ = h.send(ctx, c, ChatMessage{Type: "error", Message: "invalid message format"})
			continue
		}

		// Handle the message in a goroutine so the read loop
		// stays responsive to client disconnects. When c.Read
		// errors (client gone), the loop returns and the
		// deferred cancel fires, cancelling any in-flight runner.
		go h.handleMessage(ctx, c, &userMsg)
	}
}

// handleMessage processes one user message: if no runner is configured,
// respond with "offline"; otherwise relay to the runner and stream
// responses back.
func (h *ChatHandler) handleMessage(ctx context.Context, c *websocket.Conn, userMsg *ChatMessage) {
	if h.runner == nil {
		_ = h.send(ctx, c, ChatMessage{
			Type:    "agent",
			Message: "AI Agent is offline in this build. Configure an AgentRunner to enable chat.",
		})
		return
	}

	// Relay to the agent runner. The runner sends responses on outCh.
	outCh := make(chan ChatMessage, 8)
	var runnerErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runnerErr = h.runner.Run(ctx, userMsg.Message, outCh)
		close(outCh)
	}()

	// Forward agent responses to the WebSocket.
	for msg := range outCh {
		if err := h.send(ctx, c, msg); err != nil {
			// Write failed — the goroutine running Run will see
			// ctx cancellation when ServeHTTP returns and the
			// deferred cancel fires. Just stop forwarding.
			return
		}
	}

	wg.Wait()

	if runnerErr != nil && !errors.Is(runnerErr, context.Canceled) {
		log.Printf("chat: agent runner error: %v", runnerErr)
		_ = h.send(ctx, c, ChatMessage{
			Type:    "error",
			Message: "Agent encountered an error. Please try again.",
		})
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
