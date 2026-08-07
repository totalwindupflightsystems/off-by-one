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

(function () {
  'use strict';

  const WS_PATH = '/ws/chat';
  const RECONNECT_DELAY = 3000;
  const MAX_RECONNECT = 5;

  let ws = null;
  let reconnectAttempts = 0;
  let chatBody = null;
  let chatInput = null;
  let chatSend = null;


  function appendMessage(type, text, actions) {
    if (!chatBody) return;

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

  function connect() {
    if (!chatBody) return;

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

    ws.onclose = function () {
      chatInput.disabled = true;
      chatSend.disabled = true;
      chatInput.placeholder = 'Reconnecting…';

      if (reconnectAttempts < MAX_RECONNECT) {
        reconnectAttempts++;
        setTimeout(connect, RECONNECT_DELAY);
      } else {
        appendMessage('system', 'Chat disconnected. Reload the page to reconnect.');
      }
    };
  }

  function send() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    var text = chatInput.value.trim();
    if (!text) return;
    appendMessage('user', text);
    ws.send(JSON.stringify({ type: 'user', message: text }));
    chatInput.value = '';
  }

  function init() {
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
