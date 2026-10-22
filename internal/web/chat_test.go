package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// mockAgent is a fake AgentRunner for testing. It echoes a canned
// response (or a sequence of responses) and optionally returns an error.
type mockAgent struct {
	responses []ChatMessage
	err       error
	delay     time.Duration
}

func (m *mockAgent) Run(ctx context.Context, userMessage string, outCh chan<- ChatMessage) error {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.delay):
		}
	}
	for _, r := range m.responses {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case outCh <- r:
		}
	}
	return m.err
}

// testWaitBudget is the wall-clock budget for a single WebSocket frame read in
// this file. It is deliberately generous: every frame these tests wait for
// arrives in milliseconds, so the budget exists only to bound a broken handler,
// never to time the happy path. Do NOT tighten it toward the ~0.5s a passing
// run needs — the suite runs on hosts that are simultaneously building,
// testing, and serving other work, and a starved host can stall this process
// for seconds (DF-OFF-BY-ONE-56: a tier-1 gate lost the agent frame under a 5s
// budget while the same tree was green on two other runs).
const testWaitBudget = 15 * time.Second

// stallSignature is the substring the WebSocket client returns when its read
// context expires before the frame arrives. On a loaded host this is what a
// scheduling stall looks like (DF-OFF-BY-ONE-56), as opposed to a handler bug.
const stallSignature = "context deadline exceeded"

// dialWS connects to the test server and returns the WebSocket conn.
func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// readMsgErr reads one JSON ChatMessage from the WebSocket within
// testWaitBudget and returns the error instead of failing the test, so a
// caller can classify a host stall separately from a handler defect.
func readMsgErr(c *websocket.Conn) (ChatMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), testWaitBudget)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("websocket read: %w", err)
	}
	var msg ChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ChatMessage{}, fmt.Errorf("unmarshal message: %v (raw=%s)", err, string(data))
	}
	return msg, nil
}

// readMsg reads one JSON ChatMessage from the WebSocket, failing the test when
// the read budget is exhausted or the frame is unreadable.
func readMsg(t *testing.T, c *websocket.Conn) ChatMessage {
	t.Helper()
	msg, err := readMsgErr(c)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return msg
}

// isStall reports whether err is the read-deadline signature (host stall)
// rather than a closed connection, a protocol error, or an unexpected frame.
func isStall(err error) bool {
	return err != nil && strings.Contains(err.Error(), stallSignature)
}

// TestChatHandler_OfflineResponse asserts that when no runner is set,
// the handler responds with an "offline" message.
func TestChatHandler_OfflineResponse(t *testing.T) {
	h := NewChatHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	// First message is the system greeting.
	greeting := readMsg(t, c)
	if greeting.Type != "system" {
		t.Errorf("greeting type: got %q, want system", greeting.Type)
	}

	// Send a user message.
	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "hello"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, userData); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Expect an "offline" agent response.
	resp := readMsg(t, c)
	if resp.Type != "agent" {
		t.Errorf("response type: got %q, want agent", resp.Type)
	}
	if !strings.Contains(strings.ToLower(resp.Message), "offline") {
		t.Errorf("response should mention offline, got %q", resp.Message)
	}
}

// TestChatHandler_RelaysToRunner asserts the handler forwards user
// messages to the runner and streams the response back.
func TestChatHandler_RelaysToRunner(t *testing.T) {
	agent := &mockAgent{
		responses: []ChatMessage{
			{Type: "agent", Message: "I found a matching answer."},
			{Type: "agent", Message: "Here's the solution: ..."},
		},
	}
	h := NewChatHandler(agent)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	// Read greeting.
	_ = readMsg(t, c)

	// Send user message.
	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "Docker ownership error"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, userData); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read two agent responses. The immediate status frame
	// (DF-OFF-BY-ONE-16) precedes them.
	_ = readMsg(t, c)
	resp1 := readMsg(t, c)
	if resp1.Message != "I found a matching answer." {
		t.Errorf("response 1: got %q", resp1.Message)
	}
	resp2 := readMsg(t, c)
	if resp2.Message != "Here's the solution: ..." {
		t.Errorf("response 2: got %q", resp2.Message)
	}
}

// TestChatHandler_ActionsInResponse asserts the handler correctly
// serializes action suggestions from the runner.
func TestChatHandler_ActionsInResponse(t *testing.T) {
	agent := &mockAgent{
		responses: []ChatMessage{
			{
				Type:    "agent",
				Message: "This looks like a known issue.",
				Actions: []ChatAction{
					{Type: "submit", Label: "Submit as new problem", Data: map[string]any{"class": "docker-permission"}},
					{Type: "view_answer", Label: "View answer #42", Data: map[string]any{"id": 42}},
				},
			},
		},
	}
	h := NewChatHandler(agent)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c) // greeting

	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "test"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, userData)

	_ = readMsg(t, c) // immediate status frame (DF-OFF-BY-ONE-16)
	resp := readMsg(t, c)
	if len(resp.Actions) != 2 {
		t.Fatalf("actions: got %d, want 2", len(resp.Actions))
	}
	if resp.Actions[0].Type != "submit" {
		t.Errorf("action 0 type: got %q, want submit", resp.Actions[0].Type)
	}
	if resp.Actions[1].Label != "View answer #42" {
		t.Errorf("action 1 label: got %q", resp.Actions[1].Label)
	}
}

// TestChatHandler_RunnerError asserts that when the runner returns an
// error, the handler sends an error message to the client.
func TestChatHandler_RunnerError(t *testing.T) {
	agent := &mockAgent{
		responses: []ChatMessage{
			{Type: "agent", Message: "partial response"},
		},
		err: errors.New("agent crashed"),
	}
	h := NewChatHandler(agent)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c) // greeting

	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "hi"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, userData)

	// Immediate status frame (DF-OFF-BY-ONE-16), then the partial
	// response.
	_ = readMsg(t, c)
	partial := readMsg(t, c)
	if partial.Message != "partial response" {
		t.Errorf("partial: got %q", partial.Message)
	}

	// Error message.
	errMsg := readMsg(t, c)
	if errMsg.Type != "error" {
		t.Errorf("error type: got %q, want error", errMsg.Type)
	}
	if !strings.Contains(strings.ToLower(errMsg.Message), "error") {
		t.Errorf("error message should contain 'error', got %q", errMsg.Message)
	}
}

// TestChatHandler_InvalidJSON asserts malformed messages get an error
// response instead of closing the connection.
func TestChatHandler_InvalidJSON(t *testing.T) {
	h := NewChatHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c) // greeting

	// Send invalid JSON.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, []byte("{not valid json"))

	errMsg := readMsg(t, c)
	if errMsg.Type != "error" {
		t.Errorf("expected error for invalid JSON, got type %q", errMsg.Type)
	}
}

// TestChatHandler_BinaryMessageIgnored asserts binary messages are
// silently ignored (we only process text).
func TestChatHandler_BinaryMessageIgnored(t *testing.T) {
	h := NewChatHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c) // greeting

	// Send a binary message.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageBinary, []byte{0x00, 0x01, 0x02})

	// Now send a text message and verify we get a response — the
	// binary should have been ignored, not killed the connection.
	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "test"})
	_ = c.Write(ctx, websocket.MessageText, userData)

	// The nil-runner turn answers with the offline agent message and
	// sends NO status frame (progress requires a live agent).
	resp := readMsg(t, c)
	if resp.Type != "agent" {
		t.Errorf("after binary, response type: got %q, want agent", resp.Type)
	}
}

// TestChatHandler_LongTurnSurvivesReadTimeout is the regression test for
// DF-OFF-BY-ONE-2: a turn that takes longer than readTimeout must still
// deliver the agent's reply. Before the fix, the read loop's per-read
// deadline fired while the client waited for the answer, the loop
// returned, and the deferred cancel killed the in-flight runner.
func TestChatHandler_LongTurnSurvivesReadTimeout(t *testing.T) {
	const readTimeout = 100 * time.Millisecond
	const turnDuration = 5 * readTimeout // 500ms — well past the read timeout

	agent := &mockAgent{
		responses: []ChatMessage{{Type: "agent", Message: "slow but correct answer"}},
		delay:     turnDuration,
	}
	h := NewChatHandler(agent)
	h.readTimeout = readTimeout
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	// Send the message before reading the greeting so the tiny idle
	// window is irrelevant to the timing under test.
	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "why does docker say permission denied?"})
	writeCtx, writeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer writeCancel()
	if err := c.Write(writeCtx, websocket.MessageText, userData); err != nil {
		t.Fatalf("write: %v", err)
	}

	greeting := readMsg(t, c)
	if greeting.Type != "system" {
		t.Fatalf("greeting type: got %q, want system", greeting.Type)
	}

	// The reply must arrive even though the turn outlives readTimeout.
	// The immediate status frame may land first (DF-OFF-BY-ONE-16).
	skipStatus := func(m ChatMessage) ChatMessage {
		if m.Type == "status" {
			return readMsg(t, c)
		}
		return m
	}
	resp := skipStatus(readMsg(t, c))
	if resp.Type != "agent" {
		t.Fatalf("response type: got %q, want agent (message=%q)", resp.Type, resp.Message)
	}
	if resp.Message != "slow but correct answer" {
		t.Errorf("response: got %q, want %q", resp.Message, "slow but correct answer")
	}
}

// TestChatHandler_SecondMessageDuringTurnRejected asserts the explicit
// "one turn at a time" policy: a message that arrives while a turn is in
// flight is rejected with an error instead of running concurrently, and
// the in-flight turn still delivers its answer.
func TestChatHandler_SecondMessageDuringTurnRejected(t *testing.T) {
	agent := &mockAgent{
		responses: []ChatMessage{{Type: "agent", Message: "first answer"}},
		delay:     time.Second,
	}
	h := NewChatHandler(agent)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c) // greeting

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, _ := json.Marshal(ChatMessage{Type: "user", Message: "first"})
	if err := c.Write(ctx, websocket.MessageText, first); err != nil {
		t.Fatalf("write first: %v", err)
	}
	second, _ := json.Marshal(ChatMessage{Type: "user", Message: "second"})
	if err := c.Write(ctx, websocket.MessageText, second); err != nil {
		t.Fatalf("write second: %v", err)
	}

	// The second message is rejected while the first turn runs.
	reject := readMsg(t, c)
	// A status frame may have been sent first (DF-OFF-BY-ONE-16).
	if reject.Type == "status" {
		reject = readMsg(t, c)
	}
	if reject.Type != "error" {
		t.Fatalf("second message: got type %q, want error", reject.Type)
	}
	if !strings.Contains(strings.ToLower(reject.Message), "in progress") {
		t.Errorf("rejection should mention the in-flight turn, got %q", reject.Message)
	}

	// ... and the first turn still completes.
	resp := readMsg(t, c)
	if resp.Type == "status" {
		resp = readMsg(t, c)
	}
	if resp.Message != "first answer" {
		t.Errorf("first turn answer: got %q, want %q", resp.Message, "first answer")
	}
}

// TestChatHandler_IdleTimeoutClosesConnection asserts the readTimeout
// still applies to a connection that is idle with NO turn in flight.
func TestChatHandler_IdleTimeoutClosesConnection(t *testing.T) {
	h := NewChatHandler(nil)
	h.readTimeout = 200 * time.Millisecond
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.CloseNow() }()

	_ = readMsg(t, c) // greeting

	// No message is sent; the server should close the idle connection.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("expected the idle connection to be closed by the server")
	}
}

// TestChatHandler_ContextCancellation asserts that when the client
// disconnects mid-response, the runner's context is cancelled.
func TestChatHandler_ContextCancellation(t *testing.T) {
	cancelled := make(chan struct{})
	// Wrap to detect cancellation.
	h := NewChatHandler(&cancellingAgent{cancelled: cancelled})
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))

	_ = readMsg(t, c) // greeting

	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "test"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, userData)

	// Close the connection immediately to cancel the runner.
	_ = c.Close(websocket.StatusNormalClosure, "done")

	// Wait for the runner to observe cancellation.
	select {
	case <-cancelled:
		// Good — the runner saw ctx cancellation.
	case <-time.After(3 * time.Second):
		t.Fatal("runner was not cancelled when client disconnected")
	}
}

// cancellingAgent wraps mockAgent and signals when its context is
// cancelled.
type cancellingAgent struct {
	cancelled chan struct{}
}

func (c *cancellingAgent) Run(ctx context.Context, _ string, _ chan<- ChatMessage) error {
	<-ctx.Done()
	close(c.cancelled)
	return ctx.Err()
}

// TestChat_StatusFrameBeforeAgentFrame is the DF-OFF-BY-ONE-16
// regression test: the client must receive an immediate "status" frame
// when a turn starts, BEFORE any "agent" frame — a silent multi-minute
// wait is indistinguishable from a dead connection.
func TestChat_StatusFrameBeforeAgentFrame(t *testing.T) {
	agent := &mockAgent{
		responses: []ChatMessage{{Type: "agent", Message: "eventual answer"}},
		delay:     300 * time.Millisecond,
	}
	h := NewChatHandler(agent)
	h.statusInterval = time.Hour // no periodic frames: isolate the immediate one
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c) // greeting

	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "slow question"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, userData); err != nil {
		t.Fatalf("write: %v", err)
	}

	// First frame after the turn starts must be a status frame...
	status := readMsg(t, c)
	if status.Type != "status" {
		t.Fatalf("first frame after turn start: got type %q (message=%q), want status", status.Type, status.Message)
	}
	if status.Message == "" {
		t.Error("status frame must carry a message")
	}

	// ...and the agent frame must come after it.
	resp := readMsg(t, c)
	if resp.Type != "agent" {
		t.Fatalf("second frame: got type %q, want agent", resp.Type)
	}
	if resp.Message != "eventual answer" {
		t.Errorf("answer: got %q, want %q", resp.Message, "eventual answer")
	}
}

// TestChat_PeriodicStatusUntilAgentFrame asserts the periodic
// progress frames arrive while the agent is silent, and that NO status
// frame arrives after the first agent frame (the ticker is stopped at
// the first agent output, not left running into the next turn).
//
// Load tolerance (DF-OFF-BY-ONE-56): every wait here is bounded by
// testWaitBudget rather than a hand-picked few-second timeout, and the
// agent-frame wait is retried ONCE — on a fresh connection, and only when
// the failure is the read-deadline signature of a stalled host. A handler
// defect (error frame, closed connection, bad JSON) still fails on the
// first attempt, so the retry cannot mask a regression.
//
// ch:trace row=DF-OFF-BY-ONE-56 spec=internal/web/chat_test.go#TestChat_PeriodicStatusUntilAgentFrame test=same evidence=0a13957875d4730e81dc2d78dc5abf00f0c4d613
func TestChat_PeriodicStatusUntilAgentFrame(t *testing.T) {
	agent := &mockAgent{
		responses: []ChatMessage{{Type: "agent", Message: "finally"}},
		// Long enough for several periodic ticks; short enough to keep
		// the test fast. The ticker (100ms) will fire ~2-3 times.
		delay: 350 * time.Millisecond,
	}
	h := NewChatHandler(agent)
	h.statusInterval = 100 * time.Millisecond
	srv := httptest.NewServer(h)
	defer srv.Close()

	// runFirstTurn drives the delayed turn on a FRESH connection and
	// returns the frames collected up to the first agent frame. A fresh
	// connection per attempt is required: once a read context expires the
	// client library tears the socket down, so a retry cannot reuse it.
	runFirstTurn := func(t *testing.T) (sawStatus bool, agentMsg ChatMessage, err error) {
		t.Helper()
		c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
		defer func() { _ = c.CloseNow() }()

		if _, err := readMsgErr(c); err != nil { // greeting
			return false, ChatMessage{}, err
		}
		userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "long question"})
		writeCtx, writeCancel := context.WithTimeout(context.Background(), testWaitBudget)
		defer writeCancel()
		if err := c.Write(writeCtx, websocket.MessageText, userData); err != nil {
			return false, ChatMessage{}, fmt.Errorf("write: %w", err)
		}
		// Collect frames until the agent frame; require at least one
		// status frame in between (the immediate one counts).
		for {
			m, err := readMsgErr(c)
			if err != nil {
				return sawStatus, ChatMessage{}, err
			}
			switch m.Type {
			case "status":
				sawStatus = true
			case "agent":
				return sawStatus, m, nil
			case "error":
				// A product-level error frame is a defect, never a stall.
				return sawStatus, ChatMessage{}, fmt.Errorf("unexpected error frame: %q", m.Message)
			}
		}
	}

	// Bounded stall tolerance: one retry, deadline signature only.
	const maxTurnAttempts = 2
	var (
		sawStatus bool
		agentMsg  ChatMessage
		turnErr   error
	)
	for attempt := 1; attempt <= maxTurnAttempts; attempt++ {
		sawStatus, agentMsg, turnErr = runFirstTurn(t)
		if turnErr == nil {
			break
		}
		if !isStall(turnErr) {
			t.Fatalf("turn 1 (attempt %d): %v", attempt, turnErr)
		}
		if attempt == maxTurnAttempts {
			t.Fatalf("turn 1: agent frame not delivered in %d attempts with a %s read budget each — the handler is not producing frames: %v",
				maxTurnAttempts, testWaitBudget, turnErr)
		}
		t.Logf("turn 1 attempt %d stalled (%v) — retrying once on a fresh connection", attempt, turnErr)
	}
	if !sawStatus {
		t.Fatal("no status frame before the agent frame")
	}
	if agentMsg.Message != "finally" {
		t.Errorf("answer: got %q, want %q", agentMsg.Message, "finally")
	}

	// After the agent frame, a new silent-turn-free turn must NOT receive
	// any leftover periodic frame from the previous turn's ticker. This
	// turn answers instantly (delay only on the first run), so the frames
	// around it must be exactly: one immediate status, then the answer.
	// A SECOND status frame would be a stale periodic one leaking past
	// the first turn's agent frame.
	quick := &firstDelayAgent{response: "finally"}
	h2 := NewChatHandler(quick)
	h2.statusInterval = 100 * time.Millisecond
	srv2 := httptest.NewServer(h2)
	defer srv2.Close()

	c2 := dialWS(t, strings.Replace(srv2.URL, "http://", "ws://", 1))
	defer func() { _ = c2.Close(websocket.StatusNormalClosure, "done") }()

	_ = readMsg(t, c2) // greeting
	userData1, _ := json.Marshal(ChatMessage{Type: "user", Message: "long question"})
	writeCtx2, writeCancel2 := context.WithTimeout(context.Background(), testWaitBudget)
	defer writeCancel2()
	if err := c2.Write(writeCtx2, websocket.MessageText, userData1); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	// Turn 1 (delayed): drain statuses until the agent frame.
	for {
		m := readMsg(t, c2)
		if m.Type == "agent" {
			break
		}
		if m.Type != "status" {
			t.Fatalf("turn 1: unexpected frame type %q", m.Type)
		}
	}
	// Turn 2 (instant): exactly one status (immediate) then the answer.
	userData2, _ := json.Marshal(ChatMessage{Type: "user", Message: "quick question"})
	if err := c2.Write(writeCtx2, websocket.MessageText, userData2); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	first := readMsg(t, c2)
	second := readMsg(t, c2)
	if first.Type != "status" {
		t.Fatalf("turn 2 first frame: got %q, want status", first.Type)
	}
	if second.Type != "agent" || second.Message != "finally" {
		t.Errorf("turn 2 second frame: got %q/%q, want agent/finally", second.Type, second.Message)
	}
	// A third read must NOT yield another stale status frame — prove the
	// ticker from turn 1 (100ms interval) leaked nothing into turn 2 by
	// racing one extra read against a short timeout: an extra frame this
	// soon after an instant turn can only be a leak.
	extraCtx, extraCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer extraCancel()
	if _, _, err := c2.Read(extraCtx); err == nil {
		t.Error("unexpected extra frame after an instant turn — stale ticker frame leaked")
	}
}

// firstDelayAgent answers instantly except on its first Run, which waits
// firstDelay (for exercising periodic status frames).
type firstDelayAgent struct {
	response   string
	firstDelay time.Duration
	ran        bool
}

func (a *firstDelayAgent) Run(ctx context.Context, _ string, out chan<- ChatMessage) error {
	if a.firstDelay > 0 && !a.ran {
		a.ran = true
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(a.firstDelay):
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- ChatMessage{Type: "agent", Message: a.response}:
	}
	return nil
}
