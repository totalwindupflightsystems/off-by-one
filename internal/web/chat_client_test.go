package web

import (
	"io/fs"
	"strings"
	"testing"

	offbyweb "github.com/totalwindupflightsystems/off-by-one/web"
)

// chatJS reads the embedded chat sidebar client. The tests below are
// source-invariant tests: the repo has no JS test toolchain, so the
// client behaviors fixed in DF-OFF-BY-ONE-44 are pinned by asserting
// the structural facts of web/js/chat.js that implement them.
func chatJS(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(offbyweb.FS(), "js/chat.js")
	if err != nil {
		t.Fatalf("read embedded js/chat.js: %v", err)
	}
	return string(data)
}

// jsFuncBody extracts the body of a top-level `function name(...)`
// declaration from the IIFE in chat.js (functions are indented two
// spaces inside the IIFE; the next declaration at the same indent
// ends the body).
func jsFuncBody(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "function "+name+"(")
	if start < 0 {
		t.Fatalf("chat.js: function %s not found", name)
	}
	rest := src[start:]
	end := strings.Index(rest[1:], "\n  function ")
	if end < 0 {
		return rest
	}
	return rest[:end+1]
}

// TestChatJS_BannerReplacesNotAppends pins DF-OFF-BY-ONE-44 acceptance 1:
// the connect banner (type "system") must update the single status line
// in place instead of appending a new bubble per (re)connect.
func TestChatJS_BannerReplacesNotAppends(t *testing.T) {
	src := chatJS(t)

	// System messages are routed to the shared status line...
	body := jsFuncBody(t, src, "appendMessage")
	if !strings.Contains(body, "type === 'status' || type === 'system'") {
		t.Error("appendMessage must route 'system' banners through showStatus (replace), not append a bubble")
	}
	if strings.Contains(body, "chat-msg-' + type") && strings.Contains(body, "showStatus(text") {
		// good: system path never reaches the append-bubble code
	} else {
		t.Error("appendMessage must call showStatus for system/status types before the append path")
	}

	// ...and the status line itself is a single element updated in place.
	show := jsFuncBody(t, src, "showStatus")
	if !strings.Contains(show, "statusLine.textContent = text") {
		t.Error("showStatus must update statusLine.textContent in place")
	}
	if strings.Count(show, "createElement") != 1 {
		t.Error("showStatus must create at most one element (the shared status line)")
	}
	if !strings.Contains(show, "if (!statusLine || !statusLine.parentNode)") {
		t.Error("showStatus must reuse the existing statusLine element")
	}
}

// TestChatJS_UserEchoBeforeSocketGuard pins DF-OFF-BY-ONE-44 acceptance
// 2: send() must echo the user's message into the transcript BEFORE (and
// regardless of) the socket-state check, with a pending marker.
func TestChatJS_UserEchoBeforeSocketGuard(t *testing.T) {
	src := chatJS(t)
	body := jsFuncBody(t, src, "send")

	echoIdx := strings.Index(body, "appendMessage('user'")
	guardIdx := strings.Index(body, "readyState !== WebSocket.OPEN")
	openIdx := strings.Index(body, "readyState === WebSocket.OPEN")

	if echoIdx < 0 {
		t.Fatal("send() must echo the user message via appendMessage('user', ...)")
	}
	if guardIdx >= 0 && guardIdx < echoIdx {
		t.Error("send() must not gate the local echo on socket state (message drops silently during reconnect)")
	}
	if openIdx < 0 || openIdx < echoIdx {
		t.Error("send() must echo before the OPEN-check decides between ws.send and the pending queue")
	}
	if !strings.Contains(body, "markPending(") {
		t.Error("send() must mark the echoed bubble pending ('sending…' marker)")
	}
	if !strings.Contains(body, "pendingQueue.push(") {
		t.Error("send() must queue the text for delivery when the socket is not open")
	}

	// The marker resolves on agent/error frames and on final failure.
	appendBody := jsFuncBody(t, src, "appendMessage")
	if !strings.Contains(appendBody, "resolvePending(false)") {
		t.Error("agent/error frames must resolve pending user-message markers")
	}
	closeBody := jsFuncBody(t, src, "connect")
	if !strings.Contains(closeBody, "resolvePending(true") {
		t.Error("exhausted reconnects must mark pending echoes as not sent")
	}
	if !strings.Contains(closeBody, "pendingQueue.shift()") {
		t.Error("onopen must flush queued messages")
	}
}

// TestChatJS_ReconnectLoopGuard pins DF-OFF-BY-ONE-44 acceptance 3: the
// reconnect-loop trigger (server idle close after readTimeout=30s,
// observed as onclose 1006) is countered by a client keepalive that
// resets the server idle timer without starting an agent turn, plus
// duplicate-connection guards.
func TestChatJS_ReconnectLoopGuard(t *testing.T) {
	src := chatJS(t)

	// Keepalive exists and uses a BINARY frame (text frames would start
	// a bogus agent turn server-side).
	if !strings.Contains(src, "KEEPALIVE_MS") {
		t.Error("chat.js must define a keepalive interval (KEEPALIVE_MS) below the server's 30s idle timeout")
	}
	if !strings.Contains(src, "new Uint8Array(0)") {
		t.Error("keepalive must send an empty binary frame — a text frame would start an agent turn")
	}
	openBody := jsFuncBody(t, src, "connect")
	if !strings.Contains(openBody, "startKeepalive()") {
		t.Error("ws.onopen must start the keepalive")
	}
	if !strings.Contains(openBody, "stopKeepalive()") {
		t.Error("ws.onclose must stop the keepalive")
	}

	// onclose logs the close code for diagnosis.
	if !strings.Contains(openBody, "event.code") {
		t.Error("ws.onclose must log the close code (1006 = server idle close)")
	}

	// Duplicate-connection guards: connect() refuses while a socket is
	// open/connecting, and init() runs at most once.
	if !strings.Contains(openBody, "WebSocket.CONNECTING") {
		t.Error("connect() must refuse to open a second socket while one is open or connecting")
	}
	initBody := jsFuncBody(t, src, "init")
	if !strings.Contains(initBody, "if (initialized) return") {
		t.Error("init() must be idempotent (auto-init + app.js view activation)")
	}
}
