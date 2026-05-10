import { api } from './adapter';
import { Patent, FamilyResponse } from '../types/domain';
import { ApiResponse } from '../types/api';

export const patentService = {
  async getPatents(page = 1, pageSize = 20, query = ''): Promise<ApiResponse<Patent[]>> {
    return api.post<ApiResponse<Patent[]>>('/patents/search', { page, page_size: pageSize, query });
  },

  async getPatentById(id: string): Promise<ApiResponse<Patent>> {
    return api.get<ApiResponse<Patent>>(`/patents/${id}`);
  },

  async getFamily(id: string): Promise<ApiResponse<FamilyResponse>> {
    return api.get<ApiResponse<FamilyResponse>>(`/patents/${id}/family`);
  }
};
