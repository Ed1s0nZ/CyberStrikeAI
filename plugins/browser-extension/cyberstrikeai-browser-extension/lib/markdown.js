/** Minimal Markdown → HTML (aligned with Burp plugin renderer). */

function mdEscapeHtml(s) {
  return String(s || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function mdHeadingLevel(s) {
  let i = 0;
  while (i < s.length && s[i] === '#') i++;
  if (i >= 1 && i <= 6 && i < s.length && /\s/.test(s[i])) return i;
  return 0;
}

function mdReplaceInlineCode(s) {
  let out = '';
  let inCode = false;
  let buf = '';
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '`') {
      if (!inCode) {
        inCode = true;
        buf = '';
      } else {
        out += `<code>${buf}</code>`;
        inCode = false;
      }
      continue;
    }
    if (inCode) buf += c;
    else out += c;
  }
  if (inCode) out += '`' + buf;
  return out;
}

function mdReplaceBold(s) {
  let out = '';
  let i = 0;
  while (i < s.length) {
    const start = s.indexOf('**', i);
    if (start < 0) {
      out += s.slice(i);
      break;
    }
    const end = s.indexOf('**', start + 2);
    if (end < 0) {
      out += s.slice(i);
      break;
    }
    out += s.slice(i, start) + '<b>' + s.slice(start + 2, end) + '</b>';
    i = end + 2;
  }
  return out;
}

function mdInlineFormat(text) {
  let escaped = mdEscapeHtml(text);
  escaped = mdReplaceInlineCode(escaped);
  escaped = mdReplaceBold(escaped);
  return escaped;
}

function markdownToHtml(markdown) {
  const lines = String(markdown || '').split(/\r?\n/);
  const css =
    'body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;font-size:13px;line-height:1.45;margin:10px;color:#111;}' +
    'code,pre{font-family:ui-monospace,Menlo,Consolas,monospace;}' +
    'code{font-size:0.95em;background:#f6f8fa;border:1px solid #e5e7eb;border-radius:4px;padding:0 4px;}' +
    'pre{font-size:0.95em;background:#f6f8fa;border:1px solid #e5e7eb;border-radius:6px;padding:10px;overflow:auto;}' +
    'pre code{background:transparent;border:none;padding:0;}' +
    'p{margin:0.55em 0;}ul{margin:0.4em 0 0.6em 1.2em;padding:0;}';

  let out = `<html><head><meta charset="utf-8"><style>${css}</style></head><body>`;
  let inCode = false;
  let inList = false;
  let codeBuf = '';

  for (const raw of lines) {
    const line = raw == null ? '' : raw;
    if (line.trim().startsWith('```')) {
      if (!inCode) {
        inCode = true;
        codeBuf = '';
      } else {
        out += `<pre><code>${mdEscapeHtml(codeBuf)}</code></pre>`;
        inCode = false;
      }
      continue;
    }
    if (inCode) {
      codeBuf += line + '\n';
      continue;
    }
    const trimmed = line.trim();
    if (!trimmed) {
      if (inList) {
        out += '</ul>';
        inList = false;
      }
      out += "<div style='height:6px'></div>";
      continue;
    }
    const h = mdHeadingLevel(trimmed);
    if (h > 0) {
      if (inList) {
        out += '</ul>';
        inList = false;
      }
      out += `<h${h}>${mdInlineFormat(trimmed.slice(h).trim())}</h${h}>`;
      continue;
    }
    if (trimmed.startsWith('- ') || trimmed.startsWith('* ')) {
      if (!inList) {
        out += '<ul>';
        inList = true;
      }
      out += `<li>${mdInlineFormat(trimmed.slice(2).trim())}</li>`;
      continue;
    }
    if (inList) {
      out += '</ul>';
      inList = false;
    }
    out += `<p>${mdInlineFormat(trimmed)}</p>`;
  }
  if (inCode) out += `<pre><code>${mdEscapeHtml(codeBuf)}</code></pre>`;
  if (inList) out += '</ul>';
  out += '</body></html>';
  return out;
}
