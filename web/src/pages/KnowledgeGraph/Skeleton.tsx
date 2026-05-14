import React from 'react';
import { SkeletonLine } from '../../components/ui/Skeleton';
import { Search, ZoomIn, ZoomOut, Maximize } from 'lucide-react';

/** Specialized skeleton matching the KnowledgeGraph page layout */
const KnowledgeGraphSkeleton: React.FC = () => {
  return (
    <div className="h-[calc(100vh-8rem)] flex flex-col" aria-hidden="true">
      {/* Header with title + controls */}
      <div className="mb-4 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
        <div className="animate-pulse space-y-2">
          <div className="h-8 w-56 bg-slate-200 rounded" />
        </div>
        <div className="flex flex-wrap items-center gap-2 animate-pulse">
          {/* Search input */}
          <div className="relative">
            <div className="h-[38px] w-40 bg-slate-200 rounded-lg" />
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-300" />
          </div>
          {/* Patent ID input + button */}
          <div className="flex items-center gap-1">
            <div className="h-[38px] w-40 bg-slate-200 rounded-lg" />
            <div className="h-[38px] w-16 bg-slate-200 rounded-lg" />
          </div>
        </div>
      </div>

      {/* Graph card */}
      <div className="flex-1 relative overflow-hidden rounded-lg border border-slate-200 shadow-sm bg-white">
        {/* Cytoscape canvas area */}
        <div className="absolute inset-0 bg-slate-50">
          {/* Simulated graph nodes */}
          <div className="w-full h-full animate-pulse">
            {[1, 2, 3, 4, 5, 6, 7, 8].map((i) => {
              const top = 10 + ((i * 17 + 3) % 80);
              const left = 5 + ((i * 23 + 7) % 85);
              const size = i % 3 === 0 ? 50 : 40;
              return (
                <div
                  key={i}
                  className="absolute bg-slate-200 rounded-full"
                  style={{
                    top: `${top}%`,
                    left: `${left}%`,
                    width: size,
                    height: size,
                  }}
                />
              );
            })}
            {/* Simulated graph edges */}
            <svg className="absolute inset-0 w-full h-full pointer-events-none">
              {[1, 2, 3, 4].map((i) => (
                <line
                  key={i}
                  x1={`${10 + i * 20}%`}
                  y1={`${20 + i * 15}%`}
                  x2={`${25 + i * 18}%`}
                  y2={`${35 + i * 12}%`}
                  className="stroke-slate-200"
                  strokeWidth="2"
                />
              ))}
            </svg>
          </div>
        </div>

        {/* Zoom controls skeleton */}
        <div className="absolute bottom-4 right-4 flex flex-col gap-2 bg-white p-2 rounded-lg shadow-md border border-slate-200">
          <div className="p-2 text-slate-200"><ZoomIn className="w-5 h-5" /></div>
          <div className="p-2 text-slate-200"><ZoomOut className="w-5 h-5" /></div>
          <div className="p-2 text-slate-200"><Maximize className="w-5 h-5" /></div>
        </div>

        {/* Legend skeleton */}
        <div className="absolute top-4 left-4 bg-white/90 p-3 rounded-lg shadow-sm border border-slate-200 animate-pulse">
          <div className="h-3 w-12 bg-slate-200 rounded mb-3" />
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-slate-200" />
              <SkeletonLine width="40px" className="h-3" />
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 bg-slate-200" style={{ clipPath: 'polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)' }} />
              <SkeletonLine width="40px" className="h-3" />
            </div>
          </div>
        </div>

        {/* Mode badge skeleton */}
        <div className="absolute top-4 right-4 animate-pulse">
          <div className="h-5 w-20 bg-slate-200 rounded-full" />
        </div>
      </div>
    </div>
  );
};

export default KnowledgeGraphSkeleton;
