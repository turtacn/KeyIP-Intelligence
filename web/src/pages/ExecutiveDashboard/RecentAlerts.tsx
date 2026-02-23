import React from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import RiskLevelBadge from '../../components/ui/RiskLevelBadge';
import Button from '../../components/ui/Button';
import { InfringementAlert } from '../../types/domain';
import { Eye } from 'lucide-react';

interface RecentAlertsProps {
  alerts: InfringementAlert[];
  loading: boolean;
}

const RecentAlerts: React.FC<RecentAlertsProps> = ({ alerts, loading }) => {
  const columns: Column<InfringementAlert>[] = [
    { header: 'Alert ID', accessor: 'id' },
    { header: 'Risk Level', accessor: (row) => <RiskLevelBadge level={row.riskLevel} /> },
    { header: 'Target Patent', accessor: 'targetPatentId' },
    { header: 'Detected', accessor: (row) => new Date(row.detectedAt).toLocaleDateString() },
    {
      header: 'Action',
      accessor: () => (
        <Button size="sm" variant="ghost" leftIcon={<Eye className="w-4 h-4" />}>
          View
        </Button>
      )
    },
  ];

  return (
    <Card header="Recent Infringement Alerts" padding="none" className="h-96">
      <DataTable
        columns={columns}
        data={alerts}
        isLoading={loading}
      />
    </Card>
  );
};

export default RecentAlerts;
