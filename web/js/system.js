// Off-by-One — System view: how the lab works, live.
//
// Shows the full pipeline with REAL numbers from the API (stats, queue,
// taxonomy): fleet ingest → queue → sandboxed solve → judge → publish.
// Includes the live queue window and the recent answers the fleet has
// produced — every row links back to the catalog so visitors can read
// the answer itself ("answers other people have already done").
//
// No build step: pure ES2020. Registered as window.initView_system.

(function () {
  'use strict';

  const STAGES = [
    { n: '1', title: 'Fleet submits problems', tool: 'Muster bridge', desc: 'Every project in the lab feeds hard engineering problems — Go, Rust, TS, Python, C++ — through the discover/submit bridge.', dot: 'g' },
    { n: '2', title: 'Ingest queue', tool: 'SQLite queue', desc: 'Submissions land in a persistent queue. The lab drains it whenever compute cycles are idle — it works while you sleep.', dot: 'b' },
    { n: '3', title: 'Sandboxed solve', tool: 'bwrap + pi-agent', desc: 'Each problem is solved by pi-agent inside a bubblewrap namespace: no network, no key leaks, no escape. Timeouts are strict.', dot: 'v' },
    { n: '4', title: 'Judge', tool: 'GitReins', desc: 'Every answer is executed and judged against the task\u2019s real criteria before it can ever be marked verified.', dot: 'p' },
    { n: '5', title: 'Published', tool: 'catalog + repo', desc: 'Verified answers ship with evidence and signatures — to the live catalog, the flat-file repo, and the static bot site.', dot: 'g' },
  ];

  function el(tag, cls, text) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function fmtDur(secs) {
    if (secs == null || isNaN(secs)) return '—';
    if (secs < 60) return Math.round(secs) + 's';
    const m = Math.floor(secs / 60);
    const s = Math.round(secs % 60);
    return m + 'm ' + s + 's';
  }

  function parseTs(ts) {
    // "2026-08-12 17:58:39" (local Bogota) — Date parses with a T.
    if (!ts) return null;
    return new Date(ts.replace(' ', 'T')).getTime();
  }

  function statusBadge(status) {
    const map = {
      complete: 'badge-verified',
      failed: 'badge-failed',
      in_progress: 'badge-pending',
      pending: 'badge-pending',
      queued: 'badge-pending',
    };
    const label = { complete: 'complete', failed: 'failed', in_progress: 'solving', pending: 'queued', queued: 'queued' };
    return el('span', 'badge ' + (map[status] || 'badge-pending'), label[status] || status);
  }

  function renderPipeline(root, stats) {
    const host = root.querySelector('.sys-stages');
    STAGES.forEach(function (s, i) {
      const card = el('div', 'sys-stage sys-stage-' + s.dot);
      card.appendChild(el('div', 'sys-stage-n', s.n));
      card.appendChild(el('div', 'sys-stage-t', s.title));
      card.appendChild(el('div', 'sys-stage-tool', s.tool));
      card.appendChild(el('div', 'sys-stage-d', s.desc));
      host.appendChild(card);
    });

    // Live numbers under each stage
    const nums = root.querySelector('.sys-live');
    const liveDefs = [
      ['sys-live-ingest', 'Fleet submissions', stats ? (stats.total_problems || 0).toLocaleString() : '—', 'total_problems'],
      ['sys-live-solver', 'Verified answers', stats ? (stats.total_answers || 0).toLocaleString() : '—', 'total_answers'],
      ['sys-live-judge', 'Hit rate', stats ? Math.round((stats.hit_rate || 0) * 100) + '%' : '—', null],
      ['sys-live-queue', 'Queue depth', stats ? (stats.queue_depth || 0).toLocaleString() : '—', 'queue_depth'],
    ];
    liveDefs.forEach(function (d) {
      const cell = el('div', 'sys-live-cell');
      const v = el('div', 'sys-live-val', d[2]);
      v.id = d[0];
      cell.appendChild(v);
      cell.appendChild(el('div', 'sys-live-lbl', d[1]));
      nums.appendChild(cell);
    });
  }

  function renderQueueWindow(root, entries) {
    const host = root.querySelector('.sys-queue-list');
    if (!entries.length) {
      host.appendChild(el('div', 'sys-empty', 'Queue is empty right now.'));
      return;
    }
    entries.slice(0, 12).forEach(function (e) {
      const row = el('div', 'sys-row', null);
      const title = el('a', 'sys-row-title', e.problem_class || e.submission_id);
      title.href = '#search?q=' + encodeURIComponent(e.problem_class || '');
      const meta = el('div', 'sys-row-meta');
      const dur = parseTs(e.completed_at) && parseTs(e.started_at)
        ? (parseTs(e.completed_at) - parseTs(e.started_at)) / 1000 : null;
      meta.appendChild(statusBadge(e.status));
      if (dur != null && e.status === 'complete') meta.appendChild(el('span', 'sys-dur', fmtDur(dur)));
      row.appendChild(title);
      row.appendChild(meta);
      host.appendChild(row);
    });
  }

  function renderRecentAnswers(root, entries) {
    const host = root.querySelector('.sys-recent-list');
    const done = entries.filter(function (e) { return e.status === 'complete' && e.completed_at; })
      .sort(function (a, b) { return parseTs(b.completed_at) - parseTs(a.completed_at); })
      .slice(0, 10);
    if (!done.length) {
      host.appendChild(el('div', 'sys-empty', 'No completed answers in the recent window.'));
      return;
    }
    done.forEach(function (e) {
      const dur = (parseTs(e.completed_at) - parseTs(e.started_at)) / 1000;
      const row = el('div', 'sys-row', null);
      const title = el('a', 'sys-row-title', e.problem_class || e.submission_id);
      title.href = '#search?q=' + encodeURIComponent(e.problem_class || '');
      const meta = el('div', 'sys-row-meta');
      meta.appendChild(el('span', 'sys-dur', fmtDur(dur)));
      meta.appendChild(el('span', 'sys-time', (e.completed_at || '').slice(5, 16)));
      row.appendChild(title);
      row.appendChild(meta);
      host.appendChild(row);
    });
  }

  function renderLanguages(root) {
    const host = root.querySelector('.sys-langs');
    fetch('/api/v1/taxonomy')
      .then(function (r) { return r.json(); })
      .then(function (d) {
        const counts = {};
        (d.tree || []).forEach(function (node) {
          (node.answers || []).forEach(function (a) {
            const l = a.lang || 'other';
            counts[l] = (counts[l] || 0) + 1;
          });
        });
        const total = Object.values(counts).reduce(function (a, b) { return a + b; }, 0);
        if (!total) throw new Error('no data');
        Object.entries(counts).sort(function (a, b) { return b[1] - a[1]; }).slice(0, 6).forEach(function (e) {
          const pct = Math.round((e[1] / total) * 100);
          const row = el('div', 'sys-lang-row');
          row.appendChild(el('span', 'sys-lang-name', e[0]));
          const bar = el('div', 'sys-lang-bar');
          const fill = el('div', 'sys-lang-fill');
          fill.style.width = pct + '%';
          bar.appendChild(fill);
          row.appendChild(bar);
          row.appendChild(el('span', 'sys-lang-pct', pct + '%'));
          host.appendChild(row);
        });
      })
      .catch(function () {
        host.appendChild(el('div', 'sys-empty', 'Coverage data unavailable.'));
      });
  }

  function initView_system(root) {
    if (!root) return;
    root.innerHTML = '';

    root.appendChild(el('h2', 'sys-title', 'How the lab works'));
    root.appendChild(el('p', 'sys-sub', 'Everything below is live from the running system — no mockups. Click any problem to read its answer.'));

    const pipe = el('div', 'sys-panel');
    pipe.appendChild(el('h3', 'sys-h3', 'The pipeline'));
    pipe.appendChild(el('div', 'sys-stages'));
    pipe.appendChild(el('div', 'sys-live'));
    root.appendChild(pipe);

    const cols = el('div', 'sys-cols');
    const qPanel = el('div', 'sys-panel');
    qPanel.appendChild(el('h3', 'sys-h3', 'Live queue window'));
    qPanel.appendChild(el('div', 'sys-queue-list'));
    cols.appendChild(qPanel);
    const rPanel = el('div', 'sys-panel');
    rPanel.appendChild(el('h3', 'sys-h3', 'Recent answers from the fleet'));
    rPanel.appendChild(el('p', 'sys-panel-sub', 'Answers people have already done — click to read.'));
    rPanel.appendChild(el('div', 'sys-recent-list'));
    cols.appendChild(rPanel);
    root.appendChild(cols);

    const langPanel = el('div', 'sys-panel');
    langPanel.appendChild(el('h3', 'sys-h3', 'Language coverage'));
    langPanel.appendChild(el('div', 'sys-langs'));
    root.appendChild(langPanel);

    // Data
    fetch('/api/v1/stats')
      .then(function (r) { return r.json(); })
      .then(function (st) { renderPipeline(root, st); })
      .catch(function () { renderPipeline(root, null); });
    fetch('/api/v1/queue')
      .then(function (r) { return r.json(); })
      .then(function (d) {
        const entries = (d.entries || []).filter(function (e) { return e.problem_class !== 'off-by-one-self-test'; });
        renderQueueWindow(root, entries);
        renderRecentAnswers(root, entries);
      })
      .catch(function () {
        root.querySelector('.sys-queue-list').appendChild(el('div', 'sys-empty', 'Queue unavailable.'));
      });
    renderLanguages(root);
  }

  window.initView_system = initView_system;
})();
