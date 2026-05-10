import { useState, useEffect, useCallback } from 'react';
import { knowledgeGraphService, CitationNetworkResponse } from '../services/knowledgeGraph.service';

interface UseQueryResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useCitationNetwork(patentId?: string): UseQueryResult<CitationNetworkResponse> {
  const [data, setData] = useState<CitationNetworkResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    if (!patentId) return;
    setLoading(true);
    setError(null);
    try {
      const response = await knowledgeGraphService.getCitationNetwork(patentId);
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [patentId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}
