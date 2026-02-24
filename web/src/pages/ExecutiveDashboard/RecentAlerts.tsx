import React from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import RiskLevelBadge from '../../components/ui/RiskLevelBadge';
import Button from '../../components/ui/Button';
import { InfringementAlert } from '../../types/domain';
import { Eye } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface RecentAlertsProps {
  alerts: InfringementAlert[];
  loading: boolean;
}

const RecentAlerts: React.FC<RecentAlertsProps> = ({ alerts, loading }) => {
  const { t } = useTranslation();

  const columns: Column<InfringementAlert>[] = [
    { header: t('dashboard.alerts.id'), accessor: 'id' },
    { header: t('dashboard.alerts.risk'), accessor: (row) => <RiskLevelBadge level={row.riskLevel} /> },
    { header: t('dashboard.alerts.target'), accessor: 'targetPatentId' },
    { header: t('dashboard.alerts.detected'), accessor: (row) => new Date(row.detectedAt).toLocaleDateString() },
    {
      header: t('dashboard.alerts.action'),
      accessor: () => (
        <Button size="sm" variant="ghost" leftIcon={<Eye className="w-4 h-4" />}>
          {t('dashboard.alerts.view')}
        </Button>
      )
    },
  ];

  return (
    <Card header={t('dashboard.alerts.title')} padding="none" className="h-96">
      <DataTable
        columns={columns}
        data={alerts}
        isLoading={loading}
      />
    </Card>
  );
};

export default RecentAlerts;
