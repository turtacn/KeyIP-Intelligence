import { api } from './adapter';
import { Patent, FamilyResponse } from '../types/domain';
import { ApiResponse } from '../types/api';

export interface PatentSearchFilters {
  scope?: 'title' | 'abstract' | 'fulltext' | 'all';
  filingDateFrom?: string;
  filingDateTo?: string;
  publicationDateFrom?: string;
  publicationDateTo?: string;
  jurisdictions?: string[];
}

export const patentService = {
  async getPatents(page = 1, pageSize = 20, query = '', filters?: PatentSearchFilters): Promise<ApiResponse<Patent[]>> {
    return api.post<ApiResponse<Patent[]>>('/patents/search', {
      page,
      page_size: pageSize,
      query,
      ...(filters?.scope && filters.scope !== 'all' ? { scope: filters.scope } : {}),
      ...(filters?.filingDateFrom ? { filing_date_from: filters.filingDateFrom } : {}),
      ...(filters?.filingDateTo ? { filing_date_to: filters.filingDateTo } : {}),
      ...(filters?.publicationDateFrom ? { publication_date_from: filters.publicationDateFrom } : {}),
      ...(filters?.publicationDateTo ? { publication_date_to: filters.publicationDateTo } : {}),
      ...(filters?.jurisdictions?.length ? { jurisdictions: filters.jurisdictions } : {}),
    });
  },

  async getPatentById(id: string): Promise<ApiResponse<Patent>> {
    return api.get<ApiResponse<Patent>>(`/patents/${id}`);
  },

  async getFamily(id: string): Promise<ApiResponse<FamilyResponse>> {
    return api.get<ApiResponse<FamilyResponse>>(`/patents/${id}/family`);
  }
};
