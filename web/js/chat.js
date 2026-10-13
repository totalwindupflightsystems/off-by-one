// Off-by-One — AI Agent Chat (WI-013)
//
// Connects to the /ws/chat WebSocket endpoint and manages the chat
// sidebar UI. Messages are rendered as markdown (minimal inline renderer
// — no full markdown library; the server sends pre-formatted text from
// Pi Agent which is already markdown).
//
// No build step: pure ES2020, no framework. Registered as
// window.OffByOne_Chat so app.js can initialize it when the chat
// sidebar becomes active.
//
// Reconnect-loop root cause (DF-OFF-BY-ONE-44): the server closes a
// chat socket that has been idle for readTimeout (30s default,
// internal/web/chat.go) via CloseNow, so the client observes onclose
// code 1006. The old client treated EVERY close as a network failure
// and reconnected after 3s, and every reconnect appended a new
// "Connected to Off-by-One AI Agent" system banner — ~2 banners/minute
// of steady-state spam even on a perfectly healthy network. Fixes:
//   1. An empty BINARY keepalive frame every KEEPALIVE_MS resets the
//      server's idle timer without starting an agent turn (the server
//      read pump resets idle on any frame but only TEXT frames start
//      a turn), so a healthy socket never hits the idle close.
//   2. The connect banner is a single status line updated in place,
//      never appended per connect.
//   3. The user's own message is echoed locally on send regardless of
//      socket state, with a "sending…" marker that resolves on the
//      agent reply or on final reconnect failure.

(function () {
  'use strict';

  const WS_PATH = '/ws/chat';
  const RECONNECT_DELAY = 3000;
  const MAX_RECONNECT = 5;
  // Must stay below the server's idle readTimeout (30s). 20s leaves a
  // comfortable margin for one lost frame.
  const KEEPALIVE_MS = 20000;

  let ws = null;
  let reconnectAttempts = 0;
  let keepaliveTimer = null;
  let initialized = false;
  let chatBody = null;
  let chatInput = null;
  let chatSend = null;
  let statusLine = null;
  // Messages echoed locally but not yet answered by the agent, and
  // texts waiting for a reconnect before they can be sent.
  let pendingBubbles = [];
  let pendingQueue = [];

  // showStatus updates THE single status/banner line in place (creating
  // it on first use). System banners ("Connected…") and progress frames
  // share this line so reconnects and status updates never stack up.
  function showStatus(text, kind) {
    if (!chatBody) return;
    if (!statusLine || !statusLine.parentNode) {
      statusLine = document.createElement('div');
      chatBody.appendChild(statusLine);
    }
    statusLine.className = 'chat-msg chat-msg-' + (kind || 'status');
    statusLine.textContent = text;
    chatBody.scrollTop = chatBody.scrollHeight;
  }

  function clearStatus() {
    if (statusLine && statusLine.parentNode) {
      statusLine.parentNode.removeChild(statusLine);
    }
    statusLine = null;
  }

  function markPending(bubble) {
    if (!bubble) return;
    bubble.classList.add('chat-msg-pending');
    var marker = document.createElement('span');
    marker.className = 'chat-pending-marker';
    marker.textContent = ' · sending…';
    bubble.appendChild(marker);
    pendingBubbles.push(bubble);
  }

  // resolvePending clears the "sending…" marker on every echoed user
  // message once the agent (or an error frame) answers. With failed=true
  // the marker stays as a permanent "not sent" note (reconnects ran out).
  function resolvePending(failed, note) {
    pendingBubbles.forEach(function (b) {
      b.classList.remove('chat-msg-pending');
      var m = b.querySelector('.chat-pending-marker');
      if (!m) return;
      if (failed) {
        m.textContent = note || ' · not sent';
        m.classList.add('chat-pending-failed');
      } else {
        m.parentNode.removeChild(m);
      }
    });
    pendingBubbles = [];
  }

  function appendMessage(type, text, actions) {
    if (!chatBody) return null;
    if (type === 'status' || type === 'system') {
      // Progress frames (DF-OFF-BY-ONE-16) and system banners
      // (DF-OFF-BY-ONE-44) update the single status line in place
      // instead of stacking bubbles; the next real message replaces it.
      showStatus(text || (type === 'status' ? 'Working…' : ''), type);
      return null;
    }
    clearStatus();

    if (type === 'agent' || type === 'error') {
      // The agent answered (or failed to): every echoed user message
      // has its resolution, so clear the pending markers.
      resolvePending(false);
    }

    const bubble = document.createElement('div');
    bubble.className = 'chat-msg chat-msg-' + type;
    bubble.innerHTML = '';
    window.Ob1Markdown.renderInto(bubble, text || '');

    if (actions && actions.length > 0) {
      const actionRow = document.createElement('div');
      actionRow.className = 'chat-actions';
      actions.forEach(function (action) {
        const btn = document.createElement('button');
        btn.className = 'chat-action-btn';
        btn.textContent = action.label || action.type;
        btn.addEventListener('click', function () { handleAction(action); });
        actionRow.appendChild(btn);
      });
      bubble.appendChild(actionRow);
    }

    chatBody.appendChild(bubble);
    chatBody.scrollTop = chatBody.scrollHeight;
    return bubble;
  }

  function handleAction(action) {
    switch (action.type) {
      case 'submit':
        // Switch to the submit view and pre-fill the form.
        document.querySelector('[data-view="submit"]').click();
        setTimeout(function () {
          var ta = document.querySelector('#view-submit textarea[name="problem-class"]');
          if (ta && action.data && action.data.class) {
            ta.value = action.data.class;
            ta.dispatchEvent(new Event('input'));
          }
        }, 200);
        break;
      case 'view_answer':
        // Navigate to the search view with the answer ID.
        document.querySelector('[data-view="search"]').click();
        break;
      case 'search':
        // Trigger a search with the query.
        document.querySelector('[data-view="search"]').click();
        break;
      default:
        // Unknown action — ignore.
        break;
    }
  }

  function startKeepalive() {
    stopKeepalive();
    keepaliveTimer = setInterval(function () {
      if (ws && ws.readyState === WebSocket.OPEN) {
        // Empty BINARY frame: the server read pump resets its idle
        // timer on ANY frame but only TEXT frames start an agent turn,
        // so this keeps the socket alive without touching the agent
        // (DF-OFF-BY-ONE-44). A text keepalive would be unmarshalled
        // as a chat message and start a bogus turn.
        ws.send(new Uint8Array(0));
      }
    }, KEEPALIVE_MS);
  }

  function stopKeepalive() {
    if (keepaliveTimer) {
      clearInterval(keepaliveTimer);
      keepaliveTimer = null;
    }
  }

  function connect() {
    if (!chatBody) return;
    // Never open a second socket while one is live or still connecting —
    // a nudged reconnect (send during a reconnect phase) must not
    // double the connection.
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    var proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    var url = proto + '//' + window.location.host + WS_PATH;

    try {
      ws = new WebSocket(url);
    } catch (e) {
      appendMessage('system', 'WebSocket not supported in this browser.');
      return;
    }

    ws.onopen = function () {
      reconnectAttempts = 0;
      chatInput.disabled = false;
      chatSend.disabled = false;
      chatInput.placeholder = 'Ask the agent…';
      startKeepalive();
      // Flush messages the user typed while the socket was down. Their
      // bubbles stay pending until the agent replies.
      while (pendingQueue.length > 0) {
        ws.send(JSON.stringify({ type: 'user', message: pendingQueue.shift() }));
      }
    };

    ws.onmessage = function (event) {
      try {
        var msg = JSON.parse(event.data);
        appendMessage(msg.type, msg.message, msg.actions);
      } catch (e) {
        // Ignore malformed messages.
      }
    };

    ws.onerror = function () {
      appendMessage('system', 'Connection error.');
    };

    ws.onclose = function (event) {
      // Log the close code for diagnosis: 1006 = server dropped us
      // without a close frame (idle timeout uses CloseNow server-side);
      // with the keepalive armed a 1006 means a genuine network break.
      console.log('chat: websocket closed, code=' + event.code + ' reason=' + (event.reason || '(none)'));
      stopKeepalive();
      chatInput.disabled = true;
      chatSend.disabled = true;
      chatInput.placeholder = 'Reconnecting…';

      if (reconnectAttempts < MAX_RECONNECT) {
        reconnectAttempts++;
        setTimeout(connect, RECONNECT_DELAY);
      } else {
        appendMessage('system', 'Chat disconnected. Reload the page to reconnect.');
        // Nothing will deliver queued messages now: mark their echoes.
        resolvePending(true, ' · not sent — reconnect failed');
        pendingQueue = [];
      }
    };
  }

  function send() {
    if (!chatInput) return;
    var text = chatInput.value.trim();
    if (!text) return;
    chatInput.value = '';

    // Echo the user's own message into the transcript FIRST, regardless
    // of socket state, with a pending marker (DF-OFF-BY-ONE-44). Before
    // this fix the socket-state guard ran first and the message was
    // silently dropped during a reconnect phase.
    var bubble = appendMessage('user', text);
    markPending(bubble);

    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'user', message: text }));
    } else {
      // Socket mid-reconnect (or down): queue for delivery on the next
      // onopen and nudge a reconnect in case none is scheduled.
      pendingQueue.push(text);
      showStatus('Reconnecting — your message will be sent when the connection returns…', 'status');
      connect();
    }
  }

  function init() {
    // Re-entry guard: the auto-init below AND an app.js view activation
    // must never open two sockets or double the event listeners.
    if (initialized) return;
    initialized = true;

    chatBody = document.getElementById('chat-body');
    chatInput = document.getElementById('chat-input');
    chatSend = document.getElementById('chat-send');

    if (!chatBody) return;

    // Clear the "offline" placeholder.
    chatBody.innerHTML = '';

    // Enter to send, Shift+Enter for newline.
    chatInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    });

    chatSend.addEventListener('click', send);

    // Read-only catalog instances disable the AI agent — check the
    // stats endpoint before opening the WebSocket so the panel shows
    // a clear message instead of reconnecting forever.
    fetch('/api/v1/stats')
      .then(function (r) { return r.json(); })
      .then(function (st) {
        if (st && st.readonly) {
          chatInput.disabled = true;
          chatSend.disabled = true;
          chatInput.placeholder = 'Chat disabled on this catalog';
          appendMessage('system', 'The AI agent is disabled on this read-only catalog instance — the corpus is fully searchable.');
          // Dead panel is a distraction: start the sidebar collapsed on
          // read-only catalogs (desktop included). The toggle button
          // still works for anyone who wants to peek at the message.
          document.body.classList.add('chat-collapsed');
          var toggleBtn = document.getElementById('chat-toggle');
          if (toggleBtn) toggleBtn.textContent = '»';
          return;
        }
        connect();
      })
      .catch(function () { connect(); });
  }

  // Export for app.js to call on startup.
  window.OffByOne_Chat = { init: init };

  // Auto-init when the DOM is ready.
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
