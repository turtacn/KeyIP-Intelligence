import React, { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useHealth } from '../../hooks/useHealth';
import type { HealthServiceDetail, ServiceStatus } from '../../types/health';
import { KNOWN_SERVICES } from '../../types/health';
import PageError from '../../components/ui/PageError';
import EmptyState from '../../components/ui/EmptyState';
import Button from '../../components/ui/Button';
import {
  Activity,
  Heart,
  RotateCcw,
  RefreshCw,
  Clock,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Server,
} from 'lucide-react';

// ─── Helpers ────────────────────────────────────────────────

function statusBg(status: ServiceStatus): string {
  switch (status) {
    case 'healthy':
      return 'bg-green-500';
    case 'degraded':
      return 'bg-yellow-500';
    case 'unhealthy':
      return 'bg-red-500';
    default:
      return 'bg-slate-400';
  }
}

function statusIcon(status: ServiceStatus) {
  switch (status) {
    case 'healthy':
      return CheckCircle;
    case 'degraded':
      return AlertTriangle;
    case 'unhealthy':
      return XCircle;
    default:
      return Server;
  }
}

function formatUptime(seconds: number, t: (key: string, opts?: Record<string, unknown>) => string): string {
  if (!seconds || seconds <= 0 || isNaN(seconds)) return '';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const parts: string[] = [];
  if (days > 0) parts.push(t('health.uptime_days', { count: days }));
  if (hours > 0) parts.push(t('health.uptime_hours', { count: hours }));
  parts.push(t('health.uptime_minutes', { count: minutes }));
  return parts.join(' ');
}

function formatResponseTime(ms: number): string {
  if (!ms || ms <= 0 || isNaN(ms)) return 'N/A';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

// ─── Skeleton ───────────────────────────────────────────────

const SkeletonHealthCard: React.FC = () => (
  <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-5 animate-pulse">
    <div className="flex items-center justify-between mb-4">
      <div className="h-5 bg-slate-200 rounded w-24" />
      <div className="h-3 w-16 bg-slate-200 rounded" />
    </div>
    <div className="flex items-center gap-2">
      <div className="w-3 h-3 bg-slate-200 rounded-full" />
      <div className="h-4 bg-slate-200 rounded w-16" />
    </div>
  </div>
);

const HealthPageSkeleton: React.FC = () => (
  <div className="space-y-6 pb-12">
    <div className="h-8 bg-slate-200 rounded w-48 animate-pulse" />
    <div className="bg-slate-200 rounded-xl h-24 animate-pulse" />
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {Array.from({ length: 8 }).map((_, i) => (
        <SkeletonHealthCard key={i} />
      ))}
    </div>
  </div>
);

// ─── Service Card ───────────────────────────────────────────

interface ServiceCardProps {
  name: string;
  detail?: HealthServiceDetail;
  statusLabel: string;
  noDataLabel: string;
  responsePrefix: string;
}

const ServiceCard: React.FC<ServiceCardProps> = ({ name, detail, statusLabel, noDataLabel, responsePrefix }) => {
  const status = detail?.status ?? 'unhealthy';
  const Icon = statusIcon(status);

  return (
    <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-5 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold text-slate-900 text-sm">{name}</h3>
        <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${status === 'healthy' ? 'bg-green-50 text-green-700' : status === 'degraded' ? 'bg-yellow-50 text-yellow-700' : 'bg-red-50 text-red-700'}`}>
          <Icon className="w-3 h-3" />
          {statusLabel}
        </span>
      </div>

      <div className="flex items-center gap-2 mb-2">
        <span className={`w-2.5 h-2.5 rounded-full ${statusBg(status)}`} />
        <span className="text-xs text-slate-500">
          {detail ? `${responsePrefix} ${formatResponseTime(detail.responseTime)}` : noDataLabel}
        </span>
      </div>

      {detail?.message && (
        <p className="text-xs text-slate-400 mt-1 truncate">{detail.message}</p>
      )}
      {detail?.version && (
        <p className="text-xs text-slate-400 mt-1">v{detail.version}</p>
      )}
    </div>
  );
};

// ─── Main Component ─────────────────────────────────────────

const Health: React.FC = () => {
  const { t } = useTranslation();
  const [intervalMs, setIntervalMs] = useState(30000);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const { data, loading, error, refetch } = useHealth({
    autoRefresh,
    intervalMs,
  });

  const handleRefresh = useCallback(() => {
    refetch();
  }, [refetch]);

  const handleIntervalChange = useCallback((value: number) => {
    setIntervalMs(value);
    setAutoRefresh(value > 0);
  }, []);

  const statusText = (status: ServiceStatus): string => {
    switch (status) {
      case 'healthy': return t('health.status_healthy');
      case 'degraded': return t('health.status_degraded');
      case 'unhealthy': return t('health.status_unhealthy');
      default: return t('health.status_unknown');
    }
  };

  // ── Loading state ───────────────────────────────────────
  if (loading && !data) {
    return <HealthPageSkeleton />;
  }

  // ── Error state ─────────────────────────────────────────
  if (error && !data) {
    return (
      <PageError
        error={error}
        onRetry={handleRefresh}
        title={t('health.load_failed')}
        description={t('health.load_failed_desc')}
      />
    );
  }

  // ── Empty / no data state ───────────────────────────────
  if (!data) {
    return (
      <EmptyState
        icon={Server}
        title={t('health.no_health_data')}
        description={t('health.no_health_data_desc')}
        action={
          <Button variant="outline" onClick={handleRefresh} leftIcon={<RotateCcw className="w-4 h-4" />}>
            {t('health.refresh')}
          </Button>
        }
      />
    );
  }

  // ── Build service list ──────────────────────────────────
  const services = data.services ?? {};
  const serviceEntries = KNOWN_SERVICES.map((name) => ({
    name,
    detail: services[name],
  }));

  const knownSet = new Set(KNOWN_SERVICES);
  const extraServices = Object.entries(services)
    .filter(([name]) => !knownSet.has(name))
    .map(([name, detail]) => ({ name, detail }));

  const allEntries = [...serviceEntries, ...extraServices];

  // ── Overall status ──────────────────────────────────────
  const overallStatus = data.status;
  const OverallIcon = statusIcon(overallStatus);
  const isAllHealthy = overallStatus === 'healthy';
  const uptimeStr = formatUptime(data.uptime, t);

  return (
    <div className="space-y-6 pb-12">
      {/* Header */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t('health.title')}</h1>
          <p className="text-slate-500 mt-1">{t('health.subtitle')}</p>
        </div>
        <div className="flex items-center gap-3">
          {/* Interval selector */}
          <div className="flex items-center gap-2">
            <Clock className="w-4 h-4 text-slate-400" />
            <select
              className="text-sm border border-slate-300 rounded-lg px-3 py-2 bg-white text-slate-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={intervalMs}
              onChange={(e) => handleIntervalChange(Number(e.target.value))}
            >
              <option value={10000}>{t('health.interval_10s')}</option>
              <option value={30000}>{t('health.interval_30s')}</option>
              <option value={60000}>{t('health.interval_60s')}</option>
              <option value={300000}>{t('health.interval_5m')}</option>
              <option value={0}>{t('health.interval_off')}</option>
            </select>
          </div>

          <Button
            variant="outline"
            onClick={handleRefresh}
            leftIcon={<RefreshCw className="w-4 h-4" />}
            isLoading={loading}
          >
            {t('health.refresh')}
          </Button>
        </div>
      </div>

      {/* Overall status banner */}
      <div
        className={`rounded-xl border p-5 ${
          isAllHealthy
            ? 'bg-green-50 border-green-200'
            : data.status === 'degraded'
            ? 'bg-yellow-50 border-yellow-200'
            : 'bg-red-50 border-red-200'
        }`}
      >
        <div className="flex items-center gap-4">
          <div
            className={`p-3 rounded-full ${
              isAllHealthy
                ? 'bg-green-100'
                : data.status === 'degraded'
                ? 'bg-yellow-100'
                : 'bg-red-100'
            }`}
          >
            <OverallIcon
              className={`w-6 h-6 ${
                isAllHealthy
                  ? 'text-green-600'
                  : data.status === 'degraded'
                  ? 'text-yellow-600'
                  : 'text-red-600'
              }`}
            />
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <h2 className="text-lg font-bold text-slate-900">
                {isAllHealthy ? t('health.all_healthy') : data.status === 'degraded' ? t('health.degraded') : t('health.unhealthy')}
              </h2>
              <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${
                isAllHealthy ? 'bg-green-100 text-green-700' : data.status === 'degraded' ? 'bg-yellow-100 text-yellow-700' : 'bg-red-100 text-red-700'
              }`}>
                <Heart className="w-3 h-3" />
                {statusText(overallStatus)}
              </span>
            </div>
            <p className="text-sm text-slate-600 mt-1">
              {uptimeStr && <span>{t('health.uptime_prefix')} {uptimeStr}</span>}
              {data.timestamp && (
                <span className="text-slate-400 ml-2">
                  | {t('health.last_update', { time: new Date(data.timestamp).toLocaleString() })}
                </span>
              )}
            </p>
          </div>
          <div className="hidden sm:flex items-center gap-1 text-sm text-slate-500">
            <Activity className="w-4 h-4" />
            <span>{t('health.services_count', { count: allEntries.length })}</span>
          </div>
        </div>
      </div>

      {/* Service cards grid */}
      {allEntries.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {allEntries.map(({ name, detail }) => (
            <ServiceCard
              key={name}
              name={name}
              detail={detail}
              statusLabel={statusText(detail?.status ?? 'unhealthy')}
              noDataLabel={t('health.no_data')}
              responsePrefix={t('health.response_time', { time: '' }).replace(/\s*:\s*$/, ':')}
            />
          ))}
        </div>
      ) : (
        <EmptyState
          icon={Server}
          title={t('health.no_service_data')}
          description={t('health.no_service_data_desc')}
        />
      )}
    </div>
  );
};

export default Health;
