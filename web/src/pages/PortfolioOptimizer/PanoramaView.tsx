import React from 'react';
import Card from '../../components/ui/Card';
import StatusBadge from '../../components/ui/StatusBadge';
import { RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, Radar, ResponsiveContainer, Tooltip } from 'recharts';

interface PanoramaViewProps {
  summary: any;
  loading: boolean;
}

const PanoramaView: React.FC<PanoramaViewProps> = ({ summary, loading }) => {
  if (loading || !summary) {
    return (
      <Card className="h-64 animate-pulse bg-slate-100" />
    );
  }

  const radarData = [
    { subject: 'Depth', A: 85, fullMark: 100 },
    { subject: 'Breadth', A: 70, fullMark: 100 },
    { subject: 'Quality', A: 92, fullMark: 100 },
    { subject: 'Freshness', A: 88, fullMark: 100 },
    { subject: 'Citation Impact', A: 75, fullMark: 100 },
  ];

  return (
    <div className="space-y-6">
      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        {[
          { label: 'Total Patents', value: summary.totalPatents },
          { label: 'Granted', value: summary.granted, color: 'text-green-600' },
          { label: 'Pending', value: summary.pending, color: 'text-blue-600' },
          { label: 'Lapsed', value: summary.lapsed, color: 'text-slate-400' },
          { label: 'Est. Value', value: `$${(summary.totalValue / 1000000).toFixed(1)}M`, color: 'text-indigo-600' },
          { label: 'Health Grade', value: summary.healthGrade, color: 'text-purple-600' },
        ].map((stat, i) => (
          <div key={i} className="bg-white p-4 rounded-lg border border-slate-200 shadow-sm text-center">
            <div className="text-xs font-medium text-slate-500 uppercase tracking-wider mb-1">{stat.label}</div>
            <div className={`text-2xl font-bold ${stat.color || 'text-slate-900'}`}>{stat.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card header="Domain Coverage Analysis" className="h-[400px]">
           <ResponsiveContainer width="100%" height="100%">
             <RadarChart cx="50%" cy="50%" outerRadius="80%" data={radarData}>
               <PolarGrid />
               <PolarAngleAxis dataKey="subject" />
               <PolarRadiusAxis angle={30} domain={[0, 100]} />
               <Radar name="Portfolio" dataKey="A" stroke="#8884d8" fill="#8884d8" fillOpacity={0.6} />
               <Tooltip />
             </RadarChart>
           </ResponsiveContainer>
        </Card>

        {/* Treemap Container */}
        <div id="coverage-treemap-container" className="h-[400px]">
           {/* This will be filled by CoverageTreemap component in index.tsx composition */}
           <div className="bg-slate-50 h-full rounded-lg border border-slate-200 flex items-center justify-center text-slate-400">
             Treemap Component Location
           </div>
        </div>
      </div>
    </div>
  );
};

export default PanoramaView;
