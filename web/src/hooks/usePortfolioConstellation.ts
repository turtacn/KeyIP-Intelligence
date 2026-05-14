import { useState, useEffect, useCallback } from 'react';
import { portfolioService } from '../services/portfolio.service';
import { ConstellationData } from '../types/domain';

interface UseConstellationResult {
  data: ConstellationData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function usePortfolioConstellation(portfolioId?: string): UseConstellationResult {
  const [data, setData] = useState<ConstellationData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchConstellation = useCallback(async () => {
    const pid = portfolioId || 'default';
    setLoading(true);
    setError(null);
    try {
      const response = await portfolioService.getConstellation(pid);
      setData((response as any).data || response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [portfolioId]);

  useEffect(() => {
    fetchConstellation();
  }, [fetchConstellation]);

  return { data, loading, error, refetch: fetchConstellation };
}
