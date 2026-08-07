// Off-by-One — shared Markdown renderer (UI-001).
//
// Renders solution/evidence/chat markdown properly: GFM tables,
// fenced code blocks with language classes, headings, lists, links,
// blockquotes — and mermaid diagrams (```mermaid) rendered to SVG.
//
// marked + mermaid are loaded lazily from CDN (same pattern as D3 in
// search.js). If the CDN is unreachable we fall back to a minimal
// built-in renderer so answers never render as raw text.
//
// No build step: pure ES2020. Exposes window.Ob1Markdown.

(function () {
  'use strict';

  var MARKED_CDN = 'https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js';
  var MERMAID_CDN = 'https://cdn.jsdelivr.net/npm/mermaid@10.9.1/dist/mermaid.min.js';

  var loadedScripts = {};

  function loadScript(src, cb) {
    if (loadedScripts[src]) { cb(true); return; }
    loadedScripts[src] = true;
    var s = document.createElement('script');
    s.src = src;
    s.async = true;
    s.onload = function () { cb(true); };
    s.onerror = function () { cb(false); };
    document.head.appendChild(s);
  }

  function escapeHtml(s) {
    return s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // Render markdown → HTML. Uses marked when available (GFM: tables,
  // strikethrough, autolinks); falls back to the minimal renderer.
  function render(md) {
    if (!md) return '';
    if (window.marked) {
      try {
        return postProcess(window.marked.parse(md, { gfm: true, breaks: false }));
      } catch (e) {
        console.warn('marked failed, falling back to minimal renderer:', e);
      }
    }
    return fallbackRender(md);
  }

  // Pull ```mermaid blocks out of the rendered HTML into <pre class="mermaid">
  // so mermaid can turn them into SVGs. The nodes are NOT in the DOM yet at
  // this point — renderInto calls processMermaid after assigning innerHTML.
  function postProcess(html) {
    return html.replace(/<pre><code class="language-mermaid">([\s\S]*?)<\/code><\/pre>/g, function (_, code) {
      return '<pre class="mermaid">' + code + '</pre>';
    });
  }

  // Find mermaid blocks inside a live container and render them to SVG.
  function processMermaid(el) {
    if (!el) return;
    var nodes = Array.prototype.slice.call(el.querySelectorAll('pre.mermaid'))
      .filter(function (n) { return !n.querySelector('svg'); });
    if (nodes.length) ensureMermaid(nodes);
  }

  var mermaidInitialized = false;
  var mermaidLoading = false;

  function ensureMermaid(nodes) {
    var pending = nodes.filter(function (n) { return !n.querySelector('svg'); });
    if (!pending.length) return;
    if (window.mermaid && mermaidInitialized) {
      runMermaid(pending);
      return;
    }
    if (mermaidLoading) return;
    mermaidLoading = true;
    loadScript(MERMAID_CDN, function (ok) {
      mermaidLoading = false;
      if (!ok || !window.mermaid) { degradeMermaid(nodes); return; }
      try {
        window.mermaid.initialize({ startOnLoad: false, theme: 'default' });
        mermaidInitialized = true;
        runMermaid(pending);
      } catch (e) {
        console.warn('mermaid init failed:', e);
        degradeMermaid(nodes);
      }
    });
  }

  function runMermaid(nodes) {
    nodes.forEach(function (n) {
      if (n.querySelector('svg')) return;
      try {
        window.mermaid.run({ nodes: [n] });
      } catch (e) {
        console.warn('mermaid render failed:', e);
        degradeMermaid([n]);
      }
    });
  }

  // CDN failed / render threw: show the raw diagram text in a code block.
  function degradeMermaid(nodes) {
    nodes.forEach(function (n) {
      if (n.querySelector('svg')) return;
      var code = n.textContent;
      n.outerHTML = '<pre><code class="language-mermaid">' + escapeHtml(code) + '</code></pre>';
    });
  }

  // Render into a container. Async-safe: if marked hasn't loaded yet we
  // show a placeholder, fetch it, then render (fallback if CDN is down).
  function renderInto(el, md) {
    if (!el) return;
    if (!md) { el.innerHTML = ''; return; }
    if (window.marked) {
      el.innerHTML = render(md);
      processMermaid(el);
      return;
    }
    el.innerHTML = '<div class="md-loading">Rendering…</div>';
    loadScript(MARKED_CDN, function (ok) {
      if (ok && window.marked) {
        el.innerHTML = render(md);
        processMermaid(el);
      } else {
        el.innerHTML = fallbackRender(md);
      }
    });
  }

  // Minimal built-in renderer (offline fallback): headings, bold, inline
  // code, code fences, links, line breaks.
  function fallbackRender(md) {
    var html = escapeHtml(md);
    html = html.replace(/```([\s\S]*?)```/g, function (_, code) {
      return '<pre><code>' + code + '</code></pre>';
    });
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
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

  window.Ob1Markdown = {
    render: render,
    renderInto: renderInto,
  };
})();
