import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLifecycle } from '../../hooks/useLifecycle';
import FilterPanel from './FilterPanel';
import DeadlineTable from './DeadlineTable';
import AnnuityManager from './AnnuityManager';
import LegalStatusMonitor from './LegalStatusMonitor';
import Button from '../../components/ui/Button';
import { Download } from 'lucide-react';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import { Jurisdiction } from '../../types/domain';

type Tab = 'calendar' | 'annuity' | 'status';

const LifecycleConsole: React.FC = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<Tab>('calendar');
  const [filters, setFilters] = useState({
    jurisdiction: 'All' as Jurisdiction | 'All',
    eventType: 'All',
    riskLevel: 'All',
    startDate: '',
    endDate: '',
  });

  // Fetch all events initially, filter locally for mock simplicity
  const { data: events, loading, error, refetch } = useLifecycle();

  const handleFilterChange = (newFilters: any) => {
    setFilters(newFilters);
  };

  const handleResetFilters = () => {
    setFilters({
      jurisdiction: 'All',
      eventType: 'All',
      riskLevel: 'All',
      startDate: '',
      endDate: '',
    });
  };

  // Filter Logic
  const filteredEvents = events?.filter((event) => {
    if (filters.jurisdiction !== 'All' && event.jurisdiction !== filters.jurisdiction) return false;
    if (filters.eventType !== 'All' && event.eventType !== filters.eventType) return false;

    // Date Range Logic
    if (filters.startDate && new Date(event.dueDate) < new Date(filters.startDate)) return false;
    if (filters.endDate && new Date(event.dueDate) > new Date(filters.endDate)) return false;

    // Risk Level Logic (Mock)
    const due = new Date(event.dueDate);
    const today = new Date();
    const diffDays = Math.ceil((due.getTime() - today.getTime()) / (1000 * 60 * 60 * 24));

    if (filters.riskLevel === 'Overdue' && diffDays >= 0) return false;
    if (filters.riskLevel === 'Due 7d' && (diffDays < 0 || diffDays > 7)) return false;
    if (filters.riskLevel === 'Due 30d' && (diffDays < 0 || diffDays > 30)) return false;
    if (filters.riskLevel === 'Due 90d' && (diffDays < 0 || diffDays > 90)) return false;

    return true;
  }) || [];

  const handleMarkHandled = (ids: string[]) => {
    console.log('Mark handled:', ids);
    // In a real app, call API mutation here
    alert(`Marked ${ids.length} events as handled`);
    refetch(); // Mock refresh
  };

  const handlePayAnnuity = (ids: string[]) => {
    console.log('Pay annuity:', ids);
    alert(`Initiating payment for ${ids.length} annuities`);
  };

  const handleExport = () => {
    const csvContent = "data:text/csv;charset=utf-8,"
      + "PatentID,Jurisdiction,EventType,DueDate,Status\n"
      + filteredEvents.map(e => `${e.patentId},${e.jurisdiction},${e.eventType},${e.dueDate},${e.status}`).join("\n");

    const encodedUri = encodeURI(csvContent);
    const link = document.createElement("a");
    link.setAttribute("href", encodedUri);
    link.setAttribute("download", "lifecycle_events.csv");
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  if (error) {
    return <div className="text-red-600 p-8">Error: {error}</div>;
  }

  return (
    <div className="flex flex-col lg:flex-row gap-6 h-[calc(100vh-8rem)]">
      {/* Sidebar Filters */}
      <div className="w-full lg:w-72 flex-shrink-0">
        <FilterPanel
          filters={filters}
          onFilterChange={handleFilterChange}
          onReset={handleResetFilters}
        />
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col min-w-0">
        <div className="flex justify-between items-center mb-6">
          <h1 className="text-2xl font-bold text-slate-900">{t('lifecycle.title')}</h1>
          <div className="flex space-x-2">
            <Button variant="outline" onClick={handleExport} leftIcon={<Download className="w-4 h-4" />}>
              {t('lifecycle.calendar.export')}
            </Button>
          </div>
        </div>

        {/* Tabs */}
        <div className="bg-white border-b border-slate-200 mb-6 sticky top-0 z-10">
          <nav className="-mb-px flex space-x-8" aria-label="Tabs">
            {[
              { id: 'calendar', name: t('lifecycle.tabs.calendar') },
              { id: 'annuity', name: t('lifecycle.tabs.annuity') },
              { id: 'status', name: t('lifecycle.tabs.status') },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as Tab)}
                className={`
                  whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors
                  ${activeTab === tab.id
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
                  }
                `}
              >
                {tab.name}
              </button>
            ))}
          </nav>
        </div>

        {/* Tab Content */}
        <div className="flex-1 overflow-auto pb-8">
          {loading ? (
            <div className="flex justify-center items-center h-64">
              <LoadingSpinner size="lg" />
            </div>
          ) : (
            <>
              {activeTab === 'calendar' && (
                <DeadlineTable
                  events={filteredEvents}
                  loading={loading}
                  onMarkHandled={handleMarkHandled}
                  onExport={handleExport}
                />
              )}
              {activeTab === 'annuity' && (
                <AnnuityManager
                  events={filteredEvents} // Should we pass all events or just filtered? Filtered for consistency
                  loading={loading}
                  onPay={handlePayAnnuity}
                />
              )}
              {activeTab === 'status' && (
                <LegalStatusMonitor
                  events={events || []} // Monitor uses all events to aggregate status
                  loading={loading}
                />
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default LifecycleConsole;
