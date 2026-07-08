/** In-memory cache for projects / roles lists (per baseUrl + token). */

const CATALOG_CACHE_TTL_MS = 5 * 60 * 1000;

const catalogCache = {
  baseUrl: '',
  token: '',
  projects: null,
  roles: null,
  fetchedAt: 0,
};

function catalogCacheValid(baseUrl, token) {
  if (!catalogCache.fetchedAt) return false;
  if (catalogCache.baseUrl !== baseUrl || catalogCache.token !== token) return false;
  return Date.now() - catalogCache.fetchedAt < CATALOG_CACHE_TTL_MS;
}

function invalidateCatalogCache() {
  catalogCache.projects = null;
  catalogCache.roles = null;
  catalogCache.fetchedAt = 0;
}

async function fetchCatalogCached(baseUrl, token, signal) {
  if (catalogCacheValid(baseUrl, token) && catalogCache.projects && catalogCache.roles) {
    return { projects: catalogCache.projects, roles: catalogCache.roles };
  }
  const [projects, roles] = await Promise.all([
    fetchProjects(baseUrl, token, signal),
    fetchRoles(baseUrl, token, signal),
  ]);
  catalogCache.baseUrl = baseUrl;
  catalogCache.token = token;
  catalogCache.projects = projects;
  catalogCache.roles = roles;
  catalogCache.fetchedAt = Date.now();
  return { projects, roles };
}
