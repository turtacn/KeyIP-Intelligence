import { useState, useEffect, useCallback } from 'react';
import { portfolioService } from '../services/portfolio.service';
import { PortfolioScore } from '../types/domain';

interface UseQueryResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function usePortfolio(): {
  summary: UseQueryResult<any>;
  scores: UseQueryResult<PortfolioScore>;
  coverage: UseQueryResult<any>;
} {
  const [summary, setSummary] = useState<any | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(true);
  const [summaryError, setSummaryError] = useState<string | null>(null);

  const [scores, setScores] = useState<PortfolioScore | null>(null);
  const [scoresLoading, setScoresLoading] = useState(true);
  const [scoresError, setScoresError] = useState<string | null>(null);

  const [coverage, setCoverage] = useState<any | null>(null);
  const [coverageLoading, setCoverageLoading] = useState(true);
  const [coverageError, setCoverageError] = useState<string | null>(null);

  const fetchSummary = useCallback(async () => {
    setSummaryLoading(true);
    try {
      const response = await portfolioService.getSummary();
      setSummary(response.data);
    } catch (err) {
      setSummaryError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setSummaryLoading(false);
    }
  }, []);

  const fetchScores = useCallback(async () => {
    setScoresLoading(true);
    try {
      const response = await portfolioService.getScores();
      setScores(response.data);
    } catch (err) {
      setScoresError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setScoresLoading(false);
    }
  }, []);

  const fetchCoverage = useCallback(async () => {
    setCoverageLoading(true);
    try {
      const response = await portfolioService.getCoverage();
      setCoverage(response.data);
    } catch (err) {
      setCoverageError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setCoverageLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSummary();
    fetchScores();
    fetchCoverage();
  }, [fetchSummary, fetchScores, fetchCoverage]);

  return {
    summary: { data: summary, loading: summaryLoading, error: summaryError, refetch: fetchSummary },
    scores: { data: scores, loading: scoresLoading, error: scoresError, refetch: fetchScores },
    coverage: { data: coverage, loading: coverageLoading, error: coverageError, refetch: fetchCoverage },
  };
}
