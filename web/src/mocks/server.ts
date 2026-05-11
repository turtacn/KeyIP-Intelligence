import { setupServer } from 'msw/node';
import type { RequestHandler } from 'msw';
import { handlers } from './handlers';

/**
 * Singleton MSW server for test use.
 *
 * Lifecycle is managed by testSetup.ts (start before all, reset after each,
 * close after all). Individual test files can override handlers temporarily
 * via `server.use(...)`; the global `afterEach` automatically restores defaults.
 */
export const server = setupServer(...handlers);

/**
 * Create an independent MSW server instance with the given handlers.
 *
 * Use this when a test file needs complete handler isolation (e.g. the global
 * handler set is inappropriate for the scenario). Only one MSW server can
 * listen at a time within a process, so do NOT call `.listen()` on a factory-
 * created server while the global singleton is active.
 *
 * Typical usage in a standalone test file:
 * ```
 * import { createMswServer } from '@/mocks/server';
 * const localServer = createMswServer(customHandlers);
 * beforeAll(() => localServer.listen());
 * afterEach(() => localServer.resetHandlers());
 * afterAll(() => localServer.close());
 * ```
 */
export function createMswServer(...customHandlers: RequestHandler[]) {
  return setupServer(...customHandlers);
}
