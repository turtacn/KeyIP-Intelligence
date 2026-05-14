import React, { useMemo, useCallback } from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import { useTableState } from '../../hooks/useTableState';
import StatusBadge from '../../components/ui/StatusBadge';
import { LifecycleEvent } from '../../types/domain';
import { Calendar, CheckSquare, Check } from 'lucide-react';
import Button from '../../components/ui/Button';
import { useTranslation } from 'react-i18next';

interface DeadlineTableProps {
  events: LifecycleEvent[];
  loading: boolean;
  onMarkHandled: (ids: string[]) => void;
  onExport: () => void;
}

const DeadlineTable: React.FC<DeadlineTableProps> = ({ events, loading, onMarkHandled, onExport }) => {
  const { t } = useTranslation();

  const tableState = useTableState({
    sort: { column: 'dueDate', direction: 'asc' },
  });

  // Derive selected IDs from events using the tableState's currentPage-based selection
  // In this integration, we manage selection independently since it's shared with parent
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set());

  // Sort the events
  const sortedEvents = useMemo(() => {
    if (!tableState.sort.column || !tableState.sort.direction) return events;

    return [...events].sort((a, b) => {
      let comparison = 0;
      const col = tableState.sort.column;

      switch (col) {
        case 'patentId':
          comparison = a.patentId.localeCompare(b.patentId);
          break;
        case 'eventType':
          comparison = a.eventType.localeCompare(b.eventType);
          break;
        case 'jurisdiction':
          comparison = a.jurisdiction.localeCompare(b.jurisdiction);
          break;
        case 'dueDate':
          comparison = new Date(a.dueDate).getTime() - new Date(b.dueDate).getTime();
          break;
        case 'feeAmount':
          comparison = (a.feeAmount || 0) - (b.feeAmount || 0);
          break;
        case 'status':
          comparison = a.status.localeCompare(b.status);
          break;
        default:
          return 0;
      }

      return tableState.sort.direction === 'asc' ? comparison : -comparison;
    });
  }, [events, tableState.sort]);

  const handleSelectionChange = useCallback((ids: Set<string>) => {
    setSelectedIds(ids);
  }, []);

  const getRowId = useCallback((row: LifecycleEvent) => row.id, []);

  const handleSelectAllToggle = useCallback(() => {
    if (selectedIds.size === events.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(events.map((e) => e.id)));
    }
  }, [selectedIds, events]);

  const handleBulkMarkHandled = useCallback(() => {
    onMarkHandled(Array.from(selectedIds));
    setSelectedIds(new Set());
  }, [selectedIds, onMarkHandled]);

  const columns: Column<LifecycleEvent>[] = useMemo(
    () => [
      {
        header: t('lifecycle.table.patent_id'),
        accessor: 'patentId',
        sortable: true,
        sortKey: 'patentId',
      },
      {
        header: t('lifecycle.table.event_type'),
        accessor: (row) => (row.eventType || '').replace(/_/g, ' '),
        sortable: true,
        sortKey: 'eventType',
      },
      {
        header: t('lifecycle.table.jurisdiction'),
        accessor: 'jurisdiction',
        sortable: true,
        sortKey: 'jurisdiction',
        className: 'w-28',
      },
      {
        header: t('lifecycle.table.due_date'),
        sortable: true,
        sortKey: 'dueDate',
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
        header: t('lifecycle.table.fee'),
        accessor: (row) =>
          row.feeAmount ? `${row.currency} ${row.feeAmount.toLocaleString()}` : '-',
        sortable: true,
        sortKey: 'feeAmount',
      },
      {
        header: t('lifecycle.table.status'),
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
        sortable: true,
        sortKey: 'status',
      },
    ],
    [t]
  );

  return (
    <Card padding="none" className="min-h-[500px] flex flex-col">
      <div className="px-6 py-4 border-b border-slate-200 flex justify-between items-center bg-slate-50 rounded-t-lg">
        <div className="flex items-center space-x-3">
          <div className="bg-white p-2 rounded-lg border border-slate-200 shadow-sm">
            <Calendar className="w-5 h-5 text-blue-600" />
          </div>
          <div>
            <h3 className="font-semibold text-slate-800">{t('lifecycle.calendar.title')}</h3>
            <p className="text-xs text-slate-500">{events.length} {t('lifecycle.calendar.upcoming')}</p>
          </div>
        </div>
        <div className="flex space-x-2">
          <Button
            size="sm"
            variant="secondary"
            leftIcon={<Check className="w-4 h-4" />}
            onClick={handleSelectAllToggle}
          >
            {selectedIds.size === events.length ? t('lifecycle.calendar.deselect_all') : t('lifecycle.calendar.select_all')}
          </Button>

          {selectedIds.size > 0 && (
            <Button
              size="sm"
              variant="primary"
              leftIcon={<CheckSquare className="w-4 h-4" />}
              onClick={handleBulkMarkHandled}
            >
              {t('lifecycle.calendar.mark_handled')} ({selectedIds.size})
            </Button>
          )}

          <Button
            size="sm"
            variant="outline"
            leftIcon={<Calendar className="w-4 h-4" />}
            onClick={onExport}
          >
            {t('lifecycle.calendar.export')}
          </Button>
        </div>
      </div>
      <div className="flex-1">
        <DataTable
          columns={columns}
          data={sortedEvents}
          isLoading={loading}
          sortState={tableState.sort}
          onSort={tableState.setSortState}
          showToolbar
          enableExport
          enableColumnVisibility
          selectable
          selection={{
            selectedIds,
            onSelectionChange: handleSelectionChange,
            getRowId,
          }}
          pagination={{
            currentPage: 1,
            totalPages: 1,
            onPageChange: () => {},
          }}
        />
      </div>
    </Card>
  );
};

export default DeadlineTable;
