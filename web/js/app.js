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

  // ---------- Problem pages (#/problem/<slug>) ----------
  // Not a nav tab: a shareable detail page rendered by problem.js.
  // Deactivates every tab and shows only #view-problem.
  function showProblemView(slug) {
    tabs.forEach((t) => {
      t.classList.remove('active');
      t.setAttribute('aria-selected', 'false');
    });
    views.forEach((v) => {
      const active = v.id === 'view-problem';
      v.classList.toggle('active', active);
      v.hidden = !active;
    });
    document.body.classList.remove('home-view');
    if (window.Ob1Problem) window.Ob1Problem.open(slug);
  }

  function currentProblemSlug() {
    const m = (location.hash || '').match(/^#\/problem\/(.+)$/);
    return m ? decodeURIComponent(m[1]) : null;
  }

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
    // The Home view is the dark immersive landing — flip the shell chrome.
    document.body.classList.toggle('home-view', name === 'home');

    // Best-effort: invoke per-view module if it registered an init fn.
    const fnName = 'initView_' + name.replace(/[^a-z0-9_]/gi, '_');
    if (typeof window[fnName] === 'function') {
      try { window[fnName](document.getElementById('view-' + name)); }
      catch (e) { console.error('view init failed:', name, e); }
    }

    // Keep any existing query params on the hash (e.g. #search?offset=40)
    // when the target view is already the active one — this preserves
    // shareable, paginated search URLs.
    const curHash = location.hash || '';
    const curView = curHash.split('?')[0].slice(1);
    const target = curView === name && curHash.indexOf('?') >= 0 ? curHash : '#' + name;
    if (location.hash !== target) {
      history.replaceState(null, '', target);
    }
  }

  tabs.forEach((t) => {
    t.addEventListener('click', () => activateView(t.dataset.view));
  });

  // Open the view named in the URL hash, defaulting to home (the hub
  // landing — catalog, repo links, docs). The hash may carry search
  // params (#search?limit=20&offset=40) — strip them for the view-name
  // comparison. A #/problem/<slug> hash opens the shareable problem page.
  const problemSlug = currentProblemSlug();
  if (problemSlug) {
    showProblemView(problemSlug);
  } else {
    const initial = (location.hash || '#home').slice(1).split('?')[0];
    const valid = Array.from(tabs).some((t) => t.dataset.view === initial);
    activateView(valid ? initial : 'home');
  }

  // Back/forward across problem pages: re-render on hash change. Leaving
  // a problem page restores the underlying tab view (the search view's
  // own hashchange listener re-reads its URL state).
  window.addEventListener('hashchange', () => {
    const slug = currentProblemSlug();
    if (slug) {
      showProblemView(slug);
      return;
    }
    const vp = document.getElementById('view-problem');
    if (vp) {
      vp.hidden = true;
      vp.classList.remove('active');
    }
    const viewName = (location.hash || '#home').slice(1).split('?')[0];
    const tab = Array.from(tabs).find((t) => t.dataset.view === viewName);
    if (tab) activateView(viewName);
  });

  // ---------- Theme toggle ----------
  const themeBtn = document.getElementById('theme-toggle');
  function paintThemeIcon() {
    if (!themeBtn) return;
    const dark = document.documentElement.getAttribute('data-theme') === 'dark';
    themeBtn.textContent = dark ? '☀️' : '🌙';
    themeBtn.title = dark ? 'Switch to light mode' : 'Switch to dark mode';
  }
  if (themeBtn) {
    themeBtn.addEventListener('click', () => {
      const next = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', next);
      try { localStorage.setItem('ob1-theme', next); } catch (e) { /* private mode */ }
      paintThemeIcon();
    });
  }
  paintThemeIcon();

  // ---------- Chat sidebar toggle ----------
  const chatToggle = document.getElementById('chat-toggle');
  if (chatToggle) {
    chatToggle.addEventListener('click', () => {
      const collapsed = document.body.classList.toggle('chat-collapsed');
      chatToggle.textContent = collapsed ? '»' : '«';
    });
  }

  // On phones/tablets the chat strip eats vertical space — start collapsed.
  if (window.matchMedia('(max-width: 1024px)').matches) {
    document.body.classList.add('chat-collapsed');
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
