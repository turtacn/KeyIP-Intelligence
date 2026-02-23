import { api } from './adapter';
import { DashboardMetrics } from '../types/domain';
import { ApiResponse } from '../types/api';

export const dashboardService = {
  async getMetrics(): Promise<ApiResponse<DashboardMetrics>> {
    return api.get<ApiResponse<DashboardMetrics>>('/dashboard/metrics');
  }
};
