import React from 'react';
import Card from '../../components/ui/Card';
import { ArrowUp, ArrowDown } from 'lucide-react';
import { DashboardMetrics } from '../../types/domain';
import { useTranslation } from 'react-i18next';

interface KPICardsProps {
  metrics: DashboardMetrics;
  loading: boolean;
}

const KPICards: React.FC<KPICardsProps> = ({ metrics, loading }) => {
  const { t } = useTranslation();

  if (loading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {[1, 2, 3, 4].map((i) => (
          <Card key={i} className="h-32 animate-pulse bg-slate-100">
            <div className="h-full"></div>
          </Card>
        ))}
      </div>
    );
  }

  const kpis = [
    {
      label: t('dashboard.kpi.total_patents'),
      value: metrics.totalPatents,
      trend: '+12%',
      trendUp: true,
      color: 'bg-blue-50 text-blue-700',
    },
    {
      label: t('dashboard.kpi.active_patents'),
      value: metrics.activePatents,
      trend: '+5%',
      trendUp: true,
      color: 'bg-green-50 text-green-700',
    },
    {
      label: t('dashboard.kpi.high_risk'),
      value: metrics.highRiskAlerts,
      trend: '-2',
      trendUp: false,
      trendGood: true,
      color: 'bg-red-50 text-red-700',
    },
    {
      label: t('dashboard.kpi.due_month'),
      value: metrics.dueThisMonth,
      trend: '+3',
      trendUp: true,
      trendGood: false,
      color: 'bg-amber-50 text-amber-700',
    },
  ];

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      {kpis.map((kpi, index) => (
        <Card key={index} padding="lg">
          <div className="flex justify-between items-start">
            <div>
              <p className="text-sm font-medium text-slate-500 mb-1">{kpi.label}</p>
              <h3 className="text-3xl font-bold text-slate-900">{kpi.value}</h3>
            </div>
            <div className={`p-2 rounded-lg ${kpi.color}`}>
            </div>
          </div>
          <div className="mt-4 flex items-center text-sm">
            <span
              className={`flex items-center font-medium ${
                (kpi.trendGood !== false && kpi.trendUp) || (kpi.trendGood === true && !kpi.trendUp)
                  ? 'text-green-600'
                  : 'text-red-600'
              }`}
            >
              {kpi.trendUp ? <ArrowUp className="w-4 h-4 mr-1" /> : <ArrowDown className="w-4 h-4 mr-1" />}
              {kpi.trend}
            </span>
            <span className="text-slate-400 ml-2">{t('dashboard.kpi.vs_last_month')}</span>
          </div>
        </Card>
      ))}
    </div>
  );
};

export default KPICards;
