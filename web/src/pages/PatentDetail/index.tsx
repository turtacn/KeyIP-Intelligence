import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { patentService } from '../../services/patent.service';
import { Patent } from '../../types/domain';
import Card from '../../components/ui/Card';
import Badge from '../../components/ui/Badge';
import StatusBadge from '../../components/ui/StatusBadge';
import Button from '../../components/ui/Button';
import PageError from '../../components/ui/PageError';
import { SkeletonCard, SkeletonLine } from '../../components/ui/Skeleton';
import EmptyState from '../../components/ui/EmptyState';
import { ArrowLeft, ExternalLink, FileText } from 'lucide-react';

const PatentDetail: React.FC = () => {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [patent, setPatent] = useState<Patent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchPatent = async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const response = await patentService.getPatentById(id);
      setPatent(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load patent');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!id) return;
    let mounted = true;
    const fetch = async () => {
      await fetchPatent();
      if (!mounted) return;
    };
    fetch();
    return () => { mounted = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (loading) {
    return (
      <div className="space-y-6 pb-12">
        {/* Back button skeleton */}
        <div className="animate-pulse h-4 w-16 bg-slate-200 rounded" />

        {/* Header skeleton */}
        <div className="animate-pulse space-y-3">
          <div className="h-8 w-3/4 bg-slate-200 rounded" />
          <SkeletonLine width="40%" />
        </div>

        {/* Summary card skeleton */}
        <SkeletonCard rows={2} className="h-32" />

        {/* Abstract skeleton */}
        <SkeletonCard rows={4} className="h-40" />

        {/* Two-column skeleton */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <SkeletonCard rows={3} className="h-32" />
          <SkeletonCard rows={3} className="h-32" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <PageError
        error={error}
        onRetry={fetchPatent}
        title={t('patent.error_loading')}
        description={t('patent.error_loading_desc', 'There was a problem loading this patent.')}
      />
    );
  }

  if (!patent) {
    return (
      <EmptyState
        icon={FileText}
        title={t('patent.not_found_title')}
        description={t('patent.not_found_desc')}
        action={
          <Button
            variant="outline"
            onClick={() => navigate('/patent-mining')}
            leftIcon={<ArrowLeft className="w-4 h-4" />}
          >
            {t('patent.back_to_mining')}
          </Button>
        }
      />
    );
  }

  return (
    <div className="space-y-6 pb-12">
      {/* Back navigation */}
      <button
        onClick={() => navigate(-1)}
        className="flex items-center text-sm text-slate-500 hover:text-slate-700 transition-colors"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        {t('patent.back')}
      </button>

      {/* Header */}
      <div className="flex flex-col md:flex-row justify-between items-start gap-4">
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-2xl font-bold text-slate-900">{patent.title}</h1>
            <ExternalLink className="w-5 h-5 text-slate-400 flex-shrink-0" />
          </div>
          <p className="text-slate-500">
            {patent.publicationNumber}
          </p>
        </div>
        <StatusBadge status={patent.legalStatus.toLowerCase() as any} label={patent.legalStatus} />
      </div>

      {/* Summary Card */}
      <Card>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          <div>
            <p className="text-xs font-medium text-slate-500 uppercase tracking-wider mb-1">{t('patent.assignee')}</p>
            <p className="text-sm font-semibold text-slate-800">{patent.assignee}</p>
          </div>
          <div>
            <p className="text-xs font-medium text-slate-500 uppercase tracking-wider mb-1">{t('patent.filing_date')}</p>
            <p className="text-sm font-semibold text-slate-800">{patent.filingDate}</p>
          </div>
          <div>
            <p className="text-xs font-medium text-slate-500 uppercase tracking-wider mb-1">{t('patent.publication_date')}</p>
            <p className="text-sm font-semibold text-slate-800">{patent.publicationDate}</p>
          </div>
          {patent.grantDate && (
            <div>
              <p className="text-xs font-medium text-slate-500 uppercase tracking-wider mb-1">{t('patent.grant_date')}</p>
              <p className="text-sm font-semibold text-slate-800">{patent.grantDate}</p>
            </div>
          )}
        </div>
      </Card>

      {/* Abstract */}
      <Card header={<span className="font-semibold text-slate-800">{t('patent.abstract')}</span>}>
        <p className="text-slate-700 leading-relaxed">{patent.abstract}</p>
      </Card>

      {/* Inventors & IPC */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card header={<span className="font-semibold text-slate-800">{t('patent.inventors')}</span>}>
          {patent.inventors.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {patent.inventors.map((inventor, idx) => (
                <Badge key={idx} variant="info">{inventor}</Badge>
              ))}
            </div>
          ) : (
            <p className="text-slate-400 text-sm">{t('patent.no_inventors')}</p>
          )}
        </Card>

        <Card header={<span className="font-semibold text-slate-800">{t('patent.ipc_codes')}</span>}>
          {patent.ipcCodes.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {patent.ipcCodes.map((code, idx) => (
                <Badge key={idx} variant="default">{code}</Badge>
              ))}
            </div>
          ) : (
            <p className="text-slate-400 text-sm">{t('patent.no_ipc_codes')}</p>
          )}
        </Card>
      </div>

      {/* Claims */}
      {patent.claims && patent.claims.length > 0 && (
        <Card header={<span className="font-semibold text-slate-800">{t('patent.claims')} ({patent.claims.length})</span>}>
          <div className="space-y-4">
            {patent.claims.map((claim, idx) => (
              <div key={claim.id} className="pb-4 border-b border-slate-100 last:border-b-0 last:pb-0">
                <div className="flex items-center gap-2 mb-2">
                  <Badge variant={claim.type === 'independent' ? 'info' : 'default'}>
                    {claim.type === 'independent' ? t('patent.claim_independent') : t('patent.claim_dependent')}
                  </Badge>
                  <span className="text-xs text-slate-400">{t('patent.claim_label')} {idx + 1}</span>
                </div>
                <p className="text-sm text-slate-700 leading-relaxed">{claim.text}</p>
                {claim.elements && claim.elements.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {claim.elements.map((elem, eidx) => (
                      <Badge key={eidx} variant="success" size="sm">{elem}</Badge>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Citations */}
      {patent.citations && patent.citations.length > 0 && (
        <Card header={<span className="font-semibold text-slate-800">{t('patent.citations')} ({patent.citations.length})</span>}>
          <div className="flex flex-wrap gap-2">
            {patent.citations.map((citation, idx) => (
              <Badge key={idx} variant="default" size="sm">{citation}</Badge>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
};

export default PatentDetail;
