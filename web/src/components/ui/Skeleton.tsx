import React from 'react';

interface SkeletonProps {
  className?: string;
  style?: React.CSSProperties;
}

/** Base skeleton block with animate-pulse */
const Skeleton: React.FC<SkeletonProps> = ({ className = '', style }) => {
  return (
    <div
      className={`animate-pulse bg-slate-200 rounded ${className}`}
      style={style}
      aria-hidden="true"
    />
  );
};

/** Single line of text skeleton */
export const SkeletonLine: React.FC<{ width?: string; className?: string }> = ({
  width = '100%',
  className = '',
}) => {
  return (
    <Skeleton
      className={`h-4 ${className}`}
      style={{ width }}
    />
  );
};

/** Circle/avatar skeleton */
export const SkeletonCircle: React.FC<{ size?: number; className?: string }> = ({
  size = 40,
  className = '',
}) => {
  return (
    <Skeleton
      className={`rounded-full ${className}`}
      style={{ width: size, height: size }}
    />
  );
};

/** Card skeleton with optional header and body rows */
export const SkeletonCard: React.FC<{
  rows?: number;
  header?: boolean;
  className?: string;
}> = ({ rows = 3, header = true, className = '' }) => {
  return (
    <div
      className={`bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden ${className}`}
      aria-hidden="true"
    >
      {header && (
        <div className="px-6 py-4 border-b border-slate-100">
          <SkeletonLine width="40%" className="h-5" />
        </div>
      )}
      <div className="p-6 space-y-4">
        {Array.from({ length: rows }).map((_, i) => (
          <SkeletonLine
            key={i}
            width={`${70 + Math.random() * 30}%`}
          />
        ))}
      </div>
    </div>
  );
};

/** KPI card skeleton - mimics the dashboard metric cards */
export const SkeletonKPICard: React.FC = () => {
  return (
    <div
      className="bg-white rounded-lg border border-slate-200 shadow-sm p-6 animate-pulse"
      aria-hidden="true"
    >
      <div className="flex justify-between items-start">
        <div className="space-y-2 flex-1">
          <SkeletonLine width="60%" className="h-4" />
          <Skeleton className="h-8 w-20 rounded" />
        </div>
        <Skeleton className="h-10 w-10 rounded-lg" />
      </div>
      <div className="mt-4 flex items-center gap-2">
        <Skeleton className="h-4 w-12 rounded" />
        <SkeletonLine width="30%" className="h-3" />
      </div>
    </div>
  );
};

/** Dashboard skeleton layout - matches the ExecutiveDashboard structure */
export const SkeletonDashboard: React.FC = () => {
  return (
    <div className="space-y-8 pb-12 animate-pulse" aria-hidden="true">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div className="space-y-2">
          <Skeleton className="h-8 w-48 rounded" />
          <Skeleton className="h-4 w-64 rounded" />
        </div>
        <div className="flex gap-3">
          <Skeleton className="h-10 w-28 rounded-lg" />
          <Skeleton className="h-10 w-28 rounded-lg" />
        </div>
      </div>

      {/* NL Query Widget skeleton */}
      <Skeleton className="h-16 w-full rounded-lg" />

      {/* KPI Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {[1, 2, 3, 4].map((i) => (
          <SkeletonKPICard key={i} />
        ))}
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <SkeletonCard rows={4} className="h-80" />
        <SkeletonCard rows={4} className="h-80" />
      </div>

      {/* Tables row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <SkeletonCard rows={5} className="h-96" />
        <SkeletonCard rows={5} className="h-96" />
      </div>

      {/* Bottom charts row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <SkeletonCard rows={4} className="h-80" />
        <div />
      </div>
    </div>
  );
};

/** Table skeleton - mimics DataTable structure */
export const SkeletonTable: React.FC<{
  columns?: number;
  rows?: number;
  className?: string;
}> = ({ columns = 5, rows = 5, className = '' }) => {
  return (
    <div
      className={`bg-white border border-slate-200 rounded-lg shadow-sm overflow-hidden ${className}`}
      aria-hidden="true"
    >
      {/* Header */}
      <div className="bg-slate-50 px-6 py-3 flex gap-6 animate-pulse">
        {Array.from({ length: columns }).map((_, i) => (
          <Skeleton
            key={i}
            className="h-4 flex-1 rounded"
          />
        ))}
      </div>
      {/* Body rows */}
      <div className="divide-y divide-slate-200">
        {Array.from({ length: rows }).map((_, rowIdx) => (
          <div key={rowIdx} className="px-6 py-4 flex gap-6 animate-pulse">
            {Array.from({ length: columns }).map((_, colIdx) => (
              <Skeleton
                key={colIdx}
                className="h-4 flex-1 rounded"
                style={{ width: `${60 + Math.random() * 40}%` }}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
};

export default Skeleton;
