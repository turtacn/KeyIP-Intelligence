import { api } from './adapter';
import { Patent } from '../types/domain';
import { ApiResponse } from '../types/api';

export const patentService = {
  async getPatents(page = 1, pageSize = 20, query = ''): Promise<ApiResponse<Patent[]>> {
    return api.get<ApiResponse<Patent[]>>('/patents', { page, pageSize, query });
  },

  async getPatentById(id: string): Promise<ApiResponse<Patent>> {
    return api.get<ApiResponse<Patent>>(`/patents/${id}`);
  }
};
