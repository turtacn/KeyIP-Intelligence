import { api } from './adapter';
import { Molecule } from '../types/domain';
import { ApiResponse } from '../types/api';

export const moleculeService = {
  async getMolecules(page = 1, pageSize = 20): Promise<ApiResponse<Molecule[]>> {
    return api.get<ApiResponse<Molecule[]>>('/molecules', { page, pageSize });
  },

  async getMoleculeById(id: string): Promise<ApiResponse<Molecule>> {
    return api.get<ApiResponse<Molecule>>(`/molecules/${id}`);
  }
};
