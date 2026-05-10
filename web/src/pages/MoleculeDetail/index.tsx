import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { moleculeService } from '../../services/molecule.service';
import { Molecule } from '../../types/domain';
import Card from '../../components/ui/Card';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import Button from '../../components/ui/Button';
import MoleculeViewer from '../../components/ui/MoleculeViewer';
import { ArrowLeft, AlertCircle, Beaker } from 'lucide-react';

const MoleculeDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [molecule, setMolecule] = useState<Molecule | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let mounted = true;

    const fetchMolecule = async () => {
      setLoading(true);
      setError(null);
      try {
        const response = await moleculeService.getMoleculeById(id);
        if (mounted) {
          setMolecule(response.data);
        }
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to load molecule');
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };

    fetchMolecule();
    return () => { mounted = false; };
  }, [id]);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 text-center text-red-600 bg-red-50 rounded-lg">
        <AlertCircle className="w-8 h-8 mx-auto mb-3" />
        <p className="font-semibold">Error loading molecule</p>
        <p className="text-sm mt-1">{error}</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => navigate(-1)}
          leftIcon={<ArrowLeft className="w-4 h-4" />}
        >
          Go Back
        </Button>
      </div>
    );
  }

  if (!molecule) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] text-slate-500">
        <Beaker className="w-16 h-16 mb-4 text-slate-300" />
        <h2 className="text-xl font-semibold text-slate-700 mb-2">Molecule Not Found</h2>
        <p className="mb-6">The requested molecule could not be found.</p>
        <Button
          variant="outline"
          onClick={() => navigate('/patent-mining')}
          leftIcon={<ArrowLeft className="w-4 h-4" />}
        >
          Back to Patent Mining
        </Button>
      </div>
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
        Back
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
        <Card className="lg:col-span-1" header={<span className="font-semibold text-slate-800">Structure</span>}>
          <div className="flex justify-center p-4">
            {molecule.smiles ? (
              <MoleculeViewer smiles={molecule.smiles} width={280} height={200} />
            ) : (
              <div className="flex items-center justify-center w-[280px] h-[200px] bg-slate-50 rounded text-slate-400 text-sm">
                No structure data
              </div>
            )}
          </div>
          {molecule.smiles && (
            <div className="mt-2 p-2 bg-slate-50 rounded text-xs font-mono text-slate-600 break-all">
              SMILES: {molecule.smiles}
            </div>
          )}
        </Card>

        {/* Properties */}
        <Card className="lg:col-span-2" header={<span className="font-semibold text-slate-800">Properties</span>}>
          <div className="space-y-4">
            {molecule.molecularWeight && (
              <div className="flex justify-between items-center py-2 border-b border-slate-100">
                <span className="text-sm text-slate-600">Molecular Weight</span>
                <span className="text-sm font-semibold text-slate-800">{molecule.molecularWeight.toFixed(2)} g/mol</span>
              </div>
            )}
            {molecule.inchi && (
              <div className="py-2 border-b border-slate-100">
                <span className="text-sm text-slate-600 block mb-1">InChI</span>
                <span className="text-xs font-mono text-slate-700 break-all">{molecule.inchi}</span>
              </div>
            )}
            {molecule.fingerprint && (
              <div className="py-2">
                <span className="text-sm text-slate-600 block mb-1">Fingerprint</span>
                <span className="text-xs font-mono text-slate-500 break-all">{molecule.fingerprint.substring(0, 64)}...</span>
              </div>
            )}
            {!molecule.molecularWeight && !molecule.inchi && !molecule.fingerprint && (
              <p className="text-slate-400 text-sm">No property data available</p>
            )}
          </div>
        </Card>
      </div>

      {/* Material Properties */}
      {molecule.properties && molecule.properties.length > 0 && (
        <Card header={<span className="font-semibold text-slate-800">Material Properties ({molecule.properties.length})</span>}>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200">
              <thead className="bg-slate-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Type</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Value</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Unit</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Condition</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Source</th>
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
