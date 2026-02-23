import { api } from './adapter';
import { InfringementAlert, RiskLevel } from '../types/domain';
import { ApiResponse } from '../types/api';

export const infringementService = {
  async getAlerts(riskLevel?: RiskLevel, page = 1, pageSize = 20): Promise<ApiResponse<InfringementAlert[]>> {
    const params: Record<string, any> = { page, pageSize };
    if (riskLevel) params.riskLevel = riskLevel;

    return api.get<ApiResponse<InfringementAlert[]>>('/alerts', params);
  }
};
