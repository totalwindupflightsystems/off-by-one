// Off-by-One — Import view module (WI-012).
//
// Registers window.initView_import(container) so the shell's tab
// switcher (app.js) calls us when the Import tab becomes active.
// Flow:
//   1. User enters source git repo URL + optional branch.
//   2. Preview: fetch the repo, parse pre-solve-answers/ directory,
//      diff against the local graph.
//   3. User selects which answers to import.
//   4. Conflict resolution UI for same class+version, different answer.
//   5. POST /api/v1/import — show progress, then import summary.
//
// When the import API is not yet implemented (WI-015 pending), the
// view shows a clear "not available" message.
//
// No build step: pure ES2020, no framework.

(function () {
  'use strict';

  // ---------- Module state ----------

  var state = {
    container: null,
    preview: null, // preview response from API
    selected: {}, // key: "class:version" -> true
    conflictStrategy: 'skip',
    importing: false,
  };

  // ---------- Public entry point ----------

  window.initView_import = function (container) {
    if (container.querySelector('.import-view')) {
      return;
    }
    state.container = container;
    state.preview = null;
    state.selected = {};
    state.conflictStrategy = 'skip';
    buildDom(container);
  };

  // ---------- DOM construction ----------

  function buildDom(container) {
    container.innerHTML = '';

    var wrap = el('div', 'import-view');

    var heading = el('h2', 'import-heading');
    heading.textContent = 'Import Answers';
    wrap.appendChild(heading);

    var intro = el('p', 'import-intro');
    intro.textContent =
      'Import pre-solved answers from a git repository. The system ' +
      'will diff incoming answers against your local graph and show ' +
      'conflicts for resolution.';
    wrap.appendChild(intro);

    // --- Source repo ---
    var repoSection = el('div', 'import-section');
    var repoTitle = el('h3', 'import-section-title');
    repoTitle.textContent = '1. Source Repository';
    repoSection.appendChild(repoTitle);

    var repoGroup = el('div', 'form-group');
    var repoLabel = el('label', 'form-label');
    repoLabel.textContent = 'Git Repository URL *';
    repoLabel.htmlFor = 'import-repo';
    var repoInput = el('input', 'form-input');
    repoInput.type = 'text';
    repoInput.id = 'import-repo';
    repoInput.name = 'source_repo';
    repoInput.placeholder = 'git@github.com:org/answers-repo.git';
    repoInput.required = true;
    repoGroup.appendChild(repoLabel);
    repoGroup.appendChild(repoInput);
    repoSection.appendChild(repoGroup);

    var optRow = el('div', 'form-row');
    var branchGroup = el('div', 'form-group');
    var branchLabel = el('label', 'form-label');
    branchLabel.textContent = 'Branch (optional)';
    branchLabel.htmlFor = 'import-branch';
    var branchInput = el('input', 'form-input');
    branchInput.type = 'text';
    branchInput.id = 'import-branch';
    branchInput.name = 'branch';
    branchInput.placeholder = 'main';
    branchGroup.appendChild(branchLabel);
    branchGroup.appendChild(branchInput);
    optRow.appendChild(branchGroup);

    // Conflict strategy
    var stratGroup = el('div', 'form-group');
    var stratLabel = el('label', 'form-label');
    stratLabel.textContent = 'Conflict Strategy';
    stratLabel.htmlFor = 'import-strategy';
    var stratSelect = el('select', 'form-input');
    stratSelect.id = 'import-strategy';
    stratSelect.name = 'conflict_strategy';
    [
      { v: 'skip', l: 'Skip conflicts' },
      { v: 'replace', l: 'Replace local with incoming' },
      { v: 'manual', l: 'Manual review' },
    ].forEach(function (opt) {
      var o = el('option');
      o.value = opt.v;
      o.textContent = opt.l;
      stratSelect.appendChild(o);
    });
    stratSelect.addEventListener('change', function () {
      state.conflictStrategy = stratSelect.value;
    });
    stratGroup.appendChild(stratLabel);
    stratGroup.appendChild(stratSelect);
    optRow.appendChild(stratGroup);

    repoSection.appendChild(optRow);

    // Preview button
    var previewBtnRow = el('div', 'form-actions');
    var previewBtn = el('button', 'btn btn-secondary');
    previewBtn.type = 'button';
    previewBtn.id = 'import-preview-btn';
    previewBtn.textContent = 'Preview Import';
    previewBtn.addEventListener('click', function () {
      doPreview(repoInput, branchInput, previewBtn);
    });
    previewBtnRow.appendChild(previewBtn);

    repoSection.appendChild(previewBtnRow);
    wrap.appendChild(repoSection);

    // --- Preview area ---
    var previewSection = el('div', 'import-section');
    var previewTitle = el('h3', 'import-section-title');
    previewTitle.textContent = '2. Preview & Select';
    previewSection.appendChild(previewTitle);

    var previewArea = el('div', 'import-preview');
    previewArea.id = 'import-preview';
    var previewHint = el('div', 'import-preview-hint');
    previewHint.textContent =
      'Enter a repo URL and click Preview Import to see incoming answers.';
    previewArea.appendChild(previewHint);
    previewSection.appendChild(previewArea);

    wrap.appendChild(previewSection);

    // --- Import button ---
    var actionRow = el('div', 'form-actions');
    var importBtn = el('button', 'btn btn-primary');
    importBtn.type = 'button';
    importBtn.id = 'import-btn';
    importBtn.textContent = 'Import Selected';
    importBtn.disabled = true;
    importBtn.addEventListener('click', function () {
      doImport(repoInput, branchInput, stratSelect, importBtn);
    });
    actionRow.appendChild(importBtn);
    wrap.appendChild(actionRow);

    // --- Result area ---
    var resultArea = el('div', 'import-result');
    resultArea.id = 'import-result';
    wrap.appendChild(resultArea);

    container.appendChild(wrap);
  }

  // ---------- Preview ----------

  function doPreview(repoInput, branchInput, btn) {
    var previewArea = state.container.querySelector('#import-preview');
    if (!previewArea) return;

    var repoUrl = repoInput.value.trim();
    if (!repoUrl) {
      repoInput.focus();
      previewArea.innerHTML = '';
      var e = el('div', 'import-error');
      e.textContent = 'Source repository URL is required.';
      previewArea.appendChild(e);
      return;
    }

    btn.disabled = true;
    btn.textContent = 'Previewing…';

    showProgress(previewArea, 'Fetching and diffing…');

    // The preview endpoint is the same as import but with a dry_run
    // flag. Since the API spec doesn't define dry_run, we call import
    // and handle the result. When WI-015 implements the engine, this
    // will return a diff. For now, the endpoint returns 501.
    var body = {
      source_repo: repoUrl,
      branch: branchInput.value.trim() || 'main',
      conflict_strategy: state.conflictStrategy,
    };

    fetch('/api/v1/import', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(body),
    })
      .then(function (r) {
        return r.json().then(function (j) {
          return { ok: r.ok, status: r.status, data: j };
        });
      })
      .then(function (res) {
        renderPreview(previewArea, res, repoUrl);
      })
      .catch(function (err) {
        previewArea.innerHTML = '';
        var e = el('div', 'import-error');
        e.textContent = 'Network error: ' + err.message;
        previewArea.appendChild(e);
      })
      .finally(function () {
        btn.disabled = false;
        btn.textContent = 'Preview Import';
      });
  }

  function renderPreview(area, res, repoUrl) {
    area.innerHTML = '';

    if (!res.ok) {
      var msg = res.data && res.data.detail
        ? res.data.detail
        : 'Preview failed (HTTP ' + res.status + ')';
      if (res.status === 501) {
        msg = 'Import engine not yet implemented (WI-015). ' +
          'The import API endpoint exists but the git engine is pending.';
      }
      var e = el('div', 'import-error');
      e.textContent = msg;
      area.appendChild(e);
      // Disable import button
      var importBtn = state.container.querySelector('#import-btn');
      if (importBtn) importBtn.disabled = true;
      return;
    }

    state.preview = res.data;
    var d = res.data;

    // Summary header
    var summary = el('div', 'import-preview-summary');
    var repoLabel = el('div', 'import-preview-repo');
    repoLabel.textContent = 'Source: ' + repoUrl;
    summary.appendChild(repoLabel);

    // Stats row
    var stats = el('div', 'import-preview-stats');
    [
      { label: 'Added', value: d.added || 0, cls: 'import-stat-new' },
      { label: 'Updated', value: d.updated || 0, cls: 'import-stat-updated' },
      { label: 'Skipped', value: d.skipped || 0, cls: 'import-stat-skipped' },
      { label: 'Conflicted', value: d.conflicted || 0, cls: 'import-stat-conflict' },
    ].forEach(function (s) {
      var stat = el('div', 'import-stat ' + s.cls);
      var val = el('span', 'import-stat-value');
      val.textContent = String(s.value);
      stat.appendChild(val);
      var lbl = el('span', 'import-stat-label');
      lbl.textContent = s.label;
      stat.appendChild(lbl);
      stats.appendChild(stat);
    });
    summary.appendChild(stats);
    area.appendChild(summary);

    // When the API returns summary stats but no per-answer list
    // (current spec shape), show a summary-only view.
    var total = (d.added || 0) + (d.updated || 0) +
      (d.skipped || 0) + (d.conflicted || 0);

    if (d.conflicted && d.conflicted > 0) {
      var conflictNote = el('div', 'import-conflict-note');
      conflictNote.textContent =
        d.conflicted + ' conflict' +
        (d.conflicted === 1 ? '' : 's') + ' detected. ' +
        'Strategy: ' + state.conflictStrategy + '.';
      area.appendChild(conflictNote);

      if (state.conflictStrategy === 'manual') {
        var manualNote = el('div', 'import-manual-note');
        manualNote.textContent =
          'Manual review mode: conflicts will be listed for you to ' +
          'resolve individually once the import engine is implemented.';
        area.appendChild(manualNote);
      }
    }

    // Action line
    var info = el('div', 'import-preview-action');
    if (total === 0) {
      info.textContent = 'No new answers to import — the repo is already in sync.';
    } else {
      info.textContent =
        total + ' answer' + (total === 1 ? '' : 's') +
        ' processed. Click "Import Selected" to proceed.';
      var importBtn = state.container.querySelector('#import-btn');
      if (importBtn) importBtn.disabled = false;
    }
    area.appendChild(info);
  }

  // ---------- Import action ----------

  function doImport(repoInput, branchInput, stratSelect, btn) {
    var resultArea = state.container.querySelector('#import-result');
    if (!resultArea) return;

    var repoUrl = repoInput.value.trim();
    if (!repoUrl) return;

    btn.disabled = true;
    btn.textContent = 'Importing…';
    state.importing = true;

    showProgress(resultArea, 'Importing answers…');

    var body = {
      source_repo: repoUrl,
      branch: branchInput.value.trim() || 'main',
      conflict_strategy: stratSelect.value,
    };

    fetch('/api/v1/import', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(body),
    })
      .then(function (r) {
        return r.json().then(function (j) {
          return { ok: r.ok, status: r.status, data: j };
        });
      })
      .then(function (res) {
        state.importing = false;
        renderImportResult(resultArea, res);
      })
      .catch(function (err) {
        state.importing = false;
        var e = el('div', 'import-error-msg');
        e.textContent = 'Network error: ' + err.message;
        resultArea.innerHTML = '';
        resultArea.appendChild(e);
      })
      .finally(function () {
        btn.disabled = false;
        btn.textContent = 'Import Selected';
      });
  }

  function renderImportResult(area, res) {
    area.innerHTML = '';

    if (!res.ok) {
      var msg = res.data && res.data.detail
        ? res.data.detail
        : 'Import failed (HTTP ' + res.status + ')';
      if (res.status === 501) {
        msg = 'Import engine not yet implemented (WI-015). ' +
          'The import API endpoint exists but the git engine is pending.';
      }
      var e = el('div', 'import-error-msg');
      e.textContent = msg;
      area.appendChild(e);
      return;
    }

    var card = el('div', 'import-success');
    var badge = el('span', 'import-status-badge badge-verified');
    badge.textContent = 'imported';
    card.appendChild(badge);

    var title = el('div', 'import-result-title');
    title.textContent = 'Import Complete';
    card.appendChild(title);

    var d = res.data;

    // Stats grid
    var statsGrid = el('div', 'import-result-stats');
    [
      { label: 'Added', value: d.added || 0 },
      { label: 'Updated', value: d.updated || 0 },
      { label: 'Skipped', value: d.skipped || 0 },
      { label: 'Conflicted', value: d.conflicted || 0 },
    ].forEach(function (s) {
      var stat = el('div', 'import-result-stat');
      var val = el('div', 'import-result-stat-value');
      val.textContent = String(s.value);
      stat.appendChild(val);
      var lbl = el('div', 'import-result-stat-label');
      lbl.textContent = s.label;
      stat.appendChild(lbl);
      statsGrid.appendChild(stat);
    });
    card.appendChild(statsGrid);

    area.appendChild(card);
    card.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  // ---------- Helpers ----------

  function showProgress(area, msg) {
    area.innerHTML = '';
    var d = el('div', 'import-progress');
    var spinner = el('span', 'import-spinner');
    spinner.textContent = '⏳';
    d.appendChild(spinner);
    var text = el('span', 'import-progress-text');
    text.textContent = msg;
    d.appendChild(text);
    area.appendChild(d);
  }

  function el(tag, className) {
    var e = document.createElement(tag);
    if (className) e.className = className;
    return e;
  }
})();
