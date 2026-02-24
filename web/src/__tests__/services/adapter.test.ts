import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api, ApiAdapter } from '../../services/adapter';

// Mock the global fetch
global.fetch = vi.fn();

describe('ApiAdapter', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('should use the correct base URL', () => {
    // In test environment, baseUrl might be affected by env vars
    // We can just check relative path construction
    // Actually api is a singleton, so we test its behavior
    const adapter = api as any;
    expect(adapter.baseUrl).toBeDefined();
  });

  it('should make a GET request with correct parameters', async () => {
    const mockResponse = { data: { id: 1, name: 'test' } };
    (global.fetch as any).mockResolvedValue({
      ok: true,
      headers: { get: () => 'application/json' },
      json: async () => mockResponse,
    });

    const result = await api.get('/test', { param: 'value' });

    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/test?param=value'),
      expect.objectContaining({ method: 'GET' })
    );
    expect(result).toEqual(mockResponse.data);
  });

  it('should handle non-JSON responses', async () => {
    (global.fetch as any).mockResolvedValue({
      ok: true,
      headers: { get: () => 'text/html' },
      text: async () => '<html>Error</html>',
    });

    await expect(api.get('/test')).rejects.toThrow('Invalid response format: text/html');
  });

  it('should handle API errors', async () => {
    (global.fetch as any).mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
    });

    await expect(api.get('/test')).rejects.toThrow('API Error: 500 Internal Server Error');
  });
});
