// Off-by-One — Problem view module (deep-linkable class pages).
//
// Route: #/problem/<class-slug> — a shareable page for one problem
// class: meta, problem statement, answers (solution/evidence), related
// problems, plus a copy-link button and links to the source files in
// the GitHub repo (corpus JSON + static page, shown only when they
// exist). Replaces the search view's inline expansion so every result
// has a stable URL you can send someone.
//
// No build step: pure ES2020, no framework. Loaded via /js/problem.js.

(function () {
  'use strict';

  var REPO = 'https://github.com/totalwindupflightsystems/off-by-one';
  var RAW = 'https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master';

  function el(tag, cls) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    return n;
  }

  function esc(s) {
    var d = document.createElement('div');
    d.textContent = s == null ? '' : String(s);
    return d.innerHTML;
  }

  function fmtDate(iso) {
    if (!iso) return 'unknown date';
    try {
      return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    } catch (e) {
      return String(iso);
    }
  }

  // ---------- Problem-statement synthesis ----------
  //
  // The corpus was seeded with description == title for 965/1062
  // classes, so the class API has nothing useful to show. The solution
  // markdown itself usually carries an explanation ("## What is a nil
  // pointer dereference?"); surface that as the problem statement.

  var FILLER = /^(now here|here is|here's|the complete solution|below is|i['’]ll)/i;

  function extractProblemContext(solution) {
    if (!solution) return '';
    var lines = solution.split(/\r?\n/);
    var headingRe = /^#{2,3}\s+(what is|problem|overview|background|the problem|introduction|context)[:\s]/i;
    var inSection = false;
    var buf = [];
    for (var i = 0; i < lines.length; i++) {
      var l = lines[i].trim();
      if (/^#{1,3}\s/.test(l)) {
        if (inSection) break;
        if (headingRe.test(l)) { inSection = true; continue; }
      } else if (inSection && l) {
        buf.push(l.trim());
        if (buf.join(' ').length > 420) break;
      }
    }
    var txt = buf.join(' ').replace(/\s+/g, ' ').trim();
    if (txt.length > 60) return txt;

    // Fallback: first substantive paragraph after the first # heading.
    var started = false;
    for (var j = 0; j < lines.length; j++) {
      var lj = lines[j].trim();
      if (/^#\s/.test(lj)) { started = true; continue; }
      if (started && lj && !/^#{1,6}\s/.test(lj) && !/^[-*_=]{3,}$/.test(lj) && !FILLER.test(lj)) {
        var p = lj.replace(/[*_`]/g, '').replace(/\s+/g, ' ').trim();
        if (p.length > 40) return p;
      }
    }
    return '';
  }

  // ---------- Repo links (existence-checked against raw GitHub) ----------

  function headOk(url, cb) {
    var ctl = new AbortController();
    var t = setTimeout(function () { ctl.abort(); cb(false); }, 6000);
    fetch(url, { method: 'HEAD', signal: ctl.signal, cache: 'no-store' })
      .then(function (r) { clearTimeout(t); cb(r.ok); })
      .catch(function () { clearTimeout(t); cb(false); });
  }

  function addCorpusLinks(container, classId, title) {
    var id4 = String(classId).padStart(4, '0');
    var slug = encodeURIComponent(title || '');
    var jsonPath = 'data/answers/' + id4 + '-' + slug + '.json';
    var pagePath = 'site/classes/' + id4 + '-' + slug + '.html';
    headOk(RAW + '/' + jsonPath, function (ok) {
      if (!ok) return;
      var a = el('a', 'prob-link');
      a.href = REPO + '/blob/master/' + jsonPath;
      a.target = '_blank';
      a.rel = 'noopener';
      a.textContent = '📦 Corpus source (JSON)';
      container.appendChild(a);
    });
    headOk(RAW + '/' + pagePath, function (ok) {
      if (!ok) return;
      var a = el('a', 'prob-link');
      a.href = RAW + '/' + pagePath;
      a.target = '_blank';
      a.rel = 'noopener';
      a.textContent = '🗂 Static page (bot-crawlable)';
      container.appendChild(a);
    });
  }

  // ---------- Rendering ----------

  function renderMeta(area, cls, answers, total) {
    var meta = el('div', 'prob-meta');
    var bits = [];
    if (cls.id != null) bits.push('class #' + cls.id);
    if (cls.created_at) bits.push('first seen ' + fmtDate(cls.created_at));
    if (cls.hit_count != null) bits.push(cls.hit_count + ' hits');
    bits.push((total || 0) + ' answer' + (total === 1 ? '' : 's'));
    meta.textContent = bits.join(' · ');
    area.appendChild(meta);

    var badge = el('span', 'prob-badge');
    badge.textContent = String(cls.status || 'verified').toUpperCase();
    area.appendChild(badge);
  }

  function renderProblemSection(area, cls, firstAnswer) {
    var section = el('section', 'prob-problem');
    var label = el('div', 'answer-label');
    label.textContent = 'The Problem';
    section.appendChild(label);

    var desc = (cls.description || '').trim();
    var body = el('div', 'answer-body');
    if (desc && desc !== (cls.title || '').trim() && desc.length > 20) {
      body.textContent = desc;
      section.appendChild(body);
      area.appendChild(section);
      return;
    }

    // Synthesize from the solution's own explanation.
    var context = firstAnswer && extractProblemContext(firstAnswer.solution);
    if (context) {
      var p = el('p');
      p.textContent = context;
      body.appendChild(p);
      var note = el('div', 'prob-note');
      note.textContent = 'Problem statement not captured at ingest — this context is drawn from the verified solution below.';
      section.appendChild(body);
      section.appendChild(note);
    } else {
      body.textContent = desc || 'No problem statement captured for this class yet.';
      section.appendChild(body);
    }
    area.appendChild(section);
  }

  function renderAnswer(area, a, isFirst) {
    var card = el('div', 'answer-card');
    if (!isFirst) card.classList.add('answer-secondary');

    var versionLine = el('div', 'answer-version-line');
    var versionBadge = el('span', 'answer-version');
    versionBadge.textContent = (a.version || 'any') + ' · ' + (a.lang || a.language || '?') + ' · ' + (a.env || a.environment || '?');
    versionLine.appendChild(versionBadge);
    card.appendChild(versionLine);

    if (a.solution) {
      var sol = el('div', 'answer-solution');
      var solLabel = el('div', 'answer-label');
      solLabel.textContent = isFirst ? 'Solution' : 'Solution (alternate)';
      var solBody = el('div', 'answer-body');
      window.Ob1Markdown.renderInto(solBody, a.solution);
      sol.appendChild(solLabel);
      sol.appendChild(solBody);
      card.appendChild(sol);
    }

    if (a.evidence) {
      var ev = el('div', 'answer-evidence');
      var evLabel = el('div', 'answer-label');
      evLabel.textContent = 'Evidence';
      var evBody = el('div', 'answer-body');
      window.Ob1Markdown.renderInto(evBody, a.evidence);
      ev.appendChild(evLabel);
      ev.appendChild(evBody);
      card.appendChild(ev);
    }

    if (a.signatures) {
      var sig = el('details', 'answer-signatures');
      var sum = el('summary');
      sum.textContent = 'Signatures';
      var sigBody = el('pre', 'answer-sig-body');
      var raw = typeof a.signatures === 'string' ? a.signatures : JSON.stringify(a.signatures);
      try { sigBody.textContent = JSON.stringify(JSON.parse(raw), null, 2); }
      catch (e) { sigBody.textContent = raw; }
      sig.appendChild(sum);
      sig.appendChild(sigBody);
      card.appendChild(sig);
    }

    var footer = el('div', 'answer-footer');
    footer.textContent = (a.status || 'verified') + ' · ' + fmtDate(a.created_at) + ' · answer #' + (a.answer_id != null ? a.answer_id : '?');
    card.appendChild(footer);

    area.appendChild(card);
  }

  function renderRelated(area, slug, related) {
    var section = el('section', 'prob-related');
    var label = el('div', 'answer-label');
    label.textContent = 'Related Problems';
    section.appendChild(label);
    if (!related || !related.length) {
      var none = el('div', 'search-empty');
      none.textContent = 'No related problems.';
      section.appendChild(none);
      area.appendChild(section);
      return;
    }
    var list = el('ul', 'prob-related-list');
    related.forEach(function (r) {
      var name = r.title || r.problem_class || r.class_id || '';
      var li = el('li');
      var a = el('a');
      a.href = '#/problem/' + encodeURIComponent(name);
      a.textContent = name;
      li.appendChild(a);
      list.appendChild(li);
    });
    section.appendChild(list);
    area.appendChild(section);
  }

  function loading(area) {
    area.innerHTML = '';
    var l = el('div', 'search-loading');
    l.textContent = 'Loading…';
    area.appendChild(l);
  }

  function error(area, msg) {
    area.innerHTML = '';
    var e = el('div', 'search-error');
    e.textContent = msg;
    area.appendChild(e);
  }

  // ---------- Public API ----------

  function open(slug) {
    var view = document.getElementById('view-problem');
    if (!view) return;
    view.hidden = false;
    view.innerHTML = '';

    var header = el('div', 'prob-header');
    var back = el('a', 'prob-back');
    back.href = '#search';
    back.textContent = '← Back to search';
    header.appendChild(back);

    var title = el('h2', 'prob-title');
    title.textContent = slug;
    header.appendChild(title);

    var links = el('div', 'prob-links');
    var copy = el('button', 'prob-link prob-copy');
    copy.type = 'button';
    copy.textContent = '🔗 Copy link';
    copy.addEventListener('click', function () {
      var url = location.origin + location.pathname + '#/problem/' + encodeURIComponent(slug);
      var done = function () {
        copy.textContent = '✓ Copied';
        setTimeout(function () { copy.textContent = '🔗 Copy link'; }, 1600);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(url).then(done, function () { window.prompt('Copy link:', url); done(); });
      } else {
        window.prompt('Copy link:', url);
      }
    });
    links.appendChild(copy);
    header.appendChild(links);

    var metaArea = el('div', 'prob-meta-area');
    header.appendChild(metaArea);
    view.appendChild(header);

    var body = el('div', 'prob-body');
    view.appendChild(body);

    loading(body);
    var parts = el('div');
    body.appendChild(parts);

    var clsReq = fetch('/api/v1/problems/' + encodeURIComponent(slug), { headers: { Accept: 'application/json' } });
    var ansReq = fetch('/api/v1/problems/' + encodeURIComponent(slug) + '/answers?limit=20', { headers: { Accept: 'application/json' } });
    var relReq = fetch('/api/v1/problems/' + encodeURIComponent(slug) + '/related', { headers: { Accept: 'application/json' } });

    Promise.all([clsReq, ansReq, relReq])
      .then(function (rs) {
        return Promise.all(rs.map(function (r) {
          if (!r.ok) throw new Error('HTTP ' + r.status);
          return r.json();
        }));
      })
      .then(function (data) {
        var cls = data[0];
        var answers = (data[1] && data[1].answers) || [];
        var total = (data[1] && data[1].total) || answers.length;
        var related = (data[2] && data[2].related) || [];

        parts.innerHTML = '';
        renderMeta(metaArea, cls, answers, total);
        renderProblemSection(parts, cls, answers[0]);
        addCorpusLinks(links, cls.id, cls.title || slug);

        var ansSection = el('section', 'prob-answers');
        var ansLabel = el('div', 'answer-label');
        ansLabel.textContent = 'Verified Answers';
        ansSection.appendChild(ansLabel);
        if (!answers.length) {
          var none = el('div', 'search-empty');
          none.textContent = 'No verified answers yet for this problem class.';
          ansSection.appendChild(none);
        } else {
          answers.forEach(function (a, i) { renderAnswer(ansSection, a, i === 0); });
        }
        parts.appendChild(ansSection);

        renderRelated(parts, slug, related);
      })
      .catch(function (err) {
        error(parts, 'Failed to load problem: ' + err.message);
      });
  }

  window.Ob1Problem = { open: open, extractProblemContext: extractProblemContext };
})();
