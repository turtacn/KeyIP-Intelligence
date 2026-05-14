import React from 'react';
import { SkeletonCard, SkeletonKPICard } from '../../components/ui/Skeleton';

/** Specialized skeleton matching the PortfolioOptimizer page layout */
const PortfolioOptimizerSkeleton: React.FC = () => {
  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]" aria-hidden="true">
      {/* Sticky sub-nav skeleton: 6 nav items */}
      <div className="bg-white border-b border-slate-200 sticky top-16 z-10 mb-6">
        <div className="flex space-x-8 px-1 animate-pulse">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <div key={i} className="h-4 w-20 bg-slate-200 rounded py-4" />
          ))}
        </div>
      </div>

      {/* Scrollable content area */}
      <div className="flex-1 overflow-y-auto pb-12 space-y-12">
        {/* Section 1: Panorama */}
        <div className="space-y-6">
          <div className="animate-pulse space-y-2">
            <div className="h-6 w-48 bg-slate-200 rounded" />
            <div className="h-4 w-72 bg-slate-200 rounded" />
          </div>
          {/* KPI row */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {[1, 2, 3, 4].map((i) => (
              <SkeletonKPICard key={i} />
            ))}
          </div>
          {/* Chart area */}
          <SkeletonCard rows={4} className="h-72" />
        </div>

        {/* Section 2: Constellation Map */}
        <div className="space-y-6">
          <div className="animate-pulse space-y-2">
            <div className="h-6 w-56 bg-slate-200 rounded" />
            <div className="h-4 w-80 bg-slate-200 rounded" />
          </div>
          <SkeletonCard rows={3} className="h-80" />
        </div>

        {/* Section 3: Gap Analysis */}
        <div className="space-y-6">
          <div className="animate-pulse space-y-2">
            <div className="h-6 w-48 bg-slate-200 rounded" />
            <div className="h-4 w-72 bg-slate-200 rounded" />
          </div>
          <SkeletonCard rows={3} className="h-[500px]" />
        </div>

        {/* Section 4: Value Scoring */}
        <div className="space-y-6">
          <div className="animate-pulse space-y-2">
            <div className="h-6 w-48 bg-slate-200 rounded" />
            <div className="h-4 w-72 bg-slate-200 rounded" />
          </div>
          <SkeletonCard rows={5} className="h-96" />
        </div>

        {/* Section 5: Budget */}
        <div className="space-y-6">
          <div className="animate-pulse space-y-2">
            <div className="h-6 w-48 bg-slate-200 rounded" />
            <div className="h-4 w-72 bg-slate-200 rounded" />
          </div>
          <SkeletonCard rows={3} className="h-[500px]" />
        </div>

        {/* Section 6: Simulator */}
        <div className="space-y-6">
          <div className="animate-pulse space-y-2">
            <div className="h-6 w-48 bg-slate-200 rounded" />
            <div className="h-4 w-72 bg-slate-200 rounded" />
          </div>
          <SkeletonCard rows={3} className="h-[500px]" />
        </div>
      </div>
    </div>
  );
};

export default PortfolioOptimizerSkeleton;
