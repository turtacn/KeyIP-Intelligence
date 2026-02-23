import { useState, useEffect, useCallback } from 'react';
import { infringementService } from '../services/infringement.service';
import { InfringementAlert, RiskLevel } from '../types/domain';

interface UseQueryResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useInfringement(riskLevel?: RiskLevel, page = 1, pageSize = 20): UseQueryResult<InfringementAlert[]> {
  const [data, setData] = useState<InfringementAlert[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await infringementService.getAlerts(riskLevel, page, pageSize);
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [riskLevel, page, pageSize]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}
