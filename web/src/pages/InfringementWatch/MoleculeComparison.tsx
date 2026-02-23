import React, { useEffect, useState } from 'react';
import Card from '../../components/ui/Card';
import { moleculeService } from '../../services/molecule.service';
import { Molecule } from '../../types/domain';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import MoleculeViewer from '../../components/ui/MoleculeViewer';

interface MoleculeComparisonProps {
  triggerMoleculeId: string;
  similarityScore: number;
}

const MoleculeComparison: React.FC<MoleculeComparisonProps> = ({ triggerMoleculeId, similarityScore }) => {
  const [triggerMolecule, setTriggerMolecule] = useState<Molecule | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchMolecule = async () => {
      setLoading(true);
      try {
        const response = await moleculeService.getMoleculeById(triggerMoleculeId);
        setTriggerMolecule(response.data);
      } catch (err) {
        console.error('Failed to fetch molecule', err);
      } finally {
        setLoading(false);
      }
    };

    if (triggerMoleculeId) {
      fetchMolecule();
    }
  }, [triggerMoleculeId]);

  // Mock patent molecule for comparison display
  const mockPatentMoleculeSmiles = "C1=CC=C(C=C1)N(C2=CC=CC=C2)C3=CC=C(C=C3)C4=CC=C(C=C4)N(C5=CC=CC=C5)C6=CC=CC=C6";

  if (loading) return <div className="h-40 flex items-center justify-center"><LoadingSpinner /></div>;

  return (
    <Card className="mb-6">
      <h3 className="text-lg font-semibold text-slate-800 mb-4">Molecular Structure Comparison</h3>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-start relative">
        {/* Trigger Molecule */}
        <div className="bg-slate-50 p-4 rounded-lg border border-slate-200">
          <div className="flex justify-between items-center mb-2">
            <h4 className="font-medium text-slate-700">Trigger Molecule</h4>
            <span className="text-xs bg-blue-100 text-blue-700 px-2 py-1 rounded">
              {triggerMolecule?.id || triggerMoleculeId}
            </span>
          </div>
          <div className="bg-white p-3 rounded border border-slate-200 flex justify-center h-48">
            {triggerMolecule?.smiles ? (
              <MoleculeViewer smiles={triggerMolecule.smiles} width={250} height={180} />
            ) : (
              <span className="text-slate-400 self-center">No structure available</span>
            )}
          </div>
          <div className="mt-2 text-xs text-slate-400 font-mono truncate">
            {triggerMolecule?.smiles}
          </div>
        </div>

        {/* Similarity Bridge (Visual) */}
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-white px-2 py-1 z-10 hidden md:block">
           <div className="flex flex-col items-center">
             <span className="text-xs font-bold text-slate-400 mb-1">Similarity</span>
             <div className="w-16 h-1 bg-slate-200 rounded-full">
               <div
                 className="h-full bg-red-500 rounded-full"
                 style={{ width: `${similarityScore * 100}%` }}
               ></div>
             </div>
             <span className="text-sm font-bold text-red-600 mt-1">{(similarityScore * 100).toFixed(0)}%</span>
           </div>
        </div>

        {/* Patent Molecule */}
        <div className="bg-slate-50 p-4 rounded-lg border border-slate-200">
          <div className="flex justify-between items-center mb-2">
            <h4 className="font-medium text-slate-700">Patent Coverage</h4>
            <span className="text-xs bg-amber-100 text-amber-700 px-2 py-1 rounded">
              Claim 1 (Markush)
            </span>
          </div>
          <div className="bg-white p-3 rounded border border-slate-200 flex justify-center h-48">
            <MoleculeViewer smiles={mockPatentMoleculeSmiles} width={250} height={180} />
          </div>
          <div className="mt-2 text-xs text-slate-400">
            Extracted from Claim 1
          </div>
        </div>
      </div>
    </Card>
  );
};

export default MoleculeComparison;
