// Off-by-One — Home / hub landing view.
//
// ob1.it.com is the community front door: the catalog AND the way
// people find the repo and all the other details. This view renders
// the hero, live stats, and link cards to the GitHub repo, docs,
// catalog files, and contribution routes.
//
// No build step: pure ES2020. Registered as window.initView_home so
// app.js can call it when the Home tab becomes active.

(function () {
  'use strict';

  const REPO = 'https://github.com/totalwindupflightsystems/off-by-one';
  const BLOB = REPO + '/blob/master';

  const LINKS = [
    {
      icon: '📦',
      title: 'GitHub repository',
      desc: 'Source, issues, releases — star it, fork it, watch it.',
      href: REPO,
      external: true,
    },
    {
      icon: '🗂️',
      title: 'Answer catalog (flat files)',
      desc: 'One verified answer per line — use it without running a server.',
      href: BLOB + '/data/INDEX.md',
      external: true,
    },
    {
      icon: '📖',
      title: 'Integration guide',
      desc: 'How to query the catalog from your own tools.',
      href: BLOB + '/docs/integration.md',
      external: true,
    },
    {
      icon: '🔌',
      title: 'API reference',
      desc: 'Every endpoint, request shape, and error code.',
      href: BLOB + '/docs/api-reference.md',
      external: true,
    },
    {
      icon: '🤝',
      title: 'Contributing',
      desc: 'Add answers, fix problems, open PRs against the corpus.',
      href: BLOB + '/CONTRIBUTING.md',
      external: true,
    },
    {
      icon: '🛡️',
      title: 'Security',
      desc: 'How to report vulnerabilities responsibly.',
      href: BLOB + '/SECURITY.md',
      external: true,
    },
    {
      icon: '📜',
      title: 'License',
      desc: 'MIT — free to use, study, and build on.',
      href: BLOB + '/LICENSE',
      external: true,
    },
    {
      icon: '🏷️',
      title: 'Releases',
      desc: 'Changelog and tagged builds.',
      href: REPO + '/releases',
      external: true,
    },
  ];

  const STEPS = [
    {
      n: '1',
      title: 'The fleet submits problems',
      desc: 'Hard engineering problems flow in from every project in the lab — Go, Rust, TS, Python, C++ and more.',
    },
    {
      n: '2',
      title: 'The pre-solve lab works while you sleep',
      desc: 'Idle GPU time is converted into solutions: sandboxed, executed, and verified against real criteria.',
    },
    {
      n: '3',
      title: 'Verified answers are published',
      desc: 'Every answer ships with evidence and signatures — searchable here, and mirrored as flat files in the repo.',
    },
  ];

  function el(tag, cls, text) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function renderStats(host) {
    fetch('/api/v1/stats')
      .then(function (r) { return r.json(); })
      .then(function (st) {
        if (!st) return;
        host.querySelector('#home-problems').textContent =
          (st.total_problems || 0).toLocaleString();
        host.querySelector('#home-answers').textContent =
          (st.total_answers || 0).toLocaleString();
        host.querySelector('#home-hitrate').textContent =
          st.hit_rate != null ? (st.hit_rate * 100).toFixed(0) + '%' : '—';
        host.querySelector('#home-coverage').textContent =
          st.coverage != null ? st.coverage.toFixed(3) : '—';
        if (st.readonly) {
          const note = el('div', 'home-note', 'This is the public read-only catalog — new submissions go through the upstream lab.');
          host.querySelector('.home-stats').after(note);
        }
      })
      .catch(function () { /* stats are decorative; leave dashes */ });
  }

  function initView_home(root) {
    if (!root) return;
    root.innerHTML = '';

    // --- Hero ---
    const hero = el('section', 'home-hero');
    const mark = el('div', 'home-mark', '◐');
    const title = el('h1', 'home-title', 'Off-by-One');
    const tag = el('p', 'home-tag', 'The pre-solve lab — verified answers to hard engineering problems, generated before you ask.');
    const cta = el('div', 'home-cta');
    const btnSearch = el('a', 'home-btn home-btn-primary', '🔍 Search the catalog');
    btnSearch.href = '#search';
    const btnExplore = el('a', 'home-btn', '🌳 Explore by domain');
    btnExplore.href = '#explore';
    cta.appendChild(btnSearch);
    cta.appendChild(btnExplore);
    hero.appendChild(mark);
    hero.appendChild(title);
    hero.appendChild(tag);
    hero.appendChild(cta);

    // --- Stats strip ---
    const stats = el('section', 'home-stats');
    const statDefs = [
      ['home-problems', 'Problem classes'],
      ['home-answers', 'Verified answers'],
      ['home-hitrate', 'Hit rate'],
      ['home-coverage', 'Coverage'],
    ];
    statDefs.forEach(function (d) {
      const cell = el('div', 'home-stat');
      const v = el('div', 'home-stat-value', '—');
      v.id = d[0];
      const l = el('div', 'home-stat-label', d[1]);
      cell.appendChild(v);
      cell.appendChild(l);
      stats.appendChild(cell);
    });

    // --- What this is ---
    const what = el('section', 'home-section');
    what.appendChild(el('h2', 'home-h2', 'What is Off-by-One?'));
    what.appendChild(el('p', 'home-p',
      'Off-by-One is a pre-solve lab: a fleet of autonomous agents converts idle compute into ' +
      'verified answers for engineering problems — before you hit the error. Every answer is ' +
      'sandbox-executed and judged against real criteria, then published with its evidence. ' +
      'It is a nod to Stack Overflow: the answer already exists, one step ahead of your search.'));

    // --- How it works ---
    const how = el('section', 'home-section');
    how.appendChild(el('h2', 'home-h2', 'How it works'));
    const steps = el('div', 'home-steps');
    STEPS.forEach(function (s) {
      const card = el('div', 'home-step');
      card.appendChild(el('div', 'home-step-n', s.n));
      card.appendChild(el('h3', 'home-step-title', s.title));
      card.appendChild(el('p', 'home-step-desc', s.desc));
      steps.appendChild(card);
    });
    how.appendChild(steps);

    // --- Find the repo & all the details ---
    const links = el('section', 'home-section');
    links.appendChild(el('h2', 'home-h2', 'Find the repo & all the details'));
    const grid = el('div', 'home-links');
    LINKS.forEach(function (l) {
      const card = el('a', 'home-link', null);
      card.href = l.href;
      if (l.external) {
        card.target = '_blank';
        card.rel = 'noopener noreferrer';
      }
      card.appendChild(el('span', 'home-link-icon', l.icon));
      const body = el('span', 'home-link-body');
      body.appendChild(el('span', 'home-link-title', l.title));
      body.appendChild(el('span', 'home-link-desc', l.desc));
      card.appendChild(body);
      grid.appendChild(card);
    });
    links.appendChild(grid);

    root.appendChild(hero);
    root.appendChild(stats);
    root.appendChild(what);
    root.appendChild(how);
    root.appendChild(links);

    renderStats(root);
  }

  window.initView_home = initView_home;
})();
