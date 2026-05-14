import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePartners } from '../../hooks/usePartner';
import AdminView from './AdminView';
import AgencyView from './AgencyView';
import CounselView from './CounselView';
import APIPortal from './APIPortal';
import PageError from '../../components/ui/PageError';
import { SkeletonCard } from '../../components/ui/Skeleton';

type ViewMode = 'admin' | 'agency' | 'counsel' | 'api';

const PartnerPortal: React.FC = () => {
  const { t } = useTranslation();
  const { data: partners, loading, error, refetch } = usePartners();
  const [activeView, setActiveView] = useState<ViewMode>('admin');

  if (loading) {
    return (
      <div className="space-y-6 pb-12">
        {/* Sub-nav skeleton */}
        <div className="bg-white border-b border-slate-200 rounded-lg p-4 animate-pulse">
          <div className="flex gap-8">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-4 w-24 bg-slate-200 rounded" />
            ))}
          </div>
        </div>

        {/* Content skeleton */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <SkeletonCard key={i} rows={4} className="h-48" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <PageError
        error={error}
        onRetry={refetch}
        title={t('partners.error_title', 'Failed to load partner data')}
        description={t('partners.error_desc', 'There was a problem fetching partner information.')}
      />
    );
  }

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]">
      <div className="bg-white border-b border-slate-200 sticky top-16 z-10 mb-6 flex justify-between items-center">
        <nav className="flex space-x-8 px-1 overflow-x-auto" aria-label="Portal Views">
          {[
            { id: 'admin', label: t('partners.nav.admin') },
            { id: 'agency', label: t('partners.nav.agency') },
            { id: 'counsel', label: t('partners.nav.counsel') },
            { id: 'api', label: t('partners.nav.api') },
          ].map((item) => (
            <button
              key={item.id}
              onClick={() => setActiveView(item.id as ViewMode)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors
                ${activeView === item.id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
                }
              `}
            >
              {item.label}
            </button>
          ))}
        </nav>
      </div>

      <div className="flex-1 overflow-y-auto pb-8">
        {activeView === 'admin' && <AdminView partners={partners || []} loading={false} />}
        {activeView === 'agency' && <AgencyView />}
        {activeView === 'counsel' && <CounselView />}
        {activeView === 'api' && <APIPortal />}
      </div>
    </div>
  );
};

export default PartnerPortal;
