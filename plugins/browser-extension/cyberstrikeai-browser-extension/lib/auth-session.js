/** Session token expiry helpers (aligned with web auth.js patterns). */

const SESSION_TOKEN_EXPIRY_KEY = 'csai_token_expires_at';

/** Warn when remaining session time is below this (ms). */
const TOKEN_WARN_BEFORE_MS = 30 * 60 * 1000;

function parseExpiresAt(iso) {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d;
}

function isTokenExpiredByTime(expiresAtIso) {
  const d = parseExpiresAt(expiresAtIso);
  if (!d) return false;
  return d.getTime() <= Date.now();
}

function tokenExpiresWithin(expiresAtIso, withinMs) {
  const d = parseExpiresAt(expiresAtIso);
  if (!d) return false;
  return d.getTime() - Date.now() <= withinMs;
}

function formatTokenExpiryHint(expiresAtIso) {
  const d = parseExpiresAt(expiresAtIso);
  if (!d) return 'OK (token saved)';
  const ms = d.getTime() - Date.now();
  if (ms <= 0) return 'Session expired';
  const h = Math.floor(ms / 3600000);
  const m = Math.floor((ms % 3600000) / 60000);
  if (h >= 1) return `OK · ${h}h ${m}m left`;
  if (m >= 1) return `OK · ${m}m left`;
  return 'OK · expiring soon';
}

function isAuthHttpStatus(status) {
  return status === 401 || status === 403;
}

/** fetch() failed before HTTP response (server down, connection refused, etc.). */
function isNetworkFetchError(err) {
  if (!err) return false;
  if (err.network === true) return true;
  if (err.name === 'AbortError') return false;
  const msg = String(err.message || err);
  return err.name === 'TypeError' && /fetch|network|Failed to fetch/i.test(msg);
}

function attachHttpStatus(err, status) {
  if (err && typeof err === 'object') err.status = status;
  return err;
}
