import React, { useState } from 'react';
import { useDashboard } from '../../hooks/useDashboard';
import KPICards from './KPICards';
import TrendChart from './TrendChart';
import JurisdictionPie from './JurisdictionPie';
import CompetitorRadar from './CompetitorRadar';
import UpcomingDeadlines from './UpcomingDeadlines';
import RecentAlerts from './RecentAlerts';
import NLQueryWidget from './NLQueryWidget';
import Button from '../../components/ui/Button';
import { Download } from 'lucide-react';
import LoadingSpinner from '../../components/ui/LoadingSpinner';

const ExecutiveDashboard: React.FC = () => {
  const { data, loading, error } = useDashboard();
  const [reportGenerating, setReportGenerating] = useState(false);

  const handleGenerateReport = () => {
    setReportGenerating(true);
    // Simulate generation
    setTimeout(() => {
      setReportGenerating(false);
      alert('Report generated successfully!');
    }, 2000);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-6 text-center text-red-600 bg-red-50 rounded-lg">
        Error loading dashboard data: {error}
      </div>
    );
  }

  // Transform data for charts
  const radarData = [
    { subject: 'Portfolio Size', A: 85, B: 90, fullMark: 100 },
    { subject: 'Filing Rate', A: 78, B: 65, fullMark: 100 },
    { subject: 'Grant Rate', A: 92, B: 88, fullMark: 100 },
    { subject: 'Tech Coverage', A: 65, B: 85, fullMark: 100 },
    { subject: 'Citation Index', A: 72, B: 80, fullMark: 100 },
  ];

  return (
    <div className="space-y-8 pb-12">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Executive Dashboard</h1>
          <p className="text-slate-500 mt-1">
            Real-time overview of your patent portfolio and strategic KPIs.
          </p>
        </div>
        <div className="flex gap-3">
          <Button
            variant="outline"
            leftIcon={<Download className="w-4 h-4" />}
            onClick={handleGenerateReport}
            isLoading={reportGenerating}
          >
            Export Report
          </Button>
          <Button variant="primary">Add New Patent</Button>
        </div>
      </div>

      <NLQueryWidget />

      <KPICards metrics={data} loading={loading} />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <TrendChart data={data.monthlyApplicationTrend} loading={loading} />
        <JurisdictionPie data={data.jurisdictionBreakdown} loading={loading} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <UpcomingDeadlines events={data.upcomingDeadlines} loading={loading} />
        <RecentAlerts alerts={data.recentAlerts} loading={loading} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <CompetitorRadar data={radarData} loading={loading} />
        {/* Placeholder for future widget */}
      </div>
    </div>
  );
};

export default ExecutiveDashboard;
