import { api } from './adapter';
import { Patent } from '../types/domain';
import { ApiResponse } from '../types/api';

export const patentService = {
  async getPatents(page = 1, pageSize = 20, query = '', searchType = 'text'): Promise<ApiResponse<Patent[]>> {
    // Map frontend 'searchType' to backend 'query_type'
    return api.post<ApiResponse<Patent[]>>('/patents/search', { page, page_size: pageSize, query, query_type: searchType });
  },

  async getPatentById(id: string): Promise<ApiResponse<Patent>> {
    return api.get<ApiResponse<Patent>>(`/patents/${id}`);
  }
};
