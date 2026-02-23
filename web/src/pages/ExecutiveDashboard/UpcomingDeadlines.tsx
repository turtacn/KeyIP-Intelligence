import React from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import StatusBadge from '../../components/ui/StatusBadge';
import { LifecycleEvent } from '../../types/domain';

interface UpcomingDeadlinesProps {
  events: LifecycleEvent[];
  loading: boolean;
}

const UpcomingDeadlines: React.FC<UpcomingDeadlinesProps> = ({ events, loading }) => {
  const columns: Column<LifecycleEvent>[] = [
    { header: 'Patent ID', accessor: 'patentId' },
    { header: 'Event', accessor: 'eventType' },
    { header: 'Jurisdiction', accessor: 'jurisdiction' },
    {
      header: 'Due Date',
      accessor: (row) => {
        const due = new Date(row.dueDate);
        const today = new Date();
        const diffTime = due.getTime() - today.getTime();
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

        let colorClass = 'text-slate-700';
        if (diffDays < 0) colorClass = 'text-red-600 font-bold';
        else if (diffDays <= 7) colorClass = 'text-red-500 font-semibold';
        else if (diffDays <= 30) colorClass = 'text-amber-500';

        return <span className={colorClass}>{row.dueDate} ({diffDays}d)</span>;
      }
    },
    {
      header: 'Status',
      accessor: (row) => (
        <StatusBadge
          status={row.status === 'overdue' ? 'error' : row.status === 'completed' ? 'completed' : 'pending'}
          label={row.status}
          className="text-xs"
        />
      )
    },
  ];

  return (
    <Card header="Upcoming Deadlines" padding="none" className="h-96">
      <DataTable
        columns={columns}
        data={events}
        isLoading={loading}
      />
    </Card>
  );
};

export default UpcomingDeadlines;
