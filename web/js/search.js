// Off-by-One — Search view module (WI-009).
//
// Registers window.initView_search(container) so the shell's tab
// switcher (app.js) calls us when the Search tab becomes active.
// The module is self-contained: it builds its DOM from JS, fetches
// from /api/v1/problems and related endpoints, and manages the
// debounce + filter + expand + related-graph interactions.
//
// No build step: pure ES2020, no framework. D3.js is loaded lazily
// from CDN only when a related-problems graph is actually rendered.

(function () {
  'use strict';

  // ---------- Module state (reset on every initView call) ----------

  var state = {
    container: null,
    query: '',
    env: '',
    lang: '',
    status: '',
    limit: 20,
    offset: 0,
    total: 0,
    results: [],
    debounceTimer: null,
    debounceDelay: 300, // ms
    expandedClass: null, // currently expanded problem class slug
    d3Loaded: false,
    d3Loading: false,
  };

  // ---------- Public entry point ----------

  window.initView_search = function (container) {
    // Prevent double-init if the tab is re-activated.
    if (container.querySelector('.search-view')) {
      return;
    }
    state.container = container;
    readUrlState();
    buildDom(container);
    var input = container.querySelector('.search-input');
    if (input) input.value = state.query;
    registerHashListener();
    // Fire an initial search (empty query → list all problems).
    doSearch();
  };

  // ---------- DOM construction ----------

  function buildDom(container) {
    container.innerHTML = '';

    var wrap = el('div', 'search-view');

    // --- Search bar row ---
    var searchBar = el('div', 'search-bar');
    var input = el('input', 'search-input');
    input.type = 'text';
    input.placeholder = 'Search problem classes and answers…';
    input.setAttribute('aria-label', 'Search');
    input.addEventListener('input', function () {
      state.query = input.value.trim();
      state.offset = 0; // new query → back to page 1
      clearTimeout(state.debounceTimer);
      state.debounceTimer = setTimeout(doSearch, state.debounceDelay);
    });

    var statusHint = el('span', 'search-status-hint');
    statusHint.textContent = '';

    searchBar.appendChild(input);
    searchBar.appendChild(statusHint);

    // --- Filter chips row ---
    var filterRow = el('div', 'search-filters');

    var envChips = makeChipGroup('env', 'Environment', [
      { value: '', label: 'All' },
      { value: 'dev', label: 'Dev' },
      { value: 'staging', label: 'Staging' },
      { value: 'prod', label: 'Prod' },
    ]);
    var langChips = makeChipGroup('lang', 'Language', [
      { value: '', label: 'All' },
      { value: 'go', label: 'Go' },
      { value: 'python', label: 'Python' },
      { value: 'rust', label: 'Rust' },
      { value: 'typescript', label: 'TS' },
    ]);
    var statusChips = makeChipGroup('status', 'Status', [
      { value: '', label: 'All' },
      { value: 'solved', label: 'Solved' },
      { value: 'pending', label: 'Pending' },
      { value: 'failed', label: 'Failed' },
    ]);

    filterRow.appendChild(envChips);
    filterRow.appendChild(langChips);
    filterRow.appendChild(statusChips);

    // --- Results area ---
    var resultsArea = el('div', 'search-results');

    wrap.appendChild(searchBar);
    wrap.appendChild(filterRow);
    wrap.appendChild(resultsArea);

    container.appendChild(wrap);
  }

  // Chip group helper: label + row of clickable filter chips.
  function makeChipGroup(key, labelText, options) {
    var group = el('div', 'chip-group');
    group.dataset.key = key;

    var label = el('span', 'chip-label');
    label.textContent = labelText + ':';
    group.appendChild(label);

    options.forEach(function (opt, i) {
      var chip = el('button', 'chip');
      chip.type = 'button';
      chip.textContent = opt.label;
      chip.dataset.value = opt.value;
      chip.dataset.key = key;
      if (opt.value === (state[key] || '')) {
        chip.classList.add('active');
      }
      chip.addEventListener('click', function () {
        // Deactivate siblings
        group.querySelectorAll('.chip').forEach(function (s) {
          s.classList.remove('active');
        });
        chip.classList.add('active');
        state[key] = opt.value;
        state.offset = 0; // new filter → back to page 1
        doSearch();
      });
      group.appendChild(chip);
    });

    return group;
  }

  // ---------- Search execution ----------

  // ---------- URL state (shareable #search?q=&env=&limit=&offset=) ----------

  // Read search state from the URL hash:
  // #search?q=...&env=...&lang=...&status=...&limit=20&offset=40
  // A bare #search (no params) resets the view to defaults.
  function readUrlState() {
    state.query = '';
    state.env = '';
    state.lang = '';
    state.status = '';
    state.limit = 20;
    state.offset = 0;
    var hash = window.location.hash || '';
    var qIndex = hash.indexOf('?');
    if (qIndex === -1) return;
    var params = new URLSearchParams(hash.slice(qIndex + 1));
    if (params.has('q')) state.query = params.get('q').trim();
    if (params.has('env')) state.env = params.get('env');
    if (params.has('lang')) state.lang = params.get('lang');
    if (params.has('status')) state.status = params.get('status');
    var limit = parseInt(params.get('limit'), 10);
    if (!isNaN(limit) && limit >= 1 && limit <= 100) state.limit = limit;
    var offset = parseInt(params.get('offset'), 10);
    if (!isNaN(offset) && offset >= 0) state.offset = offset;
  }

  function buildParams() {
    var params = new URLSearchParams();
    if (state.query) params.set('q', state.query);
    if (state.env) params.set('env', state.env);
    if (state.lang) params.set('lang', state.lang);
    if (state.status) params.set('status', state.status);
    params.set('limit', String(state.limit));
    params.set('offset', String(state.offset));
    return params;
  }

  // Push the current search state into the URL hash (replaceState, so
  // back/forward still steps through previous pages).
  function syncUrl() {
    var target = '#search?' + buildParams().toString();
    if (window.location.hash !== target) {
      history.replaceState(null, '', target);
    }
  }

  // React to back/forward (or manual hash edits) so the URL stays the
  // source of truth for the search view.
  function registerHashListener() {
    if (window.__ob1SearchHashListener) return;
    window.__ob1SearchHashListener = true;
    window.addEventListener('hashchange', function () {
      var view = (window.location.hash || '').split('?')[0].slice(1);
      if (view !== 'search') return;
      var prev = JSON.stringify([state.query, state.env, state.lang, state.status, state.limit, state.offset]);
      readUrlState();
      var next = JSON.stringify([state.query, state.env, state.lang, state.status, state.limit, state.offset]);
      if (prev === next) return;
      var container = state.container;
      if (container) {
        var input = container.querySelector('.search-input');
        if (input) input.value = state.query;
        // Re-apply chip active states from the new URL.
        container.querySelectorAll('.chip-group').forEach(function (group) {
          var key = group.dataset.key;
          group.querySelectorAll('.chip').forEach(function (chip) {
            chip.classList.toggle('active', chip.dataset.value === (state[key] || ''));
          });
        });
      }
      doSearch();
    });
  }

  function doSearch() {
    var resultsArea = state.container.querySelector('.search-results');
    if (!resultsArea) return;

    resultsArea.innerHTML = '';
    resultsArea.appendChild(loadingEl());

    syncUrl();

    var url = '/api/v1/problems?' + buildParams().toString();

    fetch(url, { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        state.results = data.problems || [];
        state.total = data.total || 0;
        renderResults(resultsArea);
      })
      .catch(function (err) {
        renderError(resultsArea, err);
      });
  }

  // ---------- Results rendering ----------

  function renderResults(area) {
    area.innerHTML = '';

    if (state.results.length === 0) {
      var empty = el('div', 'search-empty');
      empty.textContent =
        state.query || state.env || state.lang || state.status
          ? 'No problems match your search.'
          : 'No problems in the database yet. Submit one to get started.';
      area.appendChild(empty);
      return;
    }

    // Summary line
    var summary = el('div', 'search-summary');
    summary.textContent =
      state.total + ' problem' + (state.total === 1 ? '' : 's') +
      (state.query ? ' for "' + state.query + '"' : '');
    area.appendChild(summary);

    // Result items
    state.results.forEach(function (p) {
      area.appendChild(renderResultItem(p));
    });

    // Pagination (only when the result set spans multiple pages)
    if (state.total > state.limit) {
      area.appendChild(renderPagination());
    }
  }

  function renderPagination() {
    var pages = Math.max(1, Math.ceil(state.total / state.limit));
    var page = Math.floor(state.offset / state.limit) + 1;
    var from = state.total === 0 ? 0 : state.offset + 1;
    var to = Math.min(state.offset + state.limit, state.total);

    var nav = el('div', 'search-pagination');

    var prev = el('button', 'pagination-btn');
    prev.type = 'button';
    prev.textContent = '‹ Prev';
    prev.disabled = state.offset <= 0;
    prev.addEventListener('click', function () {
      state.offset = Math.max(0, state.offset - state.limit);
      doSearch();
    });

    var pageInfo = el('span', 'pagination-page');
    pageInfo.textContent = 'Page ' + page + ' of ' + pages;

    var range = el('span', 'pagination-range');
    range.textContent = 'Showing ' + from + '–' + to + ' of ' + state.total;

    var next = el('button', 'pagination-btn');
    next.type = 'button';
    next.textContent = 'Next ›';
    next.disabled = state.offset + state.limit >= state.total;
    next.addEventListener('click', function () {
      state.offset = Math.min(state.offset + state.limit, Math.max(0, state.total - state.limit));
      doSearch();
    });

    nav.appendChild(prev);
    nav.appendChild(pageInfo);
    nav.appendChild(range);
    nav.appendChild(next);
    return nav;
  }

  function renderResultItem(p) {
    var item = el('div', 'result-item');
    item.dataset.class = p.problem_class || p.title;

    var header = el('div', 'result-header');
    header.addEventListener('click', function () {
      toggleExpand(item, p);
    });
    header.style.cursor = 'pointer';

    var titleRow = el('div', 'result-title-row');

    var title = el('span', 'result-title');
    title.textContent = p.title || p.problem_class || '(unnamed)';
    titleRow.appendChild(title);

    // Status badge
    var badge = el('span', 'result-badge badge-' + (p.status || 'pending'));
    badge.textContent = p.status || 'pending';
    titleRow.appendChild(badge);

    header.appendChild(titleRow);

    // Meta row: answer count, hit count, created
    var meta = el('div', 'result-meta');
    var parts = [];
    if (p.answer_count !== undefined && p.answer_count !== null) {
      parts.push('Answers: ' + p.answer_count);
    }
    if (p.hit_count !== undefined && p.hit_count > 0) {
      parts.push('Hits: ' + p.hit_count);
    }
    if (p.created_at) {
      parts.push(formatDate(p.created_at));
    }
    meta.textContent = parts.join(' · ');
    header.appendChild(meta);

    // Description (if present)
    if (p.description) {
      var desc = el('div', 'result-desc');
      desc.textContent = truncate(p.description, 200);
      header.appendChild(desc);
    }

    item.appendChild(header);

    // Expandable detail area (filled on click)
    var detail = el('div', 'result-detail');
    detail.hidden = true;
    item.appendChild(detail);

    return item;
  }

  // ---------- Expand / collapse ----------

  function toggleExpand(item, problem) {
    var detail = item.querySelector('.result-detail');
    var slug = problem.title || problem.problem_class;

    if (item.classList.contains('expanded')) {
      // Collapse
      item.classList.remove('expanded');
      detail.hidden = true;
      detail.innerHTML = '';
      state.expandedClass = null;
      return;
    }

    // Collapse other expanded items
    state.container.querySelectorAll('.result-item.expanded').forEach(function (other) {
      other.classList.remove('expanded');
      var d = other.querySelector('.result-detail');
      if (d) {
        d.hidden = true;
        d.innerHTML = '';
      }
    });

    // Expand this one
    item.classList.add('expanded');
    detail.hidden = false;
    detail.innerHTML = '';
    detail.appendChild(loadingEl('Loading answers…'));
    state.expandedClass = slug;

    loadAnswers(slug, detail);
  }

  function loadAnswers(slug, detailArea) {
    var url = '/api/v1/problems/' + encodeURIComponent(slug) + '/answers?limit=5';
    fetch(url, { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        renderAnswers(slug, data.answers || [], data.total || 0, detailArea);
      })
      .catch(function (err) {
        detailArea.innerHTML = '';
        var errEl = el('div', 'search-error');
        errEl.textContent = 'Failed to load answers: ' + err.message;
        detailArea.appendChild(errEl);
      });
  }

  function renderAnswers(slug, answers, total, area) {
    area.innerHTML = '';

    if (answers.length === 0) {
      var none = el('div', 'search-empty');
      none.textContent = 'No answers yet for this problem class.';
      area.appendChild(none);
    } else {
      answers.forEach(function (a, i) {
        area.appendChild(renderAnswerDetail(a, i === 0));
      });
    }

    // Related problems section with D3 graph
    var relatedSection = el('div', 'related-section');
    var relatedHeader = el('div', 'related-header');
    relatedHeader.textContent = 'Related Problems';
    relatedSection.appendChild(relatedHeader);

    var graphDiv = el('div', 'related-graph');
    graphDiv.id = 'related-graph-' + slug.replace(/[^a-zA-Z0-9]/g, '_');
    relatedSection.appendChild(graphDiv);

    area.appendChild(relatedSection);

    loadRelated(slug, graphDiv);
  }

  function renderAnswerDetail(a, isFirst) {
    var card = el('div', 'answer-card');
    if (!isFirst) {
      card.classList.add('answer-secondary');
    }

    // Version line
    var versionLine = el('div', 'answer-version-line');
    var versionBadge = el('span', 'answer-version');
    versionBadge.textContent = (a.version || 'any') + ' · ' + (a.lang || '?') + ' · ' + (a.env || '?');
    versionLine.appendChild(versionBadge);

    // Version warning placeholder (populated by discover endpoint
    // data when available; for now we check answer status)
    if (a.version_warnings && a.version_warnings.length > 0) {
      a.version_warnings.forEach(function (w) {
        var warn = el('div', 'version-warning');
        warn.textContent = '⚠ ' + w;
        versionLine.appendChild(warn);
      });
    }
    card.appendChild(versionLine);

    // Solution
    if (a.solution) {
      var sol = el('div', 'answer-solution');
      var solLabel = el('div', 'answer-label');
      solLabel.textContent = 'Solution';
      var solBody = el('div', 'answer-body');
      solBody.innerHTML = '';
      window.Ob1Markdown.renderInto(solBody, a.solution);
      sol.appendChild(solLabel);
      sol.appendChild(solBody);
      card.appendChild(sol);
    }

    // Evidence
    if (a.evidence) {
      var ev = el('div', 'answer-evidence');
      var evLabel = el('div', 'answer-label');
      evLabel.textContent = 'Evidence';
      var evBody = el('div', 'answer-body');
      evBody.innerHTML = '';
      window.Ob1Markdown.renderInto(evBody, a.evidence);
      ev.appendChild(evLabel);
      ev.appendChild(evBody);
      card.appendChild(ev);
    }

    // Footer: created_at + status
    var footer = el('div', 'answer-footer');
    footer.textContent = (a.status || 'verified') + ' · ' + formatDate(a.created_at);
    card.appendChild(footer);

    return card;
  }

  // ---------- Related problems (D3 force graph) ----------

  function loadRelated(slug, container) {
    var url = '/api/v1/problems/' + encodeURIComponent(slug) + '/related';
    fetch(url, { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) return null;
        return r.json();
      })
      .then(function (data) {
        if (!data || !data.related || data.related.length === 0) {
          container.innerHTML = '<div class="related-empty">No related problems.</div>';
          return;
        }
        renderForceGraph(container, slug, data.related);
      })
      .catch(function () {
        container.innerHTML = '<div class="related-empty">No related problems.</div>';
      });
  }

  function renderForceGraph(container, centerSlug, related) {
    // Load D3 lazily, then render.
    if (state.d3Loaded) {
      doRenderForceGraph(container, centerSlug, related);
      return;
    }
    if (state.d3Loading) {
      // Poll until loaded
      var attempts = 0;
      var poll = setInterval(function () {
        if (state.d3Loaded) {
          clearInterval(poll);
          doRenderForceGraph(container, centerSlug, related);
        } else if (attempts++ > 30) {
          clearInterval(poll);
          container.innerHTML = '<div class="related-empty">D3 failed to load.</div>';
        }
      }, 200);
      return;
    }

    state.d3Loading = true;
    var script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/d3@7/dist/d3.min.js';
    script.onload = function () {
      state.d3Loaded = true;
      state.d3Loading = false;
      doRenderForceGraph(container, centerSlug, related);
    };
    script.onerror = function () {
      state.d3Loading = false;
      container.innerHTML = '<div class="related-empty">D3 failed to load.</div>';
    };
    document.head.appendChild(script);
  }

  function doRenderForceGraph(container, centerSlug, related) {
    container.innerHTML = '';
    var width = Math.min(container.clientWidth || 400, 500);
    var height = 200;

    var svg = d3.select(container).append('svg')
      .attr('width', width)
      .attr('height', height)
      .attr('viewBox', [-width / 2, -height / 2, width, height]);

    // Build nodes: center + each related
    var nodes = [{ id: centerSlug, label: centerSlug, center: true }];
    var links = [];
    related.forEach(function (r) {
      var label = r.problem_class || r.relationship || '?';
      nodes.push({ id: label, label: label, center: false });
      links.push({ source: centerSlug, target: label, weight: r.weight || 0.5 });
    });

    var colorScale = d3.scaleOrdinal(d3.schemeCategory10);

    var simulation = d3.forceSimulation(nodes)
      .force('link', d3.forceLink(links).id(function (d) { return d.id; }).distance(60))
      .force('charge', d3.forceManyBody().strength(-300))
      .force('center', d3.forceCenter(0, 0))
      .force('collide', d3.forceCollide(35));

    // Links
    var link = svg.append('g')
      .attr('stroke', '#cbd5e1')
      .attr('stroke-opacity', 0.6)
      .selectAll('line')
      .data(links)
      .join('line')
      .attr('stroke-width', function (d) { return 1 + (d.weight || 0.5) * 2; });

    // Nodes
    var node = svg.append('g')
      .selectAll('g')
      .data(nodes)
      .join('g')
      .call(drag(simulation));

    node.append('circle')
      .attr('r', function (d) { return d.center ? 10 : 6; })
      .attr('fill', function (d) { return d.center ? '#38bdf8' : colorScale(d.id); })
      .attr('stroke', '#fff')
      .attr('stroke-width', 1.5);

    node.append('text')
      .text(function (d) { return d.label; })
      .attr('font-size', 10)
      .attr('font-family', 'sans-serif')
      .attr('dx', 12)
      .attr('dy', 4)
      .attr('fill', '#374151');

    node.on('click', function (event, d) {
      if (d.id !== centerSlug) {
        // Navigate to that problem class in search
        var input = state.container.querySelector('.search-input');
        if (input) {
          input.value = d.id;
          state.query = d.id;
          doSearch();
        }
      }
    });

    simulation.on('tick', function () {
      link
        .attr('x1', function (d) { return d.source.x; })
        .attr('y1', function (d) { return d.source.y; })
        .attr('x2', function (d) { return d.target.x; })
        .attr('y2', function (d) { return d.target.y; });
      node.attr('transform', function (d) {
        return 'translate(' + d.x + ',' + d.y + ')';
      });
    });

    function drag(simulation) {
      function dragstarted(event) {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        event.subject.fx = event.subject.x;
        event.subject.fy = event.subject.y;
      }
      function dragged(event) {
        event.subject.fx = event.x;
        event.subject.fy = event.y;
      }
      function dragended(event) {
        if (!event.active) simulation.alphaTarget(0);
        event.subject.fx = null;
        event.subject.fy = null;
      }
      return d3.drag()
        .on('start', dragstarted)
        .on('drag', dragged)
        .on('end', dragended);
    }
  }

  // ---------- Utilities ----------

  function el(tag, className) {
    var e = document.createElement(tag);
    if (className) e.className = className;
    return e;
  }

  function loadingEl(msg) {
    var d = el('div', 'search-loading');
    d.textContent = msg || 'Searching…';
    return d;
  }

  function renderError(area, err) {
    area.innerHTML = '';
    var e = el('div', 'search-error');
    e.textContent = 'Search failed: ' + (err.message || err);
    area.appendChild(e);
  }

  function truncate(s, max) {
    if (!s) return '';
    if (s.length <= max) return s;
    return s.slice(0, max - 1) + '…';
  }

  function formatDate(iso) {
    if (!iso) return '';
    try {
      var d = new Date(iso);
      return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    } catch (e) {
      return iso;
    }
  }

})();
