import { useState, useEffect, useCallback } from 'react';
import { lifecycleService } from '../services/lifecycle.service';
import { LifecycleEvent, Jurisdiction } from '../types/domain';

interface UseQueryResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useLifecycle(jurisdiction?: Jurisdiction, status?: string): UseQueryResult<LifecycleEvent[]> {
  const [data, setData] = useState<LifecycleEvent[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await lifecycleService.getEvents(jurisdiction, status);
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [jurisdiction, status]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}
