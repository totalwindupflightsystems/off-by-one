package web

import (
	"context"
	"encoding/json"
	"errors"
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

// readMsg reads one JSON ChatMessage from the WebSocket.
func readMsg(t *testing.T, c *websocket.Conn) ChatMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("websocket read: %v", err)
	}
	var msg ChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal message: %v (raw=%s)", err, string(data))
	}
	return msg
}

// TestChatHandler_OfflineResponse asserts that when no runner is set,
// the handler responds with an "offline" message.
func TestChatHandler_OfflineResponse(t *testing.T) {
	h := NewChatHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dialWS(t, strings.Replace(srv.URL, "http://", "ws://", 1))
	defer c.Close(websocket.StatusNormalClosure, "done")

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
	defer c.Close(websocket.StatusNormalClosure, "done")

	// Read greeting.
	_ = readMsg(t, c)

	// Send user message.
	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "Docker ownership error"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, userData); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read two agent responses.
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
	defer c.Close(websocket.StatusNormalClosure, "done")

	_ = readMsg(t, c) // greeting

	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "test"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, userData)

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
	defer c.Close(websocket.StatusNormalClosure, "done")

	_ = readMsg(t, c) // greeting

	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "hi"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, userData)

	// Partial response.
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
	defer c.Close(websocket.StatusNormalClosure, "done")

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
	defer c.Close(websocket.StatusNormalClosure, "done")

	_ = readMsg(t, c) // greeting

	// Send a binary message.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageBinary, []byte{0x00, 0x01, 0x02})

	// Now send a text message and verify we get a response — the
	// binary should have been ignored, not killed the connection.
	userData, _ := json.Marshal(ChatMessage{Type: "user", Message: "test"})
	_ = c.Write(ctx, websocket.MessageText, userData)

	resp := readMsg(t, c)
	if resp.Type != "agent" {
		t.Errorf("after binary, response type: got %q, want agent", resp.Type)
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
