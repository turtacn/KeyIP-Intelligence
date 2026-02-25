import React from 'react';
import Card from '../../components/ui/Card';
import { RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, Radar, ResponsiveContainer, Tooltip } from 'recharts';
import CoverageTreemap from './CoverageTreemap';
import { useTranslation } from 'react-i18next';

interface PanoramaViewProps {
  summary: any;
  loading: boolean;
  coverageData?: { [key: string]: number };
  scoresData?: { [key: string]: number };
}

const PanoramaView: React.FC<PanoramaViewProps> = ({ summary, loading, coverageData = {}, scoresData = {} }) => {
  const { t } = useTranslation();

  if (loading || !summary) {
    return (
      <Card className="h-64 animate-pulse bg-slate-100">
        <div></div>
      </Card>
    );
  }

  const radarData = [
    { subject: t('portfolio.panorama.radar_depth'), A: 85, fullMark: 100 },
    { subject: t('portfolio.panorama.radar_breadth'), A: 70, fullMark: 100 },
    { subject: t('portfolio.panorama.radar_quality'), A: 92, fullMark: 100 },
    { subject: t('portfolio.panorama.radar_freshness'), A: 88, fullMark: 100 },
    { subject: t('portfolio.panorama.citation_impact'), A: 75, fullMark: 100 },
  ];

  return (
    <div className="space-y-6">
      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        {[
          { label: t('portfolio.panorama.total_patents'), value: summary.totalPatents },
          { label: t('portfolio.panorama.granted'), value: summary.granted, color: 'text-green-600' },
          { label: t('portfolio.panorama.pending'), value: summary.pending, color: 'text-blue-600' },
          { label: t('portfolio.panorama.lapsed'), value: summary.lapsed, color: 'text-slate-400' },
          { label: t('portfolio.panorama.est_value'), value: `$${(summary.totalValue / 1000000).toFixed(1)}M`, color: 'text-indigo-600' },
          { label: t('portfolio.panorama.health_grade'), value: summary.healthGrade, color: 'text-purple-600' },
        ].map((stat, i) => (
          <div key={i} className="bg-white p-4 rounded-lg border border-slate-200 shadow-sm text-center">
            <div className="text-xs font-medium text-slate-500 uppercase tracking-wider mb-1">{stat.label}</div>
            <div className={`text-2xl font-bold ${stat.color || 'text-slate-900'}`}>{stat.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card header={t('portfolio.panorama.domain_analysis')} className="h-[400px]">
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
           <CoverageTreemap data={coverageData} scores={scoresData} />
        </div>
      </div>
    </div>
  );
};

export default PanoramaView;
