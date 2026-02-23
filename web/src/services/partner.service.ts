import { api } from './adapter';
import { Company } from '../types/domain';
import { ApiResponse } from '../types/api';

export const partnerService = {
  async getPartners(): Promise<ApiResponse<Company[]>> {
    return api.get<ApiResponse<Company[]>>('/partners');
  }
};
