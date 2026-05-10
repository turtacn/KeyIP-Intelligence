/**
 * API Mode switching utility.
 *
 * Three modes are supported:
 *  - mock (default): MSW intercepts all requests, base URL is `/api/v1`
 *  - proxy:         Requests go to http://localhost:8080/api/v1 (local backend)
 *  - live:          Requests go to the production API endpoint
 *
 * Priority: localStorage `keyip-api-mode` > env `VITE_API_MODE` > default `mock`
 */

export type ApiMode = 'mock' | 'proxy' | 'live';

const STORAGE_KEY = 'keyip-api-mode';

const MODE_BASE_URLS: Record<ApiMode, string> = {
  mock: '/api/v1',
  proxy: 'http://localhost:8080/api/v1',
  live: 'https://api.keyip.io/api/v1',
};

/** Return the currently active API mode. */
export function getApiMode(): ApiMode {
  // 1. Runtime override from localStorage (set by the UI switcher)
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === 'mock' || stored === 'proxy' || stored === 'live') {
    return stored;
  }

  // 2. Build-time / env override
  const env = import.meta.env.VITE_API_MODE;
  if (env === 'mock' || env === 'proxy' || env === 'live') {
    return env;
  }

  return 'mock';
}

/** Switch API mode at runtime. Triggers a full page reload so MSW lifecycle is clean. */
export function setApiMode(mode: ApiMode): void {
  const prev = getApiMode();
  if (prev === mode) return;
  localStorage.setItem(STORAGE_KEY, mode);
  window.location.reload();
}

/** Resolve the correct base URL for the current mode. */
export function getBaseUrl(): string {
  const mode = getApiMode();

  // VITE_API_BASE_URL, if set, always wins (manual override)
  const manualUrl = import.meta.env.VITE_API_BASE_URL;
  if (manualUrl) return manualUrl;

  return MODE_BASE_URLS[mode];
}

/** Convenience check for mock mode. */
export function isMockMode(): boolean {
  return getApiMode() === 'mock';
}
