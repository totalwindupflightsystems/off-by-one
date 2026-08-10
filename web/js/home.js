// Off-by-One — Home / hub landing view (bento hybrid: dark lab × color pop).
//
// ob1.it.com is the community front door: the catalog AND the way
// people find the repo and all the other details. This view renders
// the hero, LIVE data tiles (stats, search preview, language coverage
// all fetched from the real API), the pipeline, and link cards to the
// GitHub repo, docs, catalog files, and contribution routes.
//
// No build step: pure ES2020. Registered as window.initView_home so
// app.js can call it when the Home tab becomes active.

(function () {
  'use strict';

  const REPO = 'https://github.com/totalwindupflightsystems/off-by-one';
  const BLOB = REPO + '/blob/master';

  const LINKS = [
    { icon: '📦', title: 'Repository', desc: 'star · fork · watch', href: REPO, accent: 'blue' },
    { icon: '📖', title: 'Integration guide', desc: 'query the catalog from your tools', href: BLOB + '/docs/integration.md', accent: 'indigo' },
    { icon: '🔌', title: 'API reference', desc: 'every endpoint, every shape', href: BLOB + '/docs/api-reference.md', accent: 'green' },
    { icon: '🤝', title: 'Contributing', desc: 'add answers, open PRs', href: BLOB + '/CONTRIBUTING.md', accent: 'violet' },
    { icon: '🛡️', title: 'Security', desc: 'responsible disclosure', href: BLOB + '/SECURITY.md', accent: 'amber' },
    { icon: '📜', title: 'MIT License', desc: 'free to use, study, build on', href: BLOB + '/LICENSE', accent: 'pink' },
    { icon: '🗂️', title: 'Answer catalog', desc: 'flat files, no server needed', href: BLOB + '/data/INDEX.md', accent: 'blue' },
    { icon: '🏷️', title: 'Releases', desc: 'changelog + builds', href: REPO + '/releases', accent: 'indigo' },
  ];

  const PIPELINE = [
    { dot: 'g', label: 'Fleet submits problem', tool: 'ingest' },
    { dot: 'b', label: 'Sandboxed solve on idle cycles', tool: 'bwrap + pi-agent' },
    { dot: 'v', label: 'Judged against criteria', tool: 'gitreins' },
    { dot: 'p', label: 'Published with evidence', tool: 'catalog + repo' },
  ];

  const LANG_COLORS = {
    go: '#00a393', python: '#6d28d9', typescript: '#2563eb', ts: '#2563eb',
    rust: '#d97706', cpp: '#db2777', c: '#dc2626', java: '#ea580c',
    javascript: '#ca8a04', js: '#ca8a04', bash: '#059669', sql: '#0891b2',
    dockerfile: '#0ea5e9', zig: '#f59e0b', ruby: '#e11d48', php: '#7c3aed',
  };

  function el(tag, cls, text) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function langColor(lang) {
    return LANG_COLORS[lang] || '#8b93a7';
  }

  // ---------- LIVE tiles ----------

  function renderSearchPreview(host) {
    const list = host.querySelector('#bh-search-results');
    const input = host.querySelector('#bh-search-input');
    fetch('/api/v1/problems?q=git-hygiene&limit=3')
      .then(function (r) { return r.json(); })
      .then(function (d) {
        const rows = d.problems || [];
        if (!rows.length) {
          list.appendChild(el('div', 'bh-search-row', 'No results — try another query.'));
          return;
        }
        rows.forEach(function (p) {
          const row = el('div', 'bh-search-row');
          const left = el('span', 'bh-search-title');
          left.textContent = p.title || '';
          row.appendChild(left);
          const badge = el('span', 'bh-pill', 'Answers: ' + (p.answer_count || 0));
          row.appendChild(badge);
          list.appendChild(row);
        });
      })
      .catch(function () {
        list.appendChild(el('div', 'bh-search-row', 'Search unavailable right now.'));
      });
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && input.value.trim()) {
        location.hash = '#search?q=' + encodeURIComponent(input.value.trim());
      }
    });
  }

  function renderStats(host) {
    fetch('/api/v1/stats')
      .then(function (r) { return r.json(); })
      .then(function (st) {
        if (!st) return;
        host.querySelector('#bh-problems').textContent = (st.total_problems || 0).toLocaleString();
        host.querySelector('#bh-answers').textContent = (st.total_answers || 0).toLocaleString();
        host.querySelector('#bh-hitrate').textContent =
          st.hit_rate != null ? Math.round(st.hit_rate * 100) + '%' : '—';
        host.querySelector('#bh-coverage').textContent =
          st.coverage != null ? st.coverage.toFixed(3) : '—';
        if (st.readonly) {
          const note = el('div', 'bh-note', 'Public read-only catalog — new submissions go through the upstream lab.');
          host.querySelector('.bh-bento').after(note);
        }
      })
      .catch(function () { /* stats are decorative; leave dashes */ });
  }

  function renderLanguages(host) {
    const list = host.querySelector('#bh-lang-bars');
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
        if (!total) throw new Error('no taxonomy data');
        const top = Object.entries(counts).sort(function (a, b) { return b[1] - a[1]; }).slice(0, 5);
        top.forEach(function (entry) {
          const lang = entry[0];
          const pct = Math.round((entry[1] / total) * 100);
          const row = el('div', 'bh-lang-row');
          row.appendChild(el('span', 'bh-lang-name', lang));
          const bar = el('div', 'bh-lang-bar');
          const fill = el('div', 'bh-lang-fill');
          fill.style.width = pct + '%';
          fill.style.background = langColor(lang);
          bar.appendChild(fill);
          row.appendChild(bar);
          row.appendChild(el('span', 'bh-lang-pct', pct + '%'));
          list.appendChild(row);
        });
      })
      .catch(function () {
        list.appendChild(el('div', 'bh-lang-row', 'Coverage data unavailable.'));
      });
  }

  // ---------- View ----------

  function initView_home(root) {
    if (!root) return;
    root.innerHTML = '';

    // --- Hero ---
    const hero = el('section', 'bh-hero');
    hero.appendChild(el('span', 'bh-eyebrow', '◐ Pre-Solve Lab'));
    const h1 = el('h1', 'bh-title', null);
    h1.appendChild(document.createTextNode('Off-'));
    h1.appendChild(el('span', 'bh-by', 'By'));
    h1.appendChild(document.createTextNode('-One'));
    hero.appendChild(h1);
    hero.appendChild(el('p', 'bh-tagline',
      'Hard engineering problems in. Verified answers out — solved on idle compute while you sleep, judged by real criteria, published with evidence.'));
    const cta = el('div', 'bh-cta');
    const btnSearch = el('a', 'bh-btn bh-btn-primary', '🔍 Search the catalog');
    btnSearch.href = '#search';
    const btnRepo = el('a', 'bh-btn bh-btn-ghost', '⭐ Browse the repo');
    btnRepo.href = REPO;
    btnRepo.target = '_blank';
    btnRepo.rel = 'noopener noreferrer';
    cta.appendChild(btnSearch);
    cta.appendChild(btnRepo);
    hero.appendChild(cta);

    // --- Bento grid ---
    const bento = el('section', 'bh-bento');

    // Search tile (live)
    const tSearch = el('div', 'bh-tile bh-tile-blue bh-span2 bh-row2');
    tSearch.appendChild(el('h3', 'bh-tile-h', 'Full-text search — live corpus'));
    tSearch.appendChild(el('p', 'bh-tile-p', 'Type like you would grep. Results below are real, from the live API.'));
    const searchBox = el('div', 'bh-search-box');
    const input = el('input', 'bh-search-input');
    input.id = 'bh-search-input';
    input.type = 'text';
    input.placeholder = 'git-hygiene';
    searchBox.appendChild(input);
    const list = el('div', 'bh-search-results');
    list.id = 'bh-search-results';
    searchBox.appendChild(list);
    tSearch.appendChild(searchBox);
    const openSearch = el('a', 'bh-tile-cta', 'Open the full search →');
    openSearch.href = '#search';
    tSearch.appendChild(openSearch);

    // Stats tiles
    const tAnswers = el('div', 'bh-tile bh-tile-indigo');
    const vAnswers = el('div', 'bh-big bh-big-indigo', '—');
    vAnswers.id = 'bh-answers';
    tAnswers.appendChild(vAnswers);
    tAnswers.appendChild(el('div', 'bh-lbl', 'Verified answers'));
    tAnswers.appendChild(el('p', 'bh-tile-p', 'Every answer is executed in a sandbox and judged against the task\u2019s real criteria before it ships.'));
    tAnswers.appendChild(el('span', 'bh-tag bh-tag-green', 'VERIFIED · NOT GUESSED'));

    const tClasses = el('div', 'bh-tile bh-tile-green');
    const vClasses = el('div', 'bh-big bh-big-green', '—');
    vClasses.id = 'bh-problems';
    tClasses.appendChild(vClasses);
    tClasses.appendChild(el('div', 'bh-lbl', 'Problem classes'));
    tClasses.appendChild(el('p', 'bh-tile-p', 'Systems, cryptography, distributed systems, ML, graphics — fed by the whole fleet, growing daily.'));
    tClasses.appendChild(el('span', 'bh-tag bh-tag-blue', 'GROWING DAILY'));

    const tRate = el('div', 'bh-tile bh-tile-amber');
    const vRate = el('div', 'bh-big bh-big-amber', '—');
    vRate.id = 'bh-hitrate';
    tRate.appendChild(vRate);
    tRate.appendChild(el('div', 'bh-lbl', 'Hit rate'));
    tRate.appendChild(el('p', 'bh-tile-p', 'Every query in the corpus finds its verified answer.'));
    tRate.appendChild(el('span', 'bh-tag bh-tag-amber', '100% COVERAGE OF CORPUS'));

    // Pipeline tile
    const tPipe = el('div', 'bh-tile bh-tile-dark bh-span2');
    tPipe.appendChild(el('h3', 'bh-tile-h', 'From problem to published'));
    const pipe = el('div', 'bh-pipe');
    PIPELINE.forEach(function (s) {
      const row = el('div', 'bh-pipe-row');
      row.appendChild(el('span', 'bh-dot bh-dot-' + s.dot, ''));
      row.appendChild(el('span', 'bh-pipe-label', s.label));
      row.appendChild(el('span', 'bh-pipe-tool', s.tool));
      pipe.appendChild(row);
    });
    tPipe.appendChild(pipe);

    // Languages tile (live)
    const tLangs = el('div', 'bh-tile bh-tile-violet');
    tLangs.appendChild(el('h3', 'bh-tile-h', 'Language coverage'));
    const langBars = el('div', 'bh-langs');
    langBars.id = 'bh-lang-bars';
    tLangs.appendChild(langBars);

    // Flat-file tile
    const tCode = el('div', 'bh-tile bh-tile-pink');
    tCode.appendChild(el('h3', 'bh-tile-h', 'Flat-file catalog'));
    tCode.appendChild(el('p', 'bh-tile-p', 'One answer per line — no server required.'));
    const code = el('div', 'bh-code');
    const curlLine = el('span', null, 'curl -O \\\n  https://raw.githubusercontent.com/\n  totalwindupflightsystems/off-by-one/\n  master/data/answers/');
    code.appendChild(curlLine);
    code.appendChild(el('span', 'bh-code-hl', '0001-unknown.json'));
    tCode.appendChild(code);

    // Coverage tile (dark)
    const tCover = el('div', 'bh-tile bh-tile-dark');
    const vCover = el('div', 'bh-big bh-big-white', '—');
    vCover.id = 'bh-coverage';
    tCover.appendChild(vCover);
    tCover.appendChild(el('div', 'bh-lbl', 'Coverage'));
    tCover.appendChild(el('p', 'bh-tile-p', 'Answers per problem class — the corpus outgrows the question set.'));
    tCover.appendChild(el('span', 'bh-tag bh-tag-blue', 'LIVE'));

    // Links band
    const tLinks = el('div', 'bh-tile bh-tile-dark bh-span4 bh-links-band');
    tLinks.appendChild(el('h3', 'bh-tile-h', 'Open source · MIT · a community'));
    const linkRow = el('div', 'bh-link-row');
    LINKS.forEach(function (l) {
      const card = el('a', 'bh-link bh-link-' + l.accent, null);
      card.href = l.href;
      card.target = '_blank';
      card.rel = 'noopener noreferrer';
      card.appendChild(el('span', 'bh-link-i', l.icon));
      card.appendChild(el('span', 'bh-link-t', l.title));
      card.appendChild(el('span', 'bh-link-s', l.desc));
      linkRow.appendChild(card);
    });
    tLinks.appendChild(linkRow);

    bento.appendChild(tSearch);
    bento.appendChild(tAnswers);
    bento.appendChild(tClasses);
    bento.appendChild(tRate);
    bento.appendChild(tLangs);
    bento.appendChild(tPipe);
    bento.appendChild(tCode);
    bento.appendChild(tCover);
    bento.appendChild(tLinks);

    // --- CTA band ---
    const band = el('section', 'bh-cta-band');
    band.appendChild(el('h2', null, 'Answers one step ahead.'));
    band.appendChild(el('p', null, 'A live, growing catalog — generated automatically, verified honestly.'));
    const bandBtn = el('a', 'bh-btn bh-btn-white', 'Start searching');
    bandBtn.href = '#search';
    band.appendChild(bandBtn);

    root.appendChild(hero);
    root.appendChild(bento);
    root.appendChild(band);

    renderSearchPreview(root);
    renderStats(root);
    renderLanguages(root);
  }

  window.initView_home = initView_home;
})();
