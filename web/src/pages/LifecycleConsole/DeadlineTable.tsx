import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import StatusBadge from '../../components/ui/StatusBadge';
import { LifecycleEvent } from '../../types/domain';
import { Calendar, Download, CheckSquare, Check } from 'lucide-react';
import Button from '../../components/ui/Button';

interface DeadlineTableProps {
  events: LifecycleEvent[];
  loading: boolean;
  onMarkHandled: (ids: string[]) => void;
  onExport: () => void;
}

const DeadlineTable: React.FC<DeadlineTableProps> = ({ events, loading, onMarkHandled, onExport }) => {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const handleSelect = (id: string) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedIds(newSelected);
  };

  const handleSelectAll = () => {
    if (selectedIds.size === events.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(events.map((e) => e.id)));
    }
  };

  const columns: Column<LifecycleEvent>[] = [
    {
      header: 'Select',
      accessor: (row) => (
        <input
          type="checkbox"
          checked={selectedIds.has(row.id)}
          onChange={() => handleSelect(row.id)}
          className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-slate-300 rounded cursor-pointer"
        />
      ),
      className: 'w-10 text-center',
    },
    { header: 'Patent ID', accessor: 'patentId' },
    { header: 'Event Type', accessor: (row) => row.eventType.replace(/_/g, ' ') },
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
        else if (diffDays <= 7) colorClass = 'text-red-600 font-semibold';
        else if (diffDays <= 30) colorClass = 'text-amber-600 font-medium';

        return (
          <span className={`${colorClass} inline-flex items-center gap-1`}>
            {row.dueDate}
            <span className="text-xs opacity-75">({diffDays}d)</span>
          </span>
        );
      },
    },
    {
      header: 'Fee',
      accessor: (row) =>
        row.feeAmount ? `${row.currency} ${row.feeAmount.toLocaleString()}` : '-',
    },
    {
      header: 'Status',
      accessor: (row) => (
        <StatusBadge
          status={
            row.status === 'overdue'
              ? 'error'
              : row.status === 'completed'
              ? 'completed'
              : 'pending'
          }
          label={row.status}
          className="capitalize"
        />
      ),
    },
  ];

  return (
    <Card padding="none" className="min-h-[500px] flex flex-col">
      <div className="px-6 py-4 border-b border-slate-200 flex justify-between items-center bg-slate-50 rounded-t-lg">
        <div className="flex items-center space-x-3">
          <div className="bg-white p-2 rounded-lg border border-slate-200 shadow-sm">
            <Calendar className="w-5 h-5 text-blue-600" />
          </div>
          <div>
            <h3 className="font-semibold text-slate-800">Deadline Calendar</h3>
            <p className="text-xs text-slate-500">{events.length} upcoming events</p>
          </div>
        </div>
        <div className="flex space-x-2">
          <Button
            size="sm"
            variant="secondary"
            leftIcon={<Check className="w-4 h-4" />}
            onClick={handleSelectAll}
          >
            {selectedIds.size === events.length ? 'Deselect All' : 'Select All'}
          </Button>

          {selectedIds.size > 0 && (
            <Button
              size="sm"
              variant="primary"
              leftIcon={<CheckSquare className="w-4 h-4" />}
              onClick={() => {
                onMarkHandled(Array.from(selectedIds));
                setSelectedIds(new Set());
              }}
            >
              Mark {selectedIds.size} Handled
            </Button>
          )}

          <Button
            size="sm"
            variant="outline"
            leftIcon={<Download className="w-4 h-4" />}
            onClick={onExport}
          >
            Export CSV
          </Button>
        </div>
      </div>
      <div className="flex-1">
        <DataTable
          columns={columns}
          data={events}
          isLoading={loading}
          pagination={{
            currentPage: 1,
            totalPages: 1, // Mock pagination for now
            onPageChange: () => {},
          }}
        />
      </div>
    </Card>
  );
};

export default DeadlineTable;
