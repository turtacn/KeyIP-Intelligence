/**
 * Unified test setup for KeyIP-Intelligence.
 *
 * Loaded by vitest before all test files (configured via vitest.config.ts).
 * Provides:
 * - jest-dom DOM matchers (toBeInTheDocument, toBeDisabled, etc.)
 * - Browser API polyfills (matchMedia, ResizeObserver) not available in jsdom
 * - MSW server lifecycle (start once, reset after each test, close at end)
 * - Cross-test isolation (localStorage cleared, vi mocks cleared, MSW handlers reset)
 */

import '@testing-library/jest-dom';
import { afterAll, afterEach, beforeAll, vi } from 'vitest';
import { server } from '../mocks/server';

// ── Browser API mocks ──────────────────────────────────────────────────────
// jsdom does not implement these, but many UI components depend on them.

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};

// ── MSW lifecycle ──────────────────────────────────────────────────────────
// MSW intercepts fetch/XMLHttpRequest at the process level.  A single server
// instance handles all requests.  After each test the handler list is reset
// to defaults so per-test overrides never leak to the next test.

beforeAll(() => server.listen({ onUnhandledRequest: 'warn' }));

afterEach(() => {
  server.resetHandlers();
  localStorage.clear();
  vi.clearAllMocks();
});

afterAll(() => server.close());
