import { describe, it, expect, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { api } from '../../services/adapter';
import { server } from '../../mocks/server';

describe('ApiAdapter', () => {
  beforeEach(() => {
    // Ensure a clean MSW handler slate before each test.
    // (testSetup.ts also resets in afterEach; this is belt-and-suspenders.)
    server.resetHandlers();
  });

  it('should use the correct base URL', () => {
    const adapter = api as any;
    expect(adapter.baseUrl).toBeDefined();
  });

  it('should make a GET request with correct parameters', async () => {
    const mockResponse = { data: { id: 1, name: 'test' } };

    server.use(
      http.get('/api/v1/test', ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.get('param')).toBe('value');
        return HttpResponse.json(mockResponse, {
          headers: { 'Content-Type': 'application/json' },
        });
      }),
    );

    const result = await api.get('/test', { param: 'value' });
    expect(result).toEqual(mockResponse);
  });

  it('should handle non-JSON responses', async () => {
    server.use(
      http.get('/api/v1/test', () => {
        return new HttpResponse('<html>Error</html>', {
          headers: { 'Content-Type': 'text/html' },
        });
      }),
    );

    await expect(api.get('/test')).rejects.toThrow('Invalid response format: text/html');
  });

  it('should handle API errors', async () => {
    server.use(
      http.get('/api/v1/test', () => {
        return new HttpResponse(null, {
          status: 500,
          statusText: 'Internal Server Error',
        });
      }),
    );

    // Default test timeout is 5s; the adapter retries 3 times with
    // exponential backoff (1s + 2s + 4s ≈ 7s), so we need a longer timeout.
    await expect(api.get('/test')).rejects.toThrow('API Error: 500 Internal Server Error');
  }, 15000);
});
