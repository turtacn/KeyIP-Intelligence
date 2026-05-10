import { getBaseUrl } from '../utils/apiMode';

/** Retry policy for API requests */
export interface RetryPolicy {
  /** Maximum number of retry attempts (default: 3) */
  maxRetries?: number;
  /** Base delay in milliseconds for exponential backoff (default: 1000) */
  baseDelayMs?: number;
  /**
   * Custom predicate to determine if an error should be retried.
   * If not provided, the default logic retries on 5xx and network errors only.
   */
  shouldRetry?: (error: unknown) => boolean;
  /** When true, suppresses the automatic error toast for this request */
  suppressToast?: boolean;
}

export interface ApiAdapter {
  baseUrl: string;
  get<T>(path: string, params?: Record<string, unknown>, retryPolicy?: RetryPolicy): Promise<T>;
  post<T>(path: string, body: unknown, retryPolicy?: RetryPolicy): Promise<T>;
  put<T>(path: string, body: unknown, retryPolicy?: RetryPolicy): Promise<T>;
  delete<T>(path: string, retryPolicy?: RetryPolicy): Promise<T>;
}

/** Default retryability check: retry on 5xx and network errors only */
function isRetryableError(error: unknown): boolean {
  // Network errors (fetch threw, e.g. TypeError for CORS / DNS / timeout)
  if (error instanceof TypeError) {
    return true;
  }
  // HTTP 5xx server errors
  if (error instanceof Error && error.message.startsWith('API Error:')) {
    const match = error.message.match(/API Error: (\d+)/);
    if (match) {
      const status = parseInt(match[1], 10);
      return status >= 500;
    }
  }
  return false;
}

class FetchAdapter implements ApiAdapter {
  private defaultRetryPolicy: RetryPolicy;

  /** Dynamic base URL — always reads from the current apiMode. */
  get baseUrl(): string {
    return getBaseUrl();
  }

  constructor(retryPolicy?: RetryPolicy) {
    this.defaultRetryPolicy = retryPolicy ?? {};
    console.log('[ApiAdapter] Initialized (dynamic baseUrl)');
  }

  /** Update the default retry policy at runtime */
  setRetryPolicy(policy: RetryPolicy): void {
    this.defaultRetryPolicy = policy;
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Low-level request execution.
   * @param silent - When true, suppresses console.error on failure (used during retries)
   */
  private async request<T>(path: string, options: RequestInit, silent = false): Promise<T> {
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
      // Return the full response object, assuming T is ApiResponse<D>
      return result;
    } catch (error) {
      if (!silent) {
        console.error(`[API] Fetch failed for ${fullUrl}:`, error);
      }
      throw error;
    }
  }

  /**
   * Executes a request with exponential backoff retry logic.
   * Retries only on 5xx and network errors (or custom shouldRetry if provided).
   * Error logging is suppressed during retry attempts and only shown on final failure.
   */
  private async requestWithRetry<T>(
    path: string,
    options: RequestInit,
    retryPolicy?: RetryPolicy,
  ): Promise<T> {
    const policy = retryPolicy ?? this.defaultRetryPolicy;
    const maxRetries = policy.maxRetries ?? 3;
    const baseDelayMs = policy.baseDelayMs ?? 1000;
    const customShouldRetry = policy.shouldRetry;

    let lastError: unknown;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        // Suppress console.error on retry attempts (silent retries)
        return await this.request<T>(path, options, attempt > 0);
      } catch (error) {
        lastError = error;

        const isRetryable = customShouldRetry
          ? customShouldRetry(error)
          : isRetryableError(error);

        if (!isRetryable || attempt >= maxRetries) {
          // Log error on final failure (first attempt is already logged)
          if (attempt > 0) {
            console.error(
              `[API] Request failed after ${attempt + 1} attempt(s) for ${path}:`,
              error,
            );
          }
          // Show error toast unless suppressed
          if (!policy.suppressToast) {
            const message =
              error instanceof Error
                ? error.message
                : 'An unexpected network error occurred. Please try again.';
            // Dynamically import to avoid circular dependency at module level
            import('../utils/notificationBridge').then(({ notify }) => {
              notify('error', 'API Error', message);
            });
          }
          throw error;
        }

        // Exponential backoff: 1s, 2s, 4s, ...
        const delay = baseDelayMs * Math.pow(2, attempt);
        console.log(
          `[API] Retrying ${path} in ${delay}ms (attempt ${attempt + 1}/${maxRetries})`,
        );
        await this.sleep(delay);
      }
    }

    throw lastError; // Should never reach here
  }

  async get<T>(path: string, params?: Record<string, unknown>, retryPolicy?: RetryPolicy): Promise<T> {
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

    return this.requestWithRetry<T>(url, { method: 'GET' }, retryPolicy);
  }

  async post<T>(path: string, body: unknown, retryPolicy?: RetryPolicy): Promise<T> {
    return this.requestWithRetry<T>(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }, retryPolicy);
  }

  async put<T>(path: string, body: unknown, retryPolicy?: RetryPolicy): Promise<T> {
    return this.requestWithRetry<T>(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }, retryPolicy);
  }

  async delete<T>(path: string, retryPolicy?: RetryPolicy): Promise<T> {
    return this.requestWithRetry<T>(path, { method: 'DELETE' }, retryPolicy);
  }
}

console.log('[ApiAdapter] Creating API client (dynamic mode)...');

export const api = new FetchAdapter();
