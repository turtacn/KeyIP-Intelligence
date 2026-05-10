import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { moleculeService } from '../../services/molecule.service';
import { Molecule } from '../../types/domain';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import MoleculeViewer from '../../components/ui/MoleculeViewer';
import PageError from '../../components/ui/PageError';
import { SkeletonCard, SkeletonLine } from '../../components/ui/Skeleton';
import EmptyState from '../../components/ui/EmptyState';
import { ArrowLeft, Beaker } from 'lucide-react';

const MoleculeDetail: React.FC = () => {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [molecule, setMolecule] = useState<Molecule | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchMolecule = async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const response = await moleculeService.getMoleculeById(id);
      setMolecule(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load molecule');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!id) return;
    let mounted = true;
    const fetch = async () => {
      await fetchMolecule();
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
          <div className="h-8 w-1/2 bg-slate-200 rounded" />
          <SkeletonLine width="30%" />
        </div>

        {/* Two-column skeleton */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <SkeletonCard rows={3} className="h-80 lg:col-span-1" />
          <SkeletonCard rows={5} className="h-80 lg:col-span-2" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <PageError
        error={error}
        onRetry={fetchMolecule}
        title={t('molecule.error_loading')}
        description={t('molecule.error_loading_desc', 'There was a problem loading this molecule.')}
      />
    );
  }

  if (!molecule) {
    return (
      <EmptyState
        icon={Beaker}
        title={t('molecule.not_found_title')}
        description={t('molecule.not_found_desc')}
        action={
          <Button
            variant="outline"
            onClick={() => navigate('/patent-mining')}
            leftIcon={<ArrowLeft className="w-4 h-4" />}
          >
            {t('molecule.back_to_mining')}
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
        {t('molecule.back')}
      </button>

      {/* Header */}
      <div className="flex flex-col md:flex-row justify-between items-start gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">
            {molecule.name || `Molecule ${molecule.id}`}
          </h1>
          <p className="text-slate-500 text-sm font-mono mt-1">{molecule.id}</p>
        </div>
      </div>

      {/* Structure & Properties */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Molecular Structure */}
        <Card className="lg:col-span-1" header={<span className="font-semibold text-slate-800">{t('molecule.structure')}</span>}>
          <div className="flex justify-center p-4">
            {molecule.smiles ? (
              <MoleculeViewer smiles={molecule.smiles} width={280} height={200} />
            ) : (
              <div className="flex items-center justify-center w-[280px] h-[200px] bg-slate-50 rounded text-slate-400 text-sm">
                {t('molecule.no_structure')}
              </div>
            )}
          </div>
          {molecule.smiles && (
            <div className="mt-2 p-2 bg-slate-50 rounded text-xs font-mono text-slate-600 break-all">
              {t('molecule.smiles')}: {molecule.smiles}
            </div>
          )}
        </Card>

        {/* Properties */}
        <Card className="lg:col-span-2" header={<span className="font-semibold text-slate-800">{t('molecule.properties')}</span>}>
          <div className="space-y-4">
            {molecule.molecularWeight && (
              <div className="flex justify-between items-center py-2 border-b border-slate-100">
                <span className="text-sm text-slate-600">{t('molecule.molecular_weight')}</span>
                <span className="text-sm font-semibold text-slate-800">{molecule.molecularWeight.toFixed(2)} g/mol</span>
              </div>
            )}
            {molecule.inchi && (
              <div className="py-2 border-b border-slate-100">
                <span className="text-sm text-slate-600 block mb-1">{t('molecule.inchikey')}</span>
                <span className="text-xs font-mono text-slate-700 break-all">{molecule.inchi}</span>
              </div>
            )}
            {molecule.fingerprint && (
              <div className="py-2">
                <span className="text-sm text-slate-600 block mb-1">{t('molecule.fingerprint')}</span>
                <span className="text-xs font-mono text-slate-500 break-all">{molecule.fingerprint.substring(0, 64)}...</span>
              </div>
            )}
            {!molecule.molecularWeight && !molecule.inchi && !molecule.fingerprint && (
              <p className="text-slate-400 text-sm">{t('molecule.no_properties')}</p>
            )}
          </div>
        </Card>
      </div>

      {/* Material Properties */}
      {molecule.properties && molecule.properties.length > 0 && (
        <Card header={<span className="font-semibold text-slate-800">{t('molecule.material_properties')} ({molecule.properties.length})</span>}>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200">
              <thead className="bg-slate-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">{t('molecule.type')}</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">{t('molecule.value')}</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">{t('molecule.unit')}</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">{t('molecule.condition')}</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">{t('molecule.source')}</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-slate-200">
                {molecule.properties.map((prop, idx) => (
                  <tr key={idx}>
                    <td className="px-4 py-3 text-sm font-medium text-slate-800">{prop.type}</td>
                    <td className="px-4 py-3 text-sm text-slate-700">{prop.value}</td>
                    <td className="px-4 py-3 text-sm text-slate-500">{prop.unit}</td>
                    <td className="px-4 py-3 text-sm text-slate-500">{prop.testCondition || '-'}</td>
                    <td className="px-4 py-3 text-sm text-slate-500">{prop.source || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  );
};

export default MoleculeDetail;
