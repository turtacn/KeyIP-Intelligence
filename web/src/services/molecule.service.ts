import { api } from './adapter';
import { Molecule } from '../types/domain';
import { ApiResponse } from '../types/api';

export const moleculeService = {
  async getMolecules(page = 1, pageSize = 20): Promise<ApiResponse<Molecule[]>> {
    return api.get<ApiResponse<Molecule[]>>('/molecules', { page, page_size: pageSize });
  },

  async getMoleculeById(id: string): Promise<ApiResponse<Molecule>> {
    return api.get<ApiResponse<Molecule>>(`/molecules/${id}`);
  },

  async searchMolecules(query: string): Promise<ApiResponse<Molecule[]>> {
    return api.get<ApiResponse<Molecule[]>>('/molecules', { q: query, page_size: 20 });
  }
};
