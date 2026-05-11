import { server } from './server';
import { handlers } from './handlers';

/**
 * Reset MSW handlers back to the original default set.
 *
 * Equivalent to calling `server.resetHandlers()` directly, but explicitly
 * references the full handler gallery for clarity. Use this in individual
 * test file `afterEach` hooks when tests temporarily override handlers
 * via `server.use()` and you want to ensure a clean slate.
 *
 * @example
 * ```ts
 * import { resetMswHandlers } from '@/mocks/resetHandlers';
 *
 * afterEach(() => {
 *   resetMswHandlers();
 * });
 * ```
 */
export function resetMswHandlers(): void {
  server.resetHandlers(...handlers);
}
