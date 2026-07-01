// Off-by-One — Explore view module (WI-011).
//
// Registers window.initView_explore(container) so the shell's tab
// switcher (app.js) calls us when the Explore tab becomes active.
// Displays the full problem taxonomy as an expandable tree:
//   problem class → environment → language → version → answer
// Clicking a problem class node also shows a D3.js force graph of
// related problems.
//
// No build step: pure ES2020, no framework. D3.js loaded lazily
// from CDN only when a related-problems graph is rendered.

(function () {
  'use strict';

  // ---------- Module state (reset on every initView call) ----------

  var state = {
    container: null,
    tree: [],
    loading: false,
    loaded: false,
    expandedNodes: {}, // key: "type:value" -> boolean
    selectedClass: null,
    d3Loaded: false,
    d3Loading: false,
  };

  // ---------- Public entry point ----------

  window.initView_explore = function (container) {
    if (container.querySelector('.explore-view')) {
      return;
    }
    state.container = container;
    state.expandedNodes = {};
    state.selectedClass = null;
    buildDom(container);
    loadTaxonomy();
  };

  // ---------- DOM construction ----------

  function buildDom(container) {
    container.innerHTML = '';

    var wrap = el('div', 'explore-view');

    // --- Heading ---
    var heading = el('h2', 'explore-heading');
    heading.textContent = 'Explore Taxonomy';
    wrap.appendChild(heading);

    var intro = el('p', 'explore-intro');
    intro.textContent =
      'Browse the full problem-class tree. Expand nodes to drill down: ' +
      'problem class → environment → language → version → answer.';
    wrap.appendChild(intro);

    // --- Layout: tree on left, details panel on right ---
    var layout = el('div', 'explore-layout');

    var treePanel = el('div', 'explore-tree-panel');
    var treeArea = el('div', 'explore-tree');
    treePanel.appendChild(treeArea);

    var detailPanel = el('div', 'explore-detail-panel');
    var detailArea = el('div', 'explore-detail');
    var detailHint = el('div', 'explore-detail-hint');
    detailHint.textContent = 'Select a node to see details.';
    detailArea.appendChild(detailHint);
    detailPanel.appendChild(detailArea);

    layout.appendChild(treePanel);
    layout.appendChild(detailPanel);

    wrap.appendChild(layout);

    container.appendChild(wrap);
  }

  // ---------- Load taxonomy ----------

  function loadTaxonomy() {
    var treeArea = state.container.querySelector('.explore-tree');
    if (!treeArea) return;

    state.loading = true;
    treeArea.innerHTML = '';
    treeArea.appendChild(loadingEl('Loading taxonomy…'));

    fetch('/api/v1/taxonomy', { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        state.tree = data.tree || [];
        state.loaded = true;
        state.loading = false;
        renderTree(treeArea);
      })
      .catch(function (err) {
        state.loading = false;
        treeArea.innerHTML = '';
        var e = el('div', 'explore-error');
        e.textContent = 'Failed to load taxonomy: ' + err.message;
        treeArea.appendChild(e);
      });
  }

  // ---------- Tree rendering ----------

  function renderTree(area) {
    area.innerHTML = '';

    if (state.tree.length === 0) {
      var empty = el('div', 'explore-empty');
      empty.textContent = 'No problem classes yet. Submit one to get started.';
      area.appendChild(empty);
      return;
    }

    // Summary line
    var summary = el('div', 'explore-summary');
    summary.textContent =
      state.tree.length + ' problem class' +
      (state.tree.length === 1 ? '' : 'es');
    area.appendChild(summary);

    // Build the tree
    state.tree.forEach(function (node) {
      area.appendChild(renderClassNode(node));
    });
  }

  function renderClassNode(node) {
    var key = 'class:' + node.title;
    var expanded = !!state.expandedNodes[key];

    var item = el('div', 'tree-node tree-class');
    item.dataset.title = node.title;

    // Row
    var row = el('div', 'tree-row tree-class-row');
    row.addEventListener('click', function () {
      toggleNode(key, item, node);
    });
    row.style.cursor = 'pointer';

    // Expand/collapse icon
    var icon = el('span', 'tree-icon');
    icon.textContent = expanded ? '▼' : '▶';

    // Title
    var title = el('span', 'tree-title');
    title.textContent = node.title;

    // Count badge
    var answerCount = (node.answers || []).length;
    var countBadge = el('span', 'tree-count');
    countBadge.textContent = String(answerCount);

    row.appendChild(icon);
    row.appendChild(title);
    row.appendChild(countBadge);
    item.appendChild(row);

    // Selected highlight
    if (state.selectedClass === node.title) {
      row.classList.add('tree-selected');
    }

    // Children area
    var children = el('div', 'tree-children');
    children.hidden = !expanded;
    item.appendChild(children);

    if (expanded) {
      renderChildren(children, node);
    }

    return item;
  }

  function toggleNode(key, item, node) {
    var children = item.querySelector('.tree-children');
    var icon = item.querySelector('.tree-icon');

    if (state.expandedNodes[key]) {
      // Collapse
      state.expandedNodes[key] = false;
      children.hidden = true;
      children.innerHTML = '';
      if (icon) icon.textContent = '▶';
      return;
    }

    // Expand
    state.expandedNodes[key] = true;
    children.hidden = false;
    if (icon) icon.textContent = '▼';

    // Select this class for detail panel + related graph
    if (key.indexOf('class:') === 0) {
      state.selectedClass = node.title;
      // Update selected highlight
      state.container.querySelectorAll('.tree-class-row').forEach(function (r) {
        r.classList.remove('tree-selected');
      });
      var thisRow = item.querySelector('.tree-class-row');
      if (thisRow) thisRow.classList.add('tree-selected');

      showClassDetail(node);
    }

    renderChildren(children, node);
  }

  function renderChildren(container, node) {
    container.innerHTML = '';

    if (keyOf(container) === 'answer') {
      // Direct answer list (for leaf nodes)
      return;
    }

    // Check what kind of node this is
    var nodeKey = container.closest('.tree-class') ? 'class' : 'sub';
    if (nodeKey === 'class') {
      // Group answers by environment
      var envGroups = groupBy(node.answers || [], function (a) {
        return a.env || a.environment || 'any';
      });

      var envKeys = Object.keys(envGroups).sort();
      if (envKeys.length === 0) {
        var none = el('div', 'explore-empty');
        none.textContent = 'No answers yet.';
        container.appendChild(none);
        return;
      }

      envKeys.forEach(function (envKey) {
        container.appendChild(renderEnvNode(node.title, envKey, envGroups[envKey]));
      });
    }
  }

  function renderEnvNode(classTitle, envValue, answers) {
    var key = 'env:' + classTitle + ':' + envValue;
    var expanded = !!state.expandedNodes[key];

    var item = el('div', 'tree-node tree-env');

    var row = el('div', 'tree-row tree-env-row');
    row.addEventListener('click', function () {
      toggleSubNode(key, item, 'env', classTitle, envValue, answers);
    });
    row.style.cursor = 'pointer';

    var icon = el('span', 'tree-icon');
    icon.textContent = expanded ? '▼' : '▶';

    var label = el('span', 'tree-label');
    label.textContent = envValue;

    var countBadge = el('span', 'tree-count');
    countBadge.textContent = String(answers.length);

    row.appendChild(icon);
    row.appendChild(label);
    row.appendChild(countBadge);
    item.appendChild(row);

    var children = el('div', 'tree-children');
    children.hidden = !expanded;
    item.appendChild(children);

    if (expanded) {
      renderEnvChildren(children, classTitle, envValue, answers);
    }

    return item;
  }

  function toggleSubNode(key, item, level, classTitle, parentValue, answers) {
    var children = item.querySelector('.tree-children');
    var icon = item.querySelector('.tree-icon');

    if (state.expandedNodes[key]) {
      state.expandedNodes[key] = false;
      children.hidden = true;
      children.innerHTML = '';
      if (icon) icon.textContent = '▶';
      return;
    }

    state.expandedNodes[key] = true;
    children.hidden = false;
    if (icon) icon.textContent = '▼';

    if (level === 'env') {
      renderEnvChildren(children, classTitle, parentValue, answers);
    } else if (level === 'lang') {
      renderLangChildren(children, classTitle, parentValue, answers);
    } else if (level === 'version') {
      renderVersionChildren(children, classTitle, parentValue, answers);
    }
  }

  function renderEnvChildren(container, classTitle, envValue, answers) {
    var langGroups = groupBy(answers, function (a) {
      return a.lang || a.language || 'any';
    });
    var langKeys = Object.keys(langGroups).sort();
    langKeys.forEach(function (langKey) {
      container.appendChild(renderLangNode(classTitle, envValue, langKey, langGroups[langKey]));
    });
  }

  function renderLangNode(classTitle, envValue, langValue, answers) {
    var key = 'lang:' + classTitle + ':' + envValue + ':' + langValue;
    var expanded = !!state.expandedNodes[key];

    var item = el('div', 'tree-node tree-lang');

    var row = el('div', 'tree-row tree-lang-row');
    row.addEventListener('click', function () {
      toggleSubNode(key, item, 'lang', classTitle, envValue + '/' + langValue, answers);
    });
    row.style.cursor = 'pointer';

    var icon = el('span', 'tree-icon');
    icon.textContent = expanded ? '▼' : '▶';

    var label = el('span', 'tree-label');
    label.textContent = langValue;

    var countBadge = el('span', 'tree-count');
    countBadge.textContent = String(answers.length);

    row.appendChild(icon);
    row.appendChild(label);
    row.appendChild(countBadge);
    item.appendChild(row);

    var children = el('div', 'tree-children');
    children.hidden = !expanded;
    item.appendChild(children);

    if (expanded) {
      renderLangChildren(children, classTitle, envValue + '/' + langValue, answers);
    }

    return item;
  }

  function renderLangChildren(container, classTitle, parentKey, answers) {
    var versionGroups = groupBy(answers, function (a) {
      return a.version || 'any';
    });
    var versionKeys = Object.keys(versionGroups).sort();
    versionKeys.forEach(function (versionKey) {
      container.appendChild(renderVersionNode(classTitle, parentKey, versionKey, versionGroups[versionKey]));
    });
  }

  function renderVersionNode(classTitle, parentKey, versionValue, answers) {
    var key = 'version:' + classTitle + ':' + parentKey + ':' + versionValue;
    var expanded = !!state.expandedNodes[key];

    var item = el('div', 'tree-node tree-version');

    var row = el('div', 'tree-row tree-version-row');
    row.addEventListener('click', function () {
      toggleSubNode(key, item, 'version', classTitle, parentKey + '/' + versionValue, answers);
    });
    row.style.cursor = 'pointer';

    var icon = el('span', 'tree-icon');
    icon.textContent = expanded ? '▼' : '▶';

    var label = el('span', 'tree-label');
    label.textContent = versionValue;

    var countBadge = el('span', 'tree-count');
    countBadge.textContent = String(answers.length);

    row.appendChild(icon);
    row.appendChild(label);
    row.appendChild(countBadge);
    item.appendChild(row);

    var children = el('div', 'tree-children');
    children.hidden = !expanded;
    item.appendChild(children);

    if (expanded) {
      renderVersionChildren(children, answers);
    }

    return item;
  }

  function renderVersionChildren(container, answers) {
    if (answers.length === 0) {
      var none = el('div', 'explore-empty');
      none.textContent = 'No answers.';
      container.appendChild(none);
      return;
    }

    answers.forEach(function (a) {
      container.appendChild(renderAnswerLeaf(a));
    });
  }

  function renderAnswerLeaf(a) {
    var item = el('div', 'tree-node tree-answer');

    var row = el('div', 'tree-row tree-answer-row');
    row.style.cursor = 'pointer';
    row.addEventListener('click', function () {
      showAnswerDetail(a);
    });

    var icon = el('span', 'tree-icon');
    icon.textContent = '●';

    var label = el('span', 'tree-label');
    label.textContent = truncate(
      (a.solution || '(no solution)'),
      60
    );

    var statusBadge = el('span', 'tree-answer-status');
    statusBadge.textContent = a.status || 'verified';
    statusBadge.classList.add('badge-' + (a.status || 'verified'));

    row.appendChild(icon);
    row.appendChild(label);
    row.appendChild(statusBadge);
    item.appendChild(row);

    return item;
  }

  // ---------- Detail panel ----------

  function showClassDetail(node) {
    var detailArea = state.container.querySelector('.explore-detail');
    if (!detailArea) return;
    detailArea.innerHTML = '';

    // Title
    var title = el('div', 'detail-title');
    title.textContent = node.title;
    detailArea.appendChild(title);

    // Description
    if (node.description) {
      var desc = el('div', 'detail-desc');
      desc.textContent = node.description;
      detailArea.appendChild(desc);
    }

    // Stats
    var answerCount = (node.answers || []).length;
    var stats = el('div', 'detail-stats');
    stats.textContent = answerCount + ' answer' + (answerCount === 1 ? '' : 's');
    detailArea.appendChild(stats);

    // Related problems graph
    var relatedSection = el('div', 'related-section');
    var relatedHeader = el('div', 'related-header');
    relatedHeader.textContent = 'Related Problems';
    relatedSection.appendChild(relatedHeader);

    var graphDiv = el('div', 'related-graph');
    graphDiv.id = 'explore-graph-' + node.title.replace(/[^a-zA-Z0-9]/g, '_');
    relatedSection.appendChild(graphDiv);

    detailArea.appendChild(relatedSection);

    loadRelated(node.title, graphDiv);
  }

  function showAnswerDetail(a) {
    var detailArea = state.container.querySelector('.explore-detail');
    if (!detailArea) return;
    detailArea.innerHTML = '';

    // Version line
    var versionLine = el('div', 'answer-version-line');
    var versionBadge = el('span', 'answer-version');
    versionBadge.textContent =
      (a.version || 'any') + ' · ' + (a.lang || a.language || '?') +
      ' · ' + (a.env || a.environment || '?');
    versionLine.appendChild(versionBadge);

    if (a.version_warnings && a.version_warnings.length > 0) {
      a.version_warnings.forEach(function (w) {
        var warn = el('div', 'version-warning');
        warn.textContent = '⚠ ' + w;
        versionLine.appendChild(warn);
      });
    }
    detailArea.appendChild(versionLine);

    // Solution
    if (a.solution) {
      var sol = el('div', 'answer-section');
      var solLabel = el('div', 'answer-label');
      solLabel.textContent = 'Solution';
      var solBody = el('div', 'answer-body');
      solBody.innerHTML = renderMarkdown(a.solution);
      sol.appendChild(solLabel);
      sol.appendChild(solBody);
      detailArea.appendChild(sol);
    }

    // Evidence
    if (a.evidence) {
      var ev = el('div', 'answer-section');
      var evLabel = el('div', 'answer-label');
      evLabel.textContent = 'Evidence';
      var evBody = el('div', 'answer-body');
      evBody.innerHTML = renderMarkdown(a.evidence);
      ev.appendChild(evLabel);
      ev.appendChild(evBody);
      detailArea.appendChild(ev);
    }

    // Footer
    var footer = el('div', 'answer-footer');
    footer.textContent = (a.status || 'verified') + ' · ' + formatDate(a.created_at);
    detailArea.appendChild(footer);
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
    if (state.d3Loaded) {
      doRenderForceGraph(container, centerSlug, related);
      return;
    }
    if (state.d3Loading) {
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

    var link = svg.append('g')
      .attr('stroke', '#cbd5e1')
      .attr('stroke-opacity', 0.6)
      .selectAll('line')
      .data(links)
      .join('line')
      .attr('stroke-width', function (d) { return 1 + (d.weight || 0.5) * 2; });

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
    d.textContent = msg || 'Loading…';
    return d;
  }

  function keyOf(el2) {
    return el2.dataset.key || '';
  }

  function groupBy(arr, fn) {
    var result = {};
    arr.forEach(function (item) {
      var key = fn(item);
      if (!result[key]) result[key] = [];
      result[key].push(item);
    });
    return result;
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
      return d.toLocaleDateString(undefined, {
        year: 'numeric', month: 'short', day: 'numeric',
      });
    } catch (e) {
      return iso;
    }
  }

  // Minimal markdown → HTML (same as search.js).
  function renderMarkdown(md) {
    if (!md) return '';
    var html = escapeHtml(md);
    html = html.replace(/```([\s\S]*?)```/g, function (_, code) {
      return '<pre><code>' + code + '</code></pre>';
    });
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/^### (.+)$/gm, '<h4>$1</h4>');
    html = html.replace(/^## (.+)$/gm, '<h3>$1</h3>');
    html = html.replace(/^# (.+)$/gm, '<h2>$1</h2>');
    html = html.replace(/(<\/pre>)/g, function (m) { return m + '\x00PRESERVE\x00'; });
    html = html.split('\x00PRESERVE\x00').map(function (block) {
      if (block.indexOf('<pre>') !== -1) return block;
      return block.replace(/\n/g, '<br>');
    }).join('\x00PRESERVE\x00').replace(/\x00PRESERVE\x00/g, '');
    return html;
  }

  function escapeHtml(s) {
    return s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }
})();
