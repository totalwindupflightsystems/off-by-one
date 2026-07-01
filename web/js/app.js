// Off-by-One — Web UI shell bootstrap.
//
// This file is loaded by every page view; it wires up the persistent
// shell (nav tabs, chat sidebar toggle, status-bar poller). Per-view
// content is mounted by per-view modules (search.js, submit.js, ...)
// which will be added in WI-009..WI-012. Those modules are loaded
// lazily when their view becomes active; if the module is not
// present we leave the placeholder text the server sent.
//
// No build step: pure ES2020, no transpilation, no framework.

(function () {
  'use strict';

  // ---------- Tab switching ----------
  const tabs = document.querySelectorAll('.nav-tab');
  const views = document.querySelectorAll('.view');

  function activateView(name) {
    tabs.forEach((t) => {
      const active = t.dataset.view === name;
      t.classList.toggle('active', active);
      t.setAttribute('aria-selected', active ? 'true' : 'false');
    });
    views.forEach((v) => {
      const active = v.id === 'view-' + name;
      v.classList.toggle('active', active);
      v.hidden = !active;
    });

    // Best-effort: invoke per-view module if it registered an init fn.
    const fnName = 'initView_' + name.replace(/[^a-z0-9_]/gi, '_');
    if (typeof window[fnName] === 'function') {
      try { window[fnName](document.getElementById('view-' + name)); }
      catch (e) { console.error('view init failed:', name, e); }
    }

    if (location.hash !== '#' + name) {
      history.replaceState(null, '', '#' + name);
    }
  }

  tabs.forEach((t) => {
    t.addEventListener('click', () => activateView(t.dataset.view));
  });

  // Open the view named in the URL hash, defaulting to search.
  const initial = (location.hash || '#search').slice(1);
  const valid = Array.from(tabs).some((t) => t.dataset.view === initial);
  activateView(valid ? initial : 'search');

  // ---------- Chat sidebar toggle ----------
  const chatToggle = document.getElementById('chat-toggle');
  if (chatToggle) {
    chatToggle.addEventListener('click', () => {
      const collapsed = document.body.classList.toggle('chat-collapsed');
      chatToggle.textContent = collapsed ? '»' : '«';
    });
  }

  // ---------- Status bar poller ----------
  const elQueue = document.getElementById('status-queue');
  const elCache = document.getElementById('status-cache');
  const elHitrate = document.getElementById('status-hitrate');
  const elVersion = document.getElementById('status-version');

  function fmtPct(x) {
    if (x === null || x === undefined || isNaN(x)) return '—';
    return (x * 100).toFixed(1) + '%';
  }
  function fmtInt(x) {
    if (x === null || x === undefined || isNaN(x)) return '—';
    return String(x);
  }

  async function refreshStatus() {
    try {
      const r = await fetch('/api/v1/stats', { headers: { 'Accept': 'application/json' } });
      if (!r.ok) throw new Error('HTTP ' + r.status);
      const j = await r.json();
      elQueue.textContent = fmtInt(j.queue_depth);
      elCache.textContent = fmtInt(j.total_answers);
      elHitrate.textContent = fmtPct(j.hit_rate);
    } catch (e) {
      elQueue.textContent = 'offline';
      elCache.textContent = 'offline';
      elHitrate.textContent = 'offline';
    }
  }

  // The version is in the X-Off-by-One-Version header, but we just
  // surface a static string for now — wired in via the health endpoint.
  fetch('/health', { headers: { 'Accept': 'application/json' } })
    .then((r) => r.ok ? r.json() : null)
    .then((j) => { if (j && j.version) elVersion.textContent = 'v' + j.version; })
    .catch(() => { /* status bar works without version */ });

  refreshStatus();
  setInterval(refreshStatus, 30_000);

  // ---------- Placeholder for per-view lazy loaders ----------
  // WI-009..WI-012 will define `window.initView_search`, etc. We
  // explicitly do not load them here — the server returns the same
  // index.html for every view, and per-view JS files will be added
  // to the bundle later.
})();
