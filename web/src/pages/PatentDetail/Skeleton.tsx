import React from 'react';
import { SkeletonLine, SkeletonCard } from '../../components/ui/Skeleton';

/** Specialized skeleton matching the PatentDetail page layout */
const PatentDetailSkeleton: React.FC = () => {
  return (
    <div className="space-y-6 pb-12" aria-hidden="true">
      {/* Back button skeleton */}
      <div className="animate-pulse h-4 w-16 bg-slate-200 rounded" />

      {/* Header skeleton: title + publication number */}
      <div className="animate-pulse space-y-3">
        <div className="h-8 w-3/4 bg-slate-200 rounded" />
        <SkeletonLine width="40%" />
      </div>

      {/* Summary card skeleton: 4-column grid */}
      <div className="bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden animate-pulse" aria-hidden="true">
        <div className="p-6 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="space-y-2">
              <div className="h-3 w-20 bg-slate-200 rounded" />
              <div className="h-5 w-32 bg-slate-200 rounded" />
            </div>
          ))}
        </div>
      </div>

      {/* Abstract card skeleton */}
      <SkeletonCard rows={3} className="h-36" />

      {/* Two-column grid: Inventors + IPC codes */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <SkeletonCard rows={3} className="h-32" />
        <SkeletonCard rows={3} className="h-32" />
      </div>

      {/* Claims card skeleton */}
      <SkeletonCard header rows={4} className="h-64" />

      {/* Citations card skeleton */}
      <SkeletonCard header rows={2} className="h-28" />

      {/* Family Tree skeleton */}
      <SkeletonCard header rows={4} className="h-72" />
    </div>
  );
};

export default PatentDetailSkeleton;
