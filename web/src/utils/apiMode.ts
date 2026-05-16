/**
 * API base URL resolution.
 *
 * KeyIP-Intelligence is a real software product — the frontend always
 * communicates with the live backend through the nginx proxy at `/api/v1`.
 *
 * Priority: env `VITE_API_BASE_URL` > default `/api/v1`
 */

/** Resolve the correct base URL for API requests. */
export function getBaseUrl(): string {
  const manualUrl = import.meta.env.VITE_API_BASE_URL;
  if (manualUrl) return manualUrl;
  return '/api/v1';
}
