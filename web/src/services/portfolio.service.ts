import { api } from './adapter';
import { PortfolioScore, ConstellationData } from '../types/domain';
import { ApiResponse } from '../types/api';

export const portfolioService = {
  async getSummary(): Promise<ApiResponse<any>> {
    return api.get<ApiResponse<any>>('/portfolio/summary');
  },

  async getScores(): Promise<ApiResponse<PortfolioScore>> {
    return api.get<ApiResponse<PortfolioScore>>('/portfolio/scores');
  },

  async getCoverage(): Promise<ApiResponse<any>> {
    return api.get<ApiResponse<any>>('/portfolio/coverage');
  },

  async getConstellation(portfolioId: string): Promise<ConstellationData> {
    return api.get<ConstellationData>(`/portfolios/${portfolioId}/constellation`);
  }
};
