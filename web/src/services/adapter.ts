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
    const response = await fetch(`${this.baseUrl}${path}`, options);

    if (!response.ok) {
      throw new Error(`API Error: ${response.status} ${response.statusText}`);
    }

    const result = await response.json();
    return result.data; // Unwrapping the envelope based on ApiResponse<T>
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
// In mock mode, we use relative paths which MSW intercepts
// In real mode, we use the VITE_API_BASE_URL
// Default to 'mock' if not specified, to align with Docker all-in-one default
const mode = import.meta.env.VITE_API_MODE || 'mock';
const isMock = mode === 'mock' || import.meta.env.DEV;
const baseUrl = isMock ? '' : (import.meta.env.VITE_API_BASE_URL || '/api/openapi/v1');

export const api = new FetchAdapter(baseUrl);
