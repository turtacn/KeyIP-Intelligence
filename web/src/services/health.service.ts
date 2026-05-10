import { api } from './adapter';
import type { HealthDetail, HealthSummary } from '../types/health';
import type { ApiResponse } from '../types/api';

export const healthService = {
  /** Simple liveness check */
  async getHealth(): Promise<ApiResponse<HealthSummary>> {
    return api.get<ApiResponse<HealthSummary>>('/healthz');
  },

  /** Detailed health check with per-service status and response times */
  async getHealthDetail(): Promise<ApiResponse<HealthDetail>> {
    return api.get<ApiResponse<HealthDetail>>('/healthz/detail');
  },
};
