import React, { useState, useCallback } from 'react';
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

// ─── Constants ──────────────────────────────────────────────

const REFRESH_INTERVALS = [
  { label: '10s', value: 10000 },
  { label: '30s', value: 30000 },
  { label: '60s', value: 60000 },
  { label: '5m', value: 300000 },
  { label: '关闭', value: 0 },
];

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

function statusLabel(status: ServiceStatus): string {
  switch (status) {
    case 'healthy':
      return '正常';
    case 'degraded':
      return '降级';
    case 'unhealthy':
      return '异常';
    default:
      return '未知';
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

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const parts: string[] = [];
  if (days > 0) parts.push(`${days}天`);
  if (hours > 0) parts.push(`${hours}小时`);
  parts.push(`${minutes}分钟`);
  return parts.join(' ');
}

function formatResponseTime(ms: number): string {
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
}

const ServiceCard: React.FC<ServiceCardProps> = ({ name, detail }) => {
  const status = detail?.status ?? 'unhealthy';
  const Icon = statusIcon(status);

  return (
    <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-5 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold text-slate-900 text-sm">{name}</h3>
        <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${status === 'healthy' ? 'bg-green-50 text-green-700' : status === 'degraded' ? 'bg-yellow-50 text-yellow-700' : 'bg-red-50 text-red-700'}`}>
          <Icon className="w-3 h-3" />
          {statusLabel(status)}
        </span>
      </div>

      <div className="flex items-center gap-2 mb-2">
        <span className={`w-2.5 h-2.5 rounded-full ${statusBg(status)}`} />
        <span className="text-xs text-slate-500">
          {detail ? `响应: ${formatResponseTime(detail.responseTime)}` : '无数据'}
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
        title="加载系统健康状态失败"
        description="无法获取服务健康信息，请检查后端服务是否正常运行。"
      />
    );
  }

  // ── Empty / no data state ───────────────────────────────
  if (!data) {
    return (
      <EmptyState
        icon={Server}
        title="暂无健康数据"
        description="当前没有可用的健康检查数据。"
        action={
          <Button variant="outline" onClick={handleRefresh} leftIcon={<RotateCcw className="w-4 h-4" />}>
            刷新
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

  // Unknown/additional services from the backend
  const knownSet = new Set(KNOWN_SERVICES);
  const extraServices = Object.entries(services)
    .filter(([name]) => !knownSet.has(name))
    .map(([name, detail]) => ({ name, detail }));

  const allEntries = [...serviceEntries, ...extraServices];

  // ── Overall status ──────────────────────────────────────
  const overallStatus = data.status;
  const OverallIcon = statusIcon(overallStatus);
  const isAllHealthy = overallStatus === 'healthy';

  return (
    <div className="space-y-6 pb-12">
      {/* Header */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">系统健康状态</h1>
          <p className="text-slate-500 mt-1">
            监控各后端服务的运行状态和响应时间
          </p>
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
              {REFRESH_INTERVALS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          <Button
            variant="outline"
            onClick={handleRefresh}
            leftIcon={<RefreshCw className="w-4 h-4" />}
            isLoading={loading}
          >
            刷新
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
                {isAllHealthy ? '所有服务正常运行' : data.status === 'degraded' ? '部分服务降级' : '系统服务异常'}
              </h2>
              <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${
                isAllHealthy ? 'bg-green-100 text-green-700' : data.status === 'degraded' ? 'bg-yellow-100 text-yellow-700' : 'bg-red-100 text-red-700'
              }`}>
                <Heart className="w-3 h-3" />
                {statusLabel(overallStatus)}
              </span>
            </div>
            <p className="text-sm text-slate-600 mt-1">
              已运行 {formatUptime(data.uptime)}
              {data.timestamp && (
                <span className="text-slate-400 ml-2">
                  | 最后更新: {new Date(data.timestamp).toLocaleString('zh-CN')}
                </span>
              )}
            </p>
          </div>
          <div className="hidden sm:flex items-center gap-1 text-sm text-slate-500">
            <Activity className="w-4 h-4" />
            <span>{allEntries.length} 个服务</span>
          </div>
        </div>
      </div>

      {/* Service cards grid */}
      {allEntries.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {allEntries.map(({ name, detail }) => (
            <ServiceCard key={name} name={name} detail={detail} />
          ))}
        </div>
      ) : (
        <EmptyState
          icon={Server}
          title="暂无服务数据"
          description="健康检查接口未返回任何服务信息。"
        />
      )}
    </div>
  );
};

export default Health;
