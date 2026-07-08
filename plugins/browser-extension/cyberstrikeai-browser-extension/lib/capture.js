/** Capture filtering and HAR entry normalization. */

const STATIC_EXT =
  /\.(js|css|png|jpe?g|gif|svg|webp|ico|woff2?|ttf|eot|map|wasm)(\?|$)/i;

const STATIC_MIME_PREFIXES = [
  'image/',
  'font/',
  'audio/',
  'video/',
  'text/css',
];

function inferResourceType(url, responseHeaders) {
  if (STATIC_EXT.test(url || '')) return 'static';
  const ct = headerValue(responseHeaders, 'content-type').toLowerCase();
  for (const p of STATIC_MIME_PREFIXES) {
    if (ct.startsWith(p)) return 'static';
  }
  return 'other';
}

function headerValue(headers, name) {
  if (!headers || !headers.length) return '';
  const lower = name.toLowerCase();
  for (const h of headers) {
    if ((h.name || '').toLowerCase() === lower) return h.value || '';
  }
  return '';
}

function headerLines(headers) {
  if (!headers || !headers.length) return '';
  return headers.map((h) => `${h.name}: ${h.value}`).join('\n');
}

function truncate(str, max) {
  if (!str || str.length <= max) return str || '';
  return str.slice(0, max) + '\n… [truncated]';
}

function shouldCaptureEntry(entry, filterApiOnly) {
  if (!entry || !entry.url) return false;
  return mightCaptureRequest(entry.url, entry.resourceType, filterApiOnly);
}

/** Fast pre-filter before reading response body in devtools. */
function mightCaptureRequest(url, resourceType, filterApiOnly) {
  const rt = (resourceType || inferResourceType(url, null) || 'other').toLowerCase();
  if (filterApiOnly) {
    return rt === 'xhr' || rt === 'fetch' || rt === 'websocket';
  }
  if (rt === 'static') return false;
  if (STATIC_EXT.test(url || '')) return false;
  return true;
}

function summarizeHarEntry(harEntry, responseBody, resourceType) {
  const req = harEntry.request || {};
  const res = harEntry.response || {};
  const url = req.url || '';
  let path = '/';
  try {
    const u = new URL(url);
    path = u.pathname + (u.search || '');
  } catch (_) {
    path = url;
  }
  const shortPath = path.length > 80 ? path.slice(0, 77) + '...' : path;
  const title = `${req.method || 'GET'} ${shortPath}`;

  return {
    id: `${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    title,
    method: req.method || 'GET',
    url,
    resourceType: resourceType || 'other',
    requestHeaders: headerLines(req.headers),
    requestBody: truncate((req.postData && req.postData.text) || '', CSAI_LIMITS.MAX_REQUEST_BODY),
    responseStatus: res.status,
    responseHeaders: headerLines(res.headers),
    responseBody: truncate(responseBody || '', CSAI_LIMITS.MAX_RESPONSE_BODY),
    capturedAt: Date.now(),
  };
}

function summarizePageContext(page) {
  return {
    id: `page_${Date.now()}`,
    title: `PAGE ${page.title || page.url || ''}`.slice(0, 80),
    method: 'PAGE',
    url: page.url || '',
    resourceType: 'document',
    requestHeaders: '',
    requestBody: '',
    responseStatus: 0,
    responseHeaders: '',
    responseBody: '',
    pageTitle: page.title || '',
    isPageContext: true,
    capturedAt: Date.now(),
  };
}
