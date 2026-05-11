import React from 'react';
import { Skeleton, SkeletonLine, SkeletonCard } from '../../components/ui/Skeleton';

/** Specialized skeleton matching the MoleculeDetail page layout */
const MoleculeDetailSkeleton: React.FC = () => {
  return (
    <div className="space-y-6 pb-12" aria-hidden="true">
      {/* Back button skeleton */}
      <div className="animate-pulse h-4 w-16 bg-slate-200 rounded" />

      {/* Header skeleton: molecule name + ID */}
      <div className="animate-pulse space-y-3">
        <div className="h-8 w-1/2 bg-slate-200 rounded" />
        <SkeletonLine width="30%" />
      </div>

      {/* Two-column layout: Structure (left 1/3) + Properties (right 2/3) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Structure viewer skeleton */}
        <div className="lg:col-span-1 bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden animate-pulse">
          <div className="px-6 py-4 border-b border-slate-100">
            <SkeletonLine width="60%" className="h-5" />
          </div>
          <div className="p-6 flex flex-col items-center gap-4">
            <div className="w-[280px] h-[200px] bg-slate-100 rounded" />
            <SkeletonLine width="80%" className="h-3" />
          </div>
        </div>

        {/* Properties skeleton */}
        <div className="lg:col-span-2 bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden animate-pulse">
          <div className="px-6 py-4 border-b border-slate-100">
            <SkeletonLine width="50%" className="h-5" />
          </div>
          <div className="p-6 space-y-5">
            {[1, 2, 3].map((i) => (
              <div key={i} className="flex justify-between items-center py-2 border-b border-slate-100">
                <Skeleton className="h-4 w-28 rounded" />
                <Skeleton className="h-4 w-20 rounded" />
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Material Properties table skeleton */}
      <SkeletonCard header rows={4} className="h-56" />

      {/* Property Comparison skeleton */}
      <SkeletonCard header rows={3} className="h-48" />
    </div>
  );
};

export default MoleculeDetailSkeleton;
