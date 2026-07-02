// Off-by-One — Export view module (WI-012).
//
// Registers window.initView_export(container) so the shell's tab
// switcher (app.js) calls us when the Export tab becomes active.
// Flow:
//   1. Load taxonomy (GET /api/v1/taxonomy) to build a selectable
//      list of problem classes and their answers.
//   2. User checks the answers they want to export.
//   3. User enters target repo URL + optional branch + commit message.
//   4. Preview shows the files that will be generated.
//   5. POST /api/v1/export — show progress, then commit SHA + PR URL.
//
// When the export API is not yet implemented (WI-014 pending), the
// view shows a clear "not available" message but the selection and
// preview UI still render so the user can see what's ready.
//
// No build step: pure ES2020, no framework.

(function () {
  'use strict';

  // ---------- Module state ----------

  var state = {
    container: null,
    taxonomy: [],
    selected: {}, // key: "classId:answerId" -> true
    loading: false,
    exporting: false,
  };

  // ---------- Public entry point ----------

  window.initView_export = function (container) {
    if (container.querySelector('.export-view')) {
      return;
    }
    state.container = container;
    state.selected = {};
    buildDom(container);
    loadTaxonomy();
  };

  // ---------- DOM construction ----------

  function buildDom(container) {
    container.innerHTML = '';

    var wrap = el('div', 'export-view');

    var heading = el('h2', 'export-heading');
    heading.textContent = 'Export Answers';
    wrap.appendChild(heading);

    var intro = el('p', 'export-intro');
    intro.textContent =
      'Select verified answers to export as a git subtree. ' +
      'Answers are formatted as solution.md + evidence.md + signatures.json.';
    wrap.appendChild(intro);

    // --- Selection area ---
    var selectionSection = el('div', 'export-section');
    var selectionHeader = el('div', 'export-section-header');
    var selTitle = el('h3', 'export-section-title');
    selTitle.textContent = '1. Select Answers';
    selectionHeader.appendChild(selTitle);

    // Select all / deselect all
    var selActions = el('div', 'export-select-actions');
    var selectAllBtn = el('button', 'btn btn-secondary btn-sm');
    selectAllBtn.type = 'button';
    selectAllBtn.textContent = 'Select All';
    selectAllBtn.addEventListener('click', function () {
      selectAll(true);
    });
    var deselectAllBtn = el('button', 'btn btn-secondary btn-sm');
    deselectAllBtn.type = 'button';
    deselectAllBtn.textContent = 'Deselect All';
    deselectAllBtn.addEventListener('click', function () {
      selectAll(false);
    });
    selActions.appendChild(selectAllBtn);
    selActions.appendChild(deselectAllBtn);
    selectionHeader.appendChild(selActions);
    selectionSection.appendChild(selectionHeader);

    var listArea = el('div', 'export-answer-list');
    listArea.appendChild(loadingEl('Loading answers…'));
    selectionSection.appendChild(listArea);

    // Summary bar
    var summaryBar = el('div', 'export-summary-bar');
    summaryBar.id = 'export-summary';
    summaryBar.textContent = '0 answers selected';
    selectionSection.appendChild(summaryBar);

    wrap.appendChild(selectionSection);

    // --- Target repo + options ---
    var repoSection = el('div', 'export-section');
    var repoTitle = el('h3', 'export-section-title');
    repoTitle.textContent = '2. Choose Target Repository';
    repoSection.appendChild(repoTitle);

    var repoGroup = el('div', 'form-group');
    var repoLabel = el('label', 'form-label');
    repoLabel.textContent = 'Git Repository URL *';
    repoLabel.htmlFor = 'export-repo';
    var repoInput = el('input', 'form-input');
    repoInput.type = 'text';
    repoInput.id = 'export-repo';
    repoInput.name = 'target_repo';
    repoInput.placeholder = 'git@github.com:org/answers-repo.git';
    repoInput.required = true;
    repoGroup.appendChild(repoLabel);
    repoGroup.appendChild(repoInput);
    repoSection.appendChild(repoGroup);

    var optRow = el('div', 'form-row');
    var branchGroup = el('div', 'form-group');
    var branchLabel = el('label', 'form-label');
    branchLabel.textContent = 'Branch (optional)';
    branchLabel.htmlFor = 'export-branch';
    var branchInput = el('input', 'form-input');
    branchInput.type = 'text';
    branchInput.id = 'export-branch';
    branchInput.name = 'branch';
    branchInput.placeholder = 'main';
    branchGroup.appendChild(branchLabel);
    branchGroup.appendChild(branchInput);
    optRow.appendChild(branchGroup);

    var msgGroup = el('div', 'form-group');
    var msgLabel = el('label', 'form-label');
    msgLabel.textContent = 'Commit Message';
    msgLabel.htmlFor = 'export-message';
    var msgInput = el('input', 'form-input');
    msgInput.type = 'text';
    msgInput.id = 'export-message';
    msgInput.name = 'commit_message';
    msgInput.placeholder = 'Export N verified answers';
    msgGroup.appendChild(msgLabel);
    msgGroup.appendChild(msgInput);
    optRow.appendChild(msgGroup);

    repoSection.appendChild(optRow);
    wrap.appendChild(repoSection);

    // --- Preview ---
    var previewSection = el('div', 'export-section');
    var previewTitle = el('h3', 'export-section-title');
    previewTitle.textContent = '3. Preview';
    previewSection.appendChild(previewTitle);
    var previewArea = el('div', 'export-preview');
    previewArea.id = 'export-preview';
    previewArea.appendChild(emptyPreview());
    previewSection.appendChild(previewArea);
    wrap.appendChild(previewSection);

    // --- Export button ---
    var actionRow = el('div', 'form-actions');
    var exportBtn = el('button', 'btn btn-primary');
    exportBtn.type = 'button';
    exportBtn.id = 'export-btn';
    exportBtn.textContent = 'Export to Git';
    exportBtn.disabled = true;
    exportBtn.addEventListener('click', function () {
      doExport(repoInput, branchInput, msgInput, exportBtn);
    });
    actionRow.appendChild(exportBtn);
    wrap.appendChild(actionRow);

    // --- Result area ---
    var resultArea = el('div', 'export-result');
    resultArea.id = 'export-result';
    wrap.appendChild(resultArea);

    container.appendChild(wrap);
  }

  // ---------- Load taxonomy ----------

  function loadTaxonomy() {
    var listArea = state.container.querySelector('.export-answer-list');
    if (!listArea) return;

    state.loading = true;
    listArea.innerHTML = '';
    listArea.appendChild(loadingEl('Loading answers…'));

    fetch('/api/v1/taxonomy', { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function (data) {
        state.taxonomy = data.tree || [];
        state.loading = false;
        renderAnswerList(listArea);
        updatePreview();
      })
      .catch(function (err) {
        state.loading = false;
        listArea.innerHTML = '';
        var e = el('div', 'export-error');
        e.textContent = 'Failed to load taxonomy: ' + err.message;
        listArea.appendChild(e);
      });
  }

  // ---------- Answer list rendering ----------

  function renderAnswerList(area) {
    area.innerHTML = '';

    var totalAnswers = 0;
    state.taxonomy.forEach(function (node) {
      totalAnswers += (node.answers || []).length;
    });

    if (totalAnswers === 0) {
      var empty = el('div', 'export-empty');
      empty.textContent =
        'No answers available to export. Solve some problems first.';
      area.appendChild(empty);
      return;
    }

    state.taxonomy.forEach(function (node) {
      var answers = node.answers || [];
      if (answers.length === 0) return;

      var group = el('div', 'export-answer-group');
      var groupHeader = el('div', 'export-group-header');

      // Group checkbox
      var groupCb = el('input', 'export-group-cb');
      groupCb.type = 'checkbox';
      groupCb.addEventListener('change', function () {
        toggleGroup(node, groupCb.checked);
      });
      groupHeader.appendChild(groupCb);

      var groupLabel = el('span', 'export-group-label');
      groupLabel.textContent = node.title + ' (' + answers.length + ')';
      groupHeader.appendChild(groupLabel);

      group.appendChild(groupHeader);

      // Answer items
      answers.forEach(function (a) {
        group.appendChild(renderAnswerItem(node, a));
      });

      area.appendChild(group);
    });
  }

  function renderAnswerItem(node, a) {
    var item = el('div', 'export-answer-item');
    item.dataset.classId = a.problem_class || node.title;
    item.dataset.answerId = String(a.id);

    var cb = el('input');
    cb.type = 'checkbox';
    cb.dataset.key = (a.problem_class || node.title) + ':' + a.id;
    cb.addEventListener('change', function () {
      if (cb.checked) {
        state.selected[cb.dataset.key] = true;
      } else {
        delete state.selected[cb.dataset.key];
      }
      updateSummary();
      updatePreview();
    });

    var info = el('div', 'export-answer-info');
    var meta = el('span', 'export-answer-meta');
    meta.textContent =
      (a.version || 'any') + ' · ' +
      (a.lang || a.language || '?') + ' · ' +
      (a.env || a.environment || '?');
    info.appendChild(meta);

    var statusBadge = el('span', 'export-answer-status');
    statusBadge.textContent = a.status || 'verified';
    statusBadge.classList.add('badge-' + (a.status || 'verified'));
    info.appendChild(statusBadge);

    item.appendChild(cb);
    item.appendChild(info);
    return item;
  }

  function toggleGroup(node, checked) {
    var answers = node.answers || [];
    answers.forEach(function (a) {
      var key = (a.problem_class || node.title) + ':' + a.id;
      if (checked) {
        state.selected[key] = true;
      } else {
        delete state.selected[key];
      }
    });
    // Sync checkboxes
    var area = state.container.querySelector('.export-answer-list');
    if (!area) return;
    area.querySelectorAll('input[type="checkbox"]').forEach(function (cb) {
      if (cb.classList.contains('export-group-cb')) return;
      cb.checked = !!state.selected[cb.dataset.key];
    });
    updateSummary();
    updatePreview();
  }

  function selectAll(checked) {
    state.selected = {};
    if (checked) {
      state.taxonomy.forEach(function (node) {
        (node.answers || []).forEach(function (a) {
          var key = (a.problem_class || node.title) + ':' + a.id;
          state.selected[key] = true;
        });
      });
    }
    var area = state.container.querySelector('.export-answer-list');
    if (!area) return;
    area.querySelectorAll('input[type="checkbox"]').forEach(function (cb) {
      cb.checked = checked;
    });
    updateSummary();
    updatePreview();
  }

  // ---------- Summary + preview ----------

  function selectedCount() {
    return Object.keys(state.selected).length;
  }

  function selectedAnswerIds() {
    return Object.keys(state.selected).map(function (key) {
      var parts = key.split(':');
      return parseInt(parts[parts.length - 1], 10);
    });
  }

  function updateSummary() {
    var bar = state.container.querySelector('#export-summary');
    if (!bar) return;
    var count = selectedCount();
    bar.textContent =
      count + ' answer' + (count === 1 ? '' : 's') + ' selected';

    var btn = state.container.querySelector('#export-btn');
    if (btn) btn.disabled = count === 0;
  }

  function updatePreview() {
    var preview = state.container.querySelector('#export-preview');
    if (!preview) return;

    var count = selectedCount();
    if (count === 0) {
      preview.innerHTML = '';
      preview.appendChild(emptyPreview());
      return;
    }

    preview.innerHTML = '';

    var info = el('div', 'export-preview-info');
    info.textContent =
      count + ' answer' + (count === 1 ? '' : 's') +
      ' will be exported as subtree files in ' +
      'pre-solve-answers/{class}/{env}/{version}/.';
    preview.appendChild(info);

    // File tree preview
    var tree = el('div', 'export-preview-tree');
    var keys = Object.keys(state.selected).sort();
    keys.forEach(function (key) {
      var parts = key.split(':');
      var className = parts[0];
      var line = el('div', 'export-preview-file');
      line.textContent =
        'pre-solve-answers/' + className + '/{env}/{version}/';
      tree.appendChild(line);
    });
    preview.appendChild(tree);

    var note = el('div', 'export-preview-note');
    note.textContent =
      'Each directory will contain: solution.md, evidence.md, signatures.json';
    preview.appendChild(note);
  }

  function emptyPreview() {
    var d = el('div', 'export-preview-empty');
    d.textContent = 'Select answers to see a preview of exported files.';
    return d;
  }

  // ---------- Export action ----------

  function doExport(repoInput, branchInput, msgInput, btn) {
    var resultArea = state.container.querySelector('#export-result');
    if (!resultArea) return;

    var repoUrl = repoInput.value.trim();
    if (!repoUrl) {
      repoInput.focus();
      showError(resultArea, 'Target repository URL is required.');
      return;
    }

    var answerIds = selectedAnswerIds();
    if (answerIds.length === 0) {
      showError(resultArea, 'No answers selected.');
      return;
    }

    btn.disabled = true;
    btn.textContent = 'Exporting…';
    state.exporting = true;

    showProgress(resultArea, 'Exporting ' + answerIds.length + ' answers…');

    var body = {
      target_repo: repoUrl,
      answer_ids: answerIds,
      branch: branchInput.value.trim() || 'main',
      commit_message: msgInput.value.trim(),
    };

    fetch('/api/v1/export', {
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
        state.exporting = false;
        renderExportResult(resultArea, res);
      })
      .catch(function (err) {
        state.exporting = false;
        showError(resultArea, 'Network error: ' + err.message);
      })
      .finally(function () {
        btn.disabled = false;
        btn.textContent = 'Export to Git';
      });
  }

  function renderExportResult(area, res) {
    area.innerHTML = '';

    if (!res.ok) {
      var msg = res.data && res.data.detail
        ? res.data.detail
        : 'Export failed (HTTP ' + res.status + ')';
      if (res.status === 501) {
        msg = 'Export engine not yet implemented (WI-014). ' +
          'The export API endpoint exists but the git engine is pending.';
      }
      showError(area, msg);
      return;
    }

    var card = el('div', 'export-success');
    var badge = el('span', 'export-status-badge badge-verified');
    badge.textContent = 'exported';
    card.appendChild(badge);

    var title = el('div', 'export-result-title');
    title.textContent = 'Export Complete';
    card.appendChild(title);

    var d = res.data;
    if (d.commit_sha) {
      var shaRow = el('div', 'export-sha-row');
      var shaLabel = el('span', 'export-sha-label');
      shaLabel.textContent = 'Commit: ';
      var shaVal = el('code', 'export-sha-val');
      shaVal.textContent = d.commit_sha;
      shaRow.appendChild(shaLabel);
      shaRow.appendChild(shaVal);
      card.appendChild(shaRow);
    }

    if (d.pr_url) {
      var prLink = el('a', 'export-pr-link');
      prLink.href = d.pr_url;
      prLink.target = '_blank';
      prLink.rel = 'noopener';
      prLink.textContent = 'View Pull Request →';
      card.appendChild(prLink);
    }

    if (d.files_changed) {
      var filesRow = el('div', 'export-files-changed');
      filesRow.textContent =
        d.files_changed + ' file' +
        (d.files_changed === 1 ? '' : 's') + ' changed';
      card.appendChild(filesRow);
    }

    area.appendChild(card);
    card.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  // ---------- Helpers ----------

  function showProgress(area, msg) {
    area.innerHTML = '';
    var d = el('div', 'export-progress');
    var spinner = el('span', 'export-spinner');
    spinner.textContent = '⏳';
    d.appendChild(spinner);
    var text = el('span', 'export-progress-text');
    text.textContent = msg;
    d.appendChild(text);
    area.appendChild(d);
  }

  function showError(area, msg) {
    area.innerHTML = '';
    var e = el('div', 'export-error-msg');
    e.textContent = msg;
    area.appendChild(e);
  }

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
})();
