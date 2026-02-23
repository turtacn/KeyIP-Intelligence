// API response envelopes

export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
  pagination?: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface ApiError {
  code: number;
  message: string;
  details?: Record<string, unknown>;
}
