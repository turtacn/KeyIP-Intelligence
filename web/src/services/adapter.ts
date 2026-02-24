export interface ApiAdapter {
  baseUrl: string;
  get<T>(path: string, params?: Record<string, unknown>): Promise<T>;
  post<T>(path: string, body: unknown): Promise<T>;
  put<T>(path: string, body: unknown): Promise<T>;
  delete<T>(path: string): Promise<T>;
}

class FetchAdapter implements ApiAdapter {
  baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(path: string, options: RequestInit): Promise<T> {
    const fullUrl = `${this.baseUrl}${path}`;
    console.log(`[API] Requesting: ${fullUrl}`, options);

    try {
      const response = await fetch(fullUrl, options);
      console.log(`[API] Response status: ${response.status} for ${fullUrl}`);

      if (!response.ok) {
        throw new Error(`API Error: ${response.status} ${response.statusText}`);
      }

      const contentType = response.headers.get('content-type');
      if (!contentType || !contentType.includes('application/json')) {
        const text = await response.text();
        console.error(`[API] Expected JSON but got ${contentType}:`, text.substring(0, 100));
        // This is often where "Unexpected token <" comes from (HTML fallback)
        throw new Error(`Invalid response format: ${contentType}`);
      }

      const result = await response.json();
      return result.data;
    } catch (error) {
      console.error(`[API] Fetch failed for ${fullUrl}:`, error);
      throw error;
    }
  }

  async get<T>(path: string, params?: Record<string, unknown>): Promise<T> {
    let url = path;
    if (params) {
      const searchParams = new URLSearchParams();
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          searchParams.append(key, String(value));
        }
      });
      url += `?${searchParams.toString()}`;
    }

    return this.request<T>(url, { method: 'GET' });
  }

  async post<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }

  async put<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }

  async delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' });
  }
}

// Environment-based base URL
// Always use the full path prefix to match MSW handlers, even in mock mode.
const mode = import.meta.env.VITE_API_MODE || 'mock';
const isMock = mode === 'mock' || import.meta.env.DEV;
const baseUrl = import.meta.env.VITE_API_BASE_URL || '/api/openapi/v1';

console.log('[ApiAdapter] Initialized', { mode, isMock, baseUrl });

export const api = new FetchAdapter(baseUrl);
