import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePartners } from '../../hooks/usePartner';
import AdminView from './AdminView';
import AgencyView from './AgencyView';
import CounselView from './CounselView';
import APIPortal from './APIPortal';
import LoadingSpinner from '../../components/ui/LoadingSpinner';

type ViewMode = 'admin' | 'agency' | 'counsel' | 'api';

const PartnerPortal: React.FC = () => {
  const { t } = useTranslation();
  const { data: partners, loading, error } = usePartners();
  const [activeView, setActiveView] = useState<ViewMode>('admin');

  if (loading) {
    return <div className="h-full flex items-center justify-center"><LoadingSpinner size="lg" /></div>;
  }

  if (error) {
    return <div className="p-8 text-center text-red-600">Error loading partner data: {error}</div>;
  }

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]">
      <div className="bg-white border-b border-slate-200 sticky top-0 z-10 mb-6 flex justify-between items-center">
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
        {activeView === 'admin' && <AdminView partners={partners || []} loading={loading} />}
        {activeView === 'agency' && <AgencyView />}
        {activeView === 'counsel' && <CounselView />}
        {activeView === 'api' && <APIPortal />}
      </div>
    </div>
  );
};

export default PartnerPortal;
