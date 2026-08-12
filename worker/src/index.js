// Off-By-One SEO prerender Worker (ob1.it.com)
//
// Mirrors the discontinuousmind.com pattern: bots get static HTML from
// the repo (site/ dir on raw.githubusercontent.com), real browsers pass
// through to the live app origin (proxied A record → the Go app).
//
// Route: ob1.it.com/*

const ORIGIN_RAW = 'https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/site';

// Bot detection — broad: search engines, AI crawlers, social scrapers,
// link checkers, curl/wget and CLI tools. Bane's policy: ANY bot gets
// the static version ("any bot can scan this").
const BOT_RE =
  /bot|crawl|spider|slurp|bingpreview|facebookexternalhit|twitterbot|linkedinbot|pinterest|tumblr|embed|curl|wget|python-requests|go-http-client|java\/|okhttp|scrapy|headless|phantomjs|semrush|ahrefs|mj12bot|dotbot|petalbot|yandex|baidu|duckduckbot|gptbot|claudebot|ccbot|bytespider|perplexitybot|googlebot|bingbot|monitor|pingdom|uptimerobot|statuscake|nagios/i;

// raw.githubusercontent.com sends a sandbox CSP that blocks all scripts.
const STRIP_HEADERS = [
  'content-security-policy',
  'x-content-type-options',
  'cross-origin-resource-policy',
  'cross-origin-opener-policy',
  'x-frame-options',
  'sandbox',
];

function isBot(ua) {
  return BOT_RE.test(ua || '');
}

// Map a public path to the matching file in the repo's site/ dir.
function mapPath(pathname) {
  if (pathname === '/' || pathname === '' || pathname === '/index.html') return '/index.html';
  if (pathname === '/sitemap.xml' || pathname === '/robots.txt') return pathname;
  if (pathname.startsWith('/classes/')) {
    const stem = pathname.slice('/classes/'.length).replace(/\.html$/, '');
    if (/^[\w-]+$/.test(stem)) return '/classes/' + stem + '.html';
  }
  return null;
}

async function serveStatic(staticPath) {
  let res;
  try {
    // no-store prevents Cloudflare from caching origin subrequests
    // (raw.githubusercontent sends max-age=300 — stale bot content).
    res = await fetch(ORIGIN_RAW + staticPath, { cache: 'no-store', cf: { cacheTtl: 0 } });
  } catch (e) {
    // Local miniflare dev doesn't implement the cache field — retry plain.
    res = await fetch(ORIGIN_RAW + staticPath, { cf: { cacheTtl: 0 } });
  }
  const headers = new Headers(res.headers);
  STRIP_HEADERS.forEach((h) => headers.delete(h));
  headers.set('content-type', res.headers.get('content-type') || 'text/html; charset=utf-8');
  return new Response(res.body, { status: res.status, headers });
}

export default {
  async fetch(request) {
    const url = new URL(request.url);
    const ua = request.headers.get('user-agent') || '';

    // ACME challenges and .well-known must always reach the origin
    // (Caddy needs them for Let's Encrypt through the proxy).
    if (url.pathname.startsWith('/.well-known/')) {
      return fetch(request);
    }

    if (isBot(ua)) {
      const staticPath = mapPath(url.pathname);
      if (staticPath) return serveStatic(staticPath);
      // Bots hitting app paths (api, assets) get the landing page.
      return serveStatic('/index.html');
    }

    // Real browsers: pass through to the origin (proxied A record →
    // the Go app on the catalog box). The app is read-only publicly;
    // the chat WebSocket is disabled there, so no upgrade proxying
    // is needed.
    return fetch(request);
  },
};
