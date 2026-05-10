import { useState, useEffect, useCallback, useRef } from 'react';
import { healthService } from '../services/health.service';
import type { HealthDetail } from '../types/health';

interface UseHealthOptions {
  /** Enable automatic periodic refresh (default: true) */
  autoRefresh?: boolean;
  /** Refresh interval in milliseconds (default: 30000) */
  intervalMs?: number;
}

interface UseHealthResult {
  data: HealthDetail | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useHealth(options: UseHealthOptions = {}): UseHealthResult {
  const { autoRefresh = true, intervalMs = 30000 } = options;

  const [data, setData] = useState<HealthDetail | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await healthService.getHealthDetail();
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();

    if (autoRefresh && intervalMs > 0) {
      intervalRef.current = setInterval(fetchData, intervalMs);
    }

    return () => {
      if (intervalRef.current !== null) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [autoRefresh, intervalMs, fetchData]);

  return { data, loading, error, refetch: fetchData };
}
