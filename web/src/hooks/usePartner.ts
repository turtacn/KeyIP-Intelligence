import { useState, useEffect, useCallback } from 'react';
import { partnerService } from '../services/partner.service';
import { Company } from '../types/domain';

interface UseQueryResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function usePartners(): UseQueryResult<Company[]> {
  const [data, setData] = useState<Company[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await partnerService.getPartners();
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}
