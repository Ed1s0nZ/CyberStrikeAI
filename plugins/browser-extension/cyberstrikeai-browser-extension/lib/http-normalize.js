/** HTTP/2 pseudo-header → HTTP/1.1 normalization for display and AI prompts.
 *  Raw HAR headers in storage are never modified. */

function parseHeaderLines(headerText) {
  const headers = [];
  for (const line of String(headerText || '').split(/\r?\n/)) {
    if (!line.trim()) continue;
    const idx = line.indexOf(':');
    if (idx <= 0) continue;
    headers.push({
      name: line.slice(0, idx).trim(),
      value: line.slice(idx + 1).trim(),
    });
  }
  return headers;
}

function urlParts(url) {
  try {
    const u = new URL(url || '');
    return {
      host: u.host,
      path: (u.pathname || '/') + (u.search || ''),
    };
  } catch (_) {
    return { host: '', path: '/' };
  }
}

/** Request line + headers (HTTP/1.1), no body. */
function normalizeRequestBlock(entry) {
  if (!entry || entry.isPageContext) return entry?.requestHeaders || '';

  const headers = parseHeaderLines(entry.requestHeaders);
  const fromUrl = urlParts(entry.url);
  let method = entry.method || 'GET';
  let path = fromUrl.path || '/';
  let host = fromUrl.host;

  const regular = [];
  let hasHost = false;

  for (const h of headers) {
    const name = h.name;
    const lower = name.toLowerCase();
    if (name.startsWith(':')) {
      if (lower === ':method') method = h.value || method;
      else if (lower === ':path') path = h.value || path;
      else if (lower === ':authority') host = h.value || host;
      continue;
    }
    if (lower === 'host') hasHost = true;
    regular.push(`${name}: ${h.value}`);
  }

  if (host && !hasHost) {
    regular.unshift(`Host: ${host}`);
  }

  if (!path.startsWith('/')) path = '/' + path;
  return `${method} ${path} HTTP/1.1\n${regular.join('\n')}`;
}

/** Status line + headers (HTTP/1.1), no body. */
function normalizeResponseBlock(entry) {
  if (!entry) return '';

  const headers = parseHeaderLines(entry.responseHeaders);
  let status = entry.responseStatus || 0;
  const regular = [];

  for (const h of headers) {
    const name = h.name;
    const lower = name.toLowerCase();
    if (name.startsWith(':')) {
      if (lower === ':status') {
        const n = parseInt(h.value, 10);
        if (!Number.isNaN(n)) status = n;
      }
      continue;
    }
    regular.push(`${name}: ${h.value}`);
  }

  const statusLine = `HTTP/1.1 ${status || '?'}`;
  return regular.length ? `${statusLine}\n${regular.join('\n')}` : statusLine;
}

function formatRequestDisplay(entry) {
  if (!entry) return '';
  if (entry.isPageContext) {
    return `PAGE ${entry.url || ''}\n\nTitle: ${entry.pageTitle || ''}`;
  }
  const block = normalizeRequestBlock(entry);
  const body = entry.requestBody || '';
  return body ? `${block}\n\n${body}` : block;
}

function formatResponseDisplay(entry) {
  if (!entry) return '';
  const block = normalizeResponseBlock(entry);
  const body = entry.responseBody || '';
  return body ? `${block}\n\n${body}` : block;
}
