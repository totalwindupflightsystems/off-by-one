// Off-by-One — Submit view module (WI-010).
//
// Registers window.initView_submit(container) so the shell's tab
// switcher (app.js) calls us when the Submit tab becomes active.
// The module builds a form that POSTs to /api/v1/problems/submit,
// with auto-suggest of existing problem classes from
// /api/v1/taxonomy.
//
// No build step: pure ES2020, no framework.

(function () {
  'use strict';

  // ---------- Module state ----------

  var state = {
    container: null,
    form: null,
    suggestions: [],
    suggestionTimer: null,
    suggestionDelay: 200, // ms
    taxonomyTitles: [],
    taxonomyLoaded: false,
  };

  // ---------- Public entry point ----------

  window.initView_submit = function (container) {
    if (container.querySelector('.submit-view')) {
      return;
    }
    state.container = container;
    buildDom(container);
  };

  // ---------- DOM construction ----------

  function buildDom(container) {
    container.innerHTML = '';

    var wrap = el('div', 'submit-view');

    // --- Heading ---
    var heading = el('h2', 'submit-heading');
    heading.textContent = 'Submit a Problem';
    wrap.appendChild(heading);

    var intro = el('p', 'submit-intro');
    intro.textContent =
      'Queue a problem for the idle pre-solve lab. The system will ' +
      'deduplicate against existing answers and solve during idle compute time.';
    wrap.appendChild(intro);

    // --- Form ---
    var form = el('form', 'submit-form');
    state.form = form;
    form.noValidate = true;

    // Problem class (with autocomplete)
    var classGroup = el('div', 'form-group');
    var classLabel = el('label', 'form-label');
    classLabel.textContent = 'Problem Class *';
    classLabel.htmlFor = 'submit-class';
    var classWrap = el('div', 'input-suggest-wrap');
    var classInput = el('input', 'form-input');
    classInput.type = 'text';
    classInput.id = 'submit-class';
    classInput.name = 'problem_class';
    classInput.placeholder = 'e.g. nil-pointer-deref-on-startup';
    classInput.setAttribute('aria-label', 'Problem class');
    classInput.setAttribute('autocomplete', 'off');
    classInput.required = true;

    var suggestBox = el('div', 'suggest-box');
    suggestBox.id = 'submit-suggest';
    suggestBox.hidden = true;

    classWrap.appendChild(classInput);
    classWrap.appendChild(suggestBox);
    classGroup.appendChild(classLabel);
    classGroup.appendChild(classWrap);

    // Suggestion logic
    classInput.addEventListener('input', function () {
      clearTimeout(state.suggestionTimer);
      state.suggestionTimer = setTimeout(function () {
        showSuggestions(classInput.value.trim(), suggestBox);
      }, state.suggestionDelay);
    });
    classInput.addEventListener('focus', function () {
      if (classInput.value.trim()) {
        showSuggestions(classInput.value.trim(), suggestBox);
      }
    });
    classInput.addEventListener('blur', function () {
      // Delay to allow click on suggestion
      setTimeout(function () { suggestBox.hidden = true; }, 150);
    });

    // --- Two-column row: environment + language ---
    var row1 = el('div', 'form-row');

    var envGroup = el('div', 'form-group');
    var envLabel = el('label', 'form-label');
    envLabel.textContent = 'Environment';
    envLabel.htmlFor = 'submit-env';
    var envSelect = el('select', 'form-input');
    envSelect.id = 'submit-env';
    envSelect.name = 'environment';
    [
      { v: '', l: '— Any —' },
      { v: 'dev', l: 'Dev' },
      { v: 'staging', l: 'Staging' },
      { v: 'prod', l: 'Prod' },
      { v: 'ci', l: 'CI' },
      { v: 'local', l: 'Local' },
    ].forEach(function (opt) {
      var o = el('option');
      o.value = opt.v;
      o.textContent = opt.l;
      envSelect.appendChild(o);
    });
    envGroup.appendChild(envLabel);
    envGroup.appendChild(envSelect);

    var langGroup = el('div', 'form-group');
    var langLabel = el('label', 'form-label');
    langLabel.textContent = 'Language';
    langLabel.htmlFor = 'submit-lang';
    var langSelect = el('select', 'form-input');
    langSelect.id = 'submit-lang';
    langSelect.name = 'language';
    [
      { v: '', l: '— Any —' },
      { v: 'go', l: 'Go' },
      { v: 'python', l: 'Python' },
      { v: 'rust', l: 'Rust' },
      { v: 'typescript', l: 'TypeScript' },
      { v: 'javascript', l: 'JavaScript' },
      { v: 'java', l: 'Java' },
      { v: 'c', l: 'C' },
      { v: 'cpp', l: 'C++' },
    ].forEach(function (opt) {
      var o = el('option');
      o.value = opt.v;
      o.textContent = opt.l;
      langSelect.appendChild(o);
    });
    langGroup.appendChild(langLabel);
    langGroup.appendChild(langSelect);

    row1.appendChild(envGroup);
    row1.appendChild(langGroup);

    // --- Version ---
    var verGroup = el('div', 'form-group');
    var verLabel = el('label', 'form-label');
    verLabel.textContent = 'Version';
    verLabel.htmlFor = 'submit-version';
    var verInput = el('input', 'form-input');
    verInput.type = 'text';
    verInput.id = 'submit-version';
    verInput.name = 'version';
    verInput.placeholder = 'e.g. v1.2.3 (optional)';
    verGroup.appendChild(verLabel);
    verGroup.appendChild(verInput);

    // --- Description ---
    var descGroup = el('div', 'form-group');
    var descLabel = el('label', 'form-label');
    descLabel.textContent = 'Description';
    descLabel.htmlFor = 'submit-desc';
    var descArea = el('textarea', 'form-input form-textarea');
    descArea.id = 'submit-desc';
    descArea.name = 'description';
    descArea.placeholder = 'Describe the problem context, what triggered it, and what you expected…';
    descArea.rows = 3;
    descGroup.appendChild(descLabel);
    descGroup.appendChild(descArea);

    // --- Error message ---
    var errGroup = el('div', 'form-group');
    var errLabel = el('label', 'form-label');
    errLabel.textContent = 'Error Message';
    errLabel.htmlFor = 'submit-error';
    var errArea = el('textarea', 'form-input form-textarea');
    errArea.id = 'submit-error';
    errArea.name = 'error_message';
    errArea.placeholder = 'Paste the exact error message (optional)…';
    errArea.rows = 3;
    errGroup.appendChild(errLabel);
    errGroup.appendChild(errArea);

    // --- File attachments ---
    var fileGroup = el('div', 'form-group');
    var fileLabel = el('label', 'form-label');
    fileLabel.textContent = 'Attachments';
    fileLabel.htmlFor = 'submit-files';
    var fileInput = el('input', 'form-input');
    fileInput.type = 'file';
    fileInput.id = 'submit-files';
    fileInput.name = 'files';
    fileInput.multiple = true;
    fileInput.accept = '.log,.txt,.json,.yaml,.yml,.md,.py,.go,.rs,.ts,.js,.toml';
    var fileHint = el('div', 'form-hint');
    fileHint.textContent = 'Log files, stack traces, config files (optional)';
    fileGroup.appendChild(fileLabel);
    fileGroup.appendChild(fileInput);
    fileGroup.appendChild(fileHint);

    // --- Cadence selector ---
    var cadenceGroup = el('div', 'form-group');
    var cadenceLabel = el('label', 'form-label');
    cadenceLabel.textContent = 'Cadence';
    cadenceLabel.htmlFor = 'submit-cadence';
    var cadenceRow = el('div', 'cadence-row');

    var cadences = [
      { v: 'pre-phase', l: 'Pre-Phase', desc: 'Solve before the next work phase' },
      { v: 'end-of-day', l: 'End-of-Day', desc: 'Solve by end of day' },
      { v: 'post-debug', l: 'Post-Debug', desc: 'Solve right after this debugging session' },
    ];

    var cadenceInput = el('input');
    cadenceInput.type = 'hidden';
    cadenceInput.id = 'submit-cadence';
    cadenceInput.name = 'cadence';
    cadenceInput.value = 'post-debug'; // highest priority default

    cadences.forEach(function (c, i) {
      var card = el('div', 'cadence-card');
      if (c.v === cadenceInput.value) card.classList.add('selected');
      card.dataset.value = c.v;

      var title = el('div', 'cadence-card-title');
      title.textContent = c.l;
      card.appendChild(title);

      var hint = el('div', 'cadence-card-desc');
      hint.textContent = c.desc;
      card.appendChild(hint);

      card.addEventListener('click', function () {
        cadenceRow.querySelectorAll('.cadence-card').forEach(function (s) {
          s.classList.remove('selected');
        });
        card.classList.add('selected');
        cadenceInput.value = c.v;
      });

      cadenceRow.appendChild(card);
    });

    cadenceGroup.appendChild(cadenceLabel);
    cadenceGroup.appendChild(cadenceRow);
    cadenceGroup.appendChild(cadenceInput);

    // --- Submit button ---
    var btnRow = el('div', 'form-actions');
    var submitBtn = el('button', 'btn btn-primary');
    submitBtn.type = 'submit';
    submitBtn.textContent = 'Submit Problem';
    btnRow.appendChild(submitBtn);

    // --- Result area (populated after submit) ---
    var resultArea = el('div', 'submit-result');

    // --- Assemble form ---
    form.appendChild(classGroup);
    form.appendChild(row1);
    form.appendChild(verGroup);
    form.appendChild(descGroup);
    form.appendChild(errGroup);
    form.appendChild(fileGroup);
    form.appendChild(cadenceGroup);
    form.appendChild(btnRow);

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      doSubmit(form, resultArea, submitBtn);
    });

    wrap.appendChild(form);
    wrap.appendChild(resultArea);

    container.appendChild(wrap);

    // Preload taxonomy for autocomplete
    loadTaxonomy();
  }

  // ---------- Taxonomy / autocomplete ----------

  function loadTaxonomy() {
    fetch('/api/v1/taxonomy', { headers: { Accept: 'application/json' } })
      .then(function (r) {
        if (!r.ok) return null;
        return r.json();
      })
      .then(function (data) {
        if (!data || !data.tree) return;
        state.taxonomyTitles = [];
        (data.tree || []).forEach(function (node) {
          if (node.title) state.taxonomyTitles.push(node.title);
        });
        state.taxonomyLoaded = true;
      })
      .catch(function () {
        // Non-fatal — autocomplete just won't have suggestions
      });
  }

  function showSuggestions(query, box) {
    if (!state.taxonomyLoaded || !query) {
      box.hidden = true;
      box.innerHTML = '';
      return;
    }

    var lower = query.toLowerCase();
    var matches = state.taxonomyTitles
      .filter(function (t) {
        return t.toLowerCase().indexOf(lower) !== -1;
      })
      .slice(0, 8);

    if (matches.length === 0) {
      box.hidden = true;
      box.innerHTML = '';
      return;
    }

    box.innerHTML = '';
    matches.forEach(function (title) {
      var item = el('div', 'suggest-item');
      item.textContent = title;
      item.addEventListener('mousedown', function (e) {
        e.preventDefault();
        var input = state.form.querySelector('#submit-class');
        if (input) {
          input.value = title;
        }
        box.hidden = true;
        box.innerHTML = '';
      });
      box.appendChild(item);
    });
    box.hidden = false;
  }

  // ---------- Submit ----------

  function doSubmit(form, resultArea, btn) {
    clearErrors(form);

    var classInput = form.querySelector('#submit-class');
    if (!classInput.value.trim()) {
      showError(classInput, 'Problem class is required');
      return;
    }

    btn.disabled = true;
    btn.textContent = 'Submitting…';

    var fileInput = form.querySelector('#submit-files');
    var hasFiles = fileInput && fileInput.files && fileInput.files.length > 0;

    var body = {
      problem_class: classInput.value.trim(),
      environment: form.querySelector('#submit-env').value,
      language: form.querySelector('#submit-lang').value,
      version: form.querySelector('#submit-version').value.trim(),
      description: form.querySelector('#submit-desc').value.trim(),
      error_message: form.querySelector('#submit-error').value.trim(),
      cadence: form.querySelector('#submit-cadence').value,
    };

    if (hasFiles) {
      // Use FormData for multipart upload with file attachments.
      var fd = new FormData();
      fd.append('data', JSON.stringify(body));
      for (var i = 0; i < fileInput.files.length; i++) {
        fd.append('file_' + i, fileInput.files[i]);
      }
      fetch('/api/v1/problems/submit', {
        method: 'POST',
        headers: { Accept: 'application/json' },
        body: fd,
      })
        .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, status: r.status, data: j }; }); })
        .then(function (res) { renderResult(resultArea, res); })
        .catch(function (err) { renderError(resultArea, 'Network error: ' + err.message); })
        .finally(function () { btn.disabled = false; btn.textContent = 'Submit Problem'; });
    } else {
      // JSON-only (no files).
      fetch('/api/v1/problems/submit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify(body),
      })
        .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, status: r.status, data: j }; }); })
        .then(function (res) { renderResult(resultArea, res); })
        .catch(function (err) { renderError(resultArea, 'Network error: ' + err.message); })
        .finally(function () { btn.disabled = false; btn.textContent = 'Submit Problem'; });
    }
  }

  function renderResult(area, res) {
    area.innerHTML = '';

    if (!res.ok) {
      var msg = res.data && res.data.detail ? res.data.detail : 'Submission failed (HTTP ' + res.status + ')';
      renderError(area, msg);
      return;
    }

    var d = res.data;
    var card = el('div', 'submit-success');

    // Status badge
    var badge = el('span', 'submit-status-badge');
    badge.textContent = d.status || 'queued';
    badge.classList.add('badge-' + (d.status || 'queued'));
    card.appendChild(badge);

    var title = el('div', 'submit-result-title');
    title.textContent = d.problem_class || '(problem)';
    card.appendChild(title);

    // Position + ETA
    if (d.status === 'queued') {
      var info = el('div', 'submit-result-info');
      var pos = el('span', 'submit-position');
      pos.textContent = 'Queue position: #' + (d.position || 0);
      info.appendChild(pos);

      if (d.estimated_time) {
        var eta = el('span', 'submit-eta');
        eta.textContent = 'Est. solve time: ' + d.estimated_time;
        info.appendChild(eta);
      }
      card.appendChild(info);
    }

    // Deduplicated
    if (d.status === 'deduplicated') {
      var dupNote = el('div', 'submit-result-info');
      dupNote.textContent = 'This problem was already solved or is already in the queue.';
      card.appendChild(dupNote);
    }

    // Existing solutions
    if (d.existing_solutions > 0) {
      var sol = el('div', 'submit-existing');
      sol.textContent = d.existing_solutions + ' existing solution' +
        (d.existing_solutions === 1 ? '' : 's') + ' found.';
      card.appendChild(sol);
    }

    // Related problems
    if (d.related_problems && d.related_problems.length > 0) {
      var relWrap = el('div', 'submit-related');
      var relLabel = el('div', 'submit-related-label');
      relLabel.textContent = 'Related problems:';
      relWrap.appendChild(relLabel);
      d.related_problems.forEach(function (rp) {
        var tag = el('span', 'related-tag');
        tag.textContent = rp;
        relWrap.appendChild(tag);
      });
      card.appendChild(relWrap);
    }

    // Submission ID
    if (d.submission_id) {
      var idRow = el('div', 'submit-id-row');
      idRow.textContent = 'Submission ID: ' + d.submission_id;
      card.appendChild(idRow);
    }

    area.appendChild(card);

    // Scroll the result into view
    card.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  // ---------- Validation helpers ----------

  function showError(input, msg) {
    input.classList.add('input-error');
    var grp = input.closest('.form-group');
    if (grp) {
      var err = el('div', 'form-error');
      err.textContent = msg;
      grp.appendChild(err);
    }
    input.focus();
  }

  function clearErrors(form) {
    form.querySelectorAll('.input-error').forEach(function (el2) {
      el2.classList.remove('input-error');
    });
    form.querySelectorAll('.form-error').forEach(function (el2) {
      el2.remove();
    });
  }

  function renderError(area, msg) {
    area.innerHTML = '';
    var e = el('div', 'submit-error-msg');
    e.textContent = msg;
    area.appendChild(e);
  }

  // ---------- Utility ----------

  function el(tag, className) {
    var e = document.createElement(tag);
    if (className) e.className = className;
    return e;
  }
})();
