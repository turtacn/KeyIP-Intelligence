import React from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import StatusBadge from '../../components/ui/StatusBadge';
import { LifecycleEvent } from '../../types/domain';
import { DollarSign, PieChart, TrendingUp, CheckCircle } from 'lucide-react';
import Button from '../../components/ui/Button';
import { useTranslation } from 'react-i18next';

interface AnnuityManagerProps {
  events: LifecycleEvent[];
  loading: boolean;
  onPay: (ids: string[]) => void;
}

const AnnuityManager: React.FC<AnnuityManagerProps> = ({ events, loading, onPay }) => {
  const { t } = useTranslation();
  const annuityEvents = events.filter(e => e.eventType === 'annuity_due');
  const totalDue = annuityEvents.reduce((acc, curr) => acc + (curr.feeAmount || 0), 0);
  const pendingCount = annuityEvents.filter(e => e.status !== 'completed').length;

  const columns: Column<LifecycleEvent>[] = [
    { header: t('lifecycle.table.patent_id'), accessor: 'patentId' },
    { header: t('lifecycle.table.jurisdiction'), accessor: 'jurisdiction' },
    { header: t('lifecycle.annuity.year'), accessor: () => new Date().getFullYear() }, // Mock year
    {
      header: t('lifecycle.annuity.fee_amount'),
      accessor: (row) => (
        <span className="font-mono">
          {row.currency} {row.feeAmount?.toLocaleString()}
        </span>
      )
    },
    {
      header: t('lifecycle.annuity.usd_equiv'),
      accessor: (row) => {
        // Mock conversion rates
        const rates: Record<string, number> = { 'USD': 1, 'CNY': 0.14, 'EUR': 1.08, 'JPY': 0.0067, 'KRW': 0.00075 };
        const usd = (row.feeAmount || 0) * (rates[row.currency || 'USD'] || 1);
        return <span className="text-slate-500 text-xs">≈ ${usd.toFixed(2)}</span>;
      }
    },
    { header: t('lifecycle.table.due_date'), accessor: 'dueDate' },
    { header: t('lifecycle.table.status'), accessor: (row) => <StatusBadge status={row.status === 'completed' ? 'completed' : 'pending'} label={row.status} /> },
    {
      header: t('partners.admin.table.actions'),
      accessor: (row) => (
        row.status !== 'completed' ? (
          <Button size="sm" variant="primary" onClick={() => onPay([row.id])}>{t('lifecycle.annuity.pay_btn')}</Button>
        ) : <span className="text-green-600 text-xs font-medium flex items-center"><CheckCircle className="w-3 h-3 mr-1" /> Paid</span>
      )
    }
  ];

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card padding="md" className="bg-blue-50 border-blue-100">
          <div className="flex items-center">
            <div className="p-3 bg-blue-100 rounded-full mr-4">
              <DollarSign className="w-6 h-6 text-blue-600" />
            </div>
            <div>
              <p className="text-sm text-blue-600 font-medium">{t('lifecycle.annuity.total_due')}</p>
              <h3 className="text-2xl font-bold text-blue-900">${(totalDue * 0.14).toLocaleString()}</h3> {/* Mock total conversion */}
            </div>
          </div>
        </Card>

        <Card padding="md" className="bg-green-50 border-green-100">
          <div className="flex items-center">
            <div className="p-3 bg-green-100 rounded-full mr-4">
              <CheckCircle className="w-6 h-6 text-green-600" />
            </div>
            <div>
              <p className="text-sm text-green-600 font-medium">{t('lifecycle.annuity.completed')}</p>
              <h3 className="text-2xl font-bold text-green-900">{annuityEvents.length - pendingCount}</h3>
            </div>
          </div>
        </Card>

        <Card padding="md" className="bg-amber-50 border-amber-100">
          <div className="flex items-center">
            <div className="p-3 bg-amber-100 rounded-full mr-4">
              <PieChart className="w-6 h-6 text-amber-600" />
            </div>
            <div>
              <p className="text-sm text-amber-600 font-medium">{t('lifecycle.annuity.pending')}</p>
              <h3 className="text-2xl font-bold text-amber-900">{pendingCount}</h3>
            </div>
          </div>
        </Card>

        <Card padding="md" className="bg-purple-50 border-purple-100">
          <div className="flex items-center">
            <div className="p-3 bg-purple-100 rounded-full mr-4">
              <TrendingUp className="w-6 h-6 text-purple-600" />
            </div>
            <div>
              <p className="text-sm text-purple-600 font-medium">{t('lifecycle.annuity.projected')}</p>
              <h3 className="text-2xl font-bold text-purple-900">+12%</h3>
            </div>
          </div>
        </Card>
      </div>

      <Card padding="none">
        <div className="px-6 py-4 border-b border-slate-200 bg-slate-50 rounded-t-lg">
          <h3 className="font-semibold text-slate-800">{t('lifecycle.annuity.table_title')}</h3>
        </div>
        <DataTable columns={columns} data={annuityEvents} isLoading={loading} />
      </Card>
    </div>
  );
};

export default AnnuityManager;
