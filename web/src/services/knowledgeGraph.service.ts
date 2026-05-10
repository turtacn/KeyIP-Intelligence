import { api } from './adapter';
import { ApiResponse } from '../types/api';

export interface CitationRef {
  patent_number: string;
  title?: string;
  relation: 'cites' | 'cited_by';
}

export interface CitationNetworkResponse {
  patent_id: string;
  patent_number: string;
  title: string;
  forward_citations: CitationRef[];
  backward_citations: CitationRef[];
  total_citations: number;
}

export const knowledgeGraphService = {
  async getCitationNetwork(patentId: string): Promise<ApiResponse<CitationNetworkResponse>> {
    return api.get<ApiResponse<CitationNetworkResponse>>(`/patents/${patentId}/citations`);
  },
};
