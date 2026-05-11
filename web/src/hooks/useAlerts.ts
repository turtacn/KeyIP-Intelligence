import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '../services/adapter';
import { isMockMode } from '../utils/apiMode';
import type { ApiResponse } from '../types/api';

/** A unified alert item representing either a lifecycle deadline or an API alert. */
export interface AlertItem {
  id: string;
  type: 'deadline' | 'infringement' | 'system';
  title: string;
  message?: string;
  severity: 'high' | 'medium' | 'low';
  createdAt: string;
  read: boolean;
  relatedEntityId?: string;
}

interface UseAlertsReturn {
  alerts: AlertItem[];
  unreadCount: number;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

function toAlertId(prefix: string, rawId: unknown): string {
  return `${prefix}-${rawId ?? Math.random().toString(36).slice(2, 10)}`;
}

function toSeverity(riskLevel?: string, status?: string): 'high' | 'medium' | 'low' {
  if (status === 'overdue') return 'high';
  if (riskLevel === 'HIGH') return 'high';
  if (riskLevel === 'MEDIUM' || status === 'pending') return 'medium';
  return 'low';
}

function parseDate(raw: string | undefined | null): string {
  if (!raw) return new Date().toISOString();
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? new Date().toISOString() : d.toISOString();
}

/**
 * Polls the backend for lifecycle deadlines and API alerts.
 *
 * - Fetches from `/lifecycle/deadlines` and `/alerts` in parallel
 * - Merges and sorts results by creation date (newest first)
 * - Only polls in non-mock mode
 *
 * @param pollIntervalMs Polling interval in milliseconds (default 60 000 = 1 min)
 */
export function useAlerts(pollIntervalMs = 60000): UseAlertsReturn {
  const [alerts, setAlerts] = useState<AlertItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchAlerts = useCallback(async () => {
    if (isMockMode()) {
      setLoading(false);
      setAlerts([]);
      return;
    }

    try {
      const [deadlinesResult, alertsResult] = await Promise.allSettled([
        api.get<ApiResponse<unknown[]>>('/lifecycle/deadlines'),
        api.get<ApiResponse<unknown[]>>('/alerts'),
      ]);

      const items: AlertItem[] = [];

      // Process lifecycle deadlines
      if (deadlinesResult.status === 'fulfilled') {
        const data = deadlinesResult.value?.data;
        if (Array.isArray(data)) {
          (data as Record<string, unknown>[]).forEach((d) => {
            items.push({
              id: toAlertId('dl', d.id),
              type: 'deadline',
              title: d.eventType
                ? `${String(d.eventType)} Due`
                : 'Upcoming Deadline',
              message: d.dueDate
                ? `Due ${new Date(String(d.dueDate)).toLocaleDateString()}`
                : undefined,
              severity: toSeverity(undefined, String(d.status ?? '')),
              createdAt: parseDate(d.dueDate as string | undefined | null),
              read: false,
              relatedEntityId: d.patentId as string | undefined,
            });
          });
        }
      }

      // Process infringement / system alerts
      if (alertsResult.status === 'fulfilled') {
        const data = alertsResult.value?.data;
        if (Array.isArray(data)) {
          (data as Record<string, unknown>[]).forEach((a) => {
            const riskLevel = a.riskLevel as string | undefined;
            items.push({
              id: toAlertId('alert', a.id),
              type: 'infringement',
              title:
                (a.title as string | undefined) ??
                (riskLevel === 'HIGH'
                  ? 'High Risk Alert'
                  : riskLevel === 'MEDIUM'
                    ? 'Medium Risk Alert'
                    : 'Alert'),
              message: (a.message ?? a.description) as string | undefined,
              severity: toSeverity(riskLevel, undefined),
              createdAt: parseDate(
                (a.detectedAt ?? a.createdAt) as string | undefined | null,
              ),
              read: a.status === 'reviewed' || a.read === true,
              relatedEntityId: (a.targetPatentId ?? a.triggerMoleculeId) as
                | string
                | undefined,
            });
          });
        }
      }

      // Sort newest first
      items.sort(
        (a, b) =>
          new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
      );

      setAlerts(items);
      setError(null);
    } catch (err) {
      console.error('[useAlerts] Failed to fetch alerts:', err);
      setError(
        err instanceof Error ? err.message : 'Failed to fetch alerts',
      );
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    fetchAlerts();
  }, [fetchAlerts]);

  // Polling interval
  useEffect(() => {
    if (isMockMode()) return;

    intervalRef.current = setInterval(fetchAlerts, pollIntervalMs);
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [fetchAlerts, pollIntervalMs]);

  const unreadCount = alerts.filter((a) => !a.read).length;

  return { alerts, unreadCount, loading, error, refetch: fetchAlerts };
}
