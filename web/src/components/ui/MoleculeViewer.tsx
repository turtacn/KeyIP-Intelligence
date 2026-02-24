import React, { useEffect, useState } from 'react';
import { getRDKit, RDKitModule, RDKitMolecule } from '../../utils/rdkitLoader';
import LoadingSpinner from './LoadingSpinner';

interface MoleculeViewerProps {
  smiles: string;
  width?: number;
  height?: number;
  className?: string;
}

const MoleculeViewer: React.FC<MoleculeViewerProps> = ({
  smiles,
  width = 300,
  height = 200,
  className = ''
}) => {
  const [svg, setSvg] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let molecule: RDKitMolecule | null = null;
    let mounted = true;

    const renderMolecule = async () => {
      if (!smiles) {
        setLoading(false);
        return;
      }

      try {
        setLoading(true);
        const rdkit: RDKitModule = await getRDKit();

        if (!mounted) return;

        molecule = rdkit.get_mol(smiles);

        if (molecule) {
          const svgString = molecule.get_svg(width, height);
          setSvg(svgString);
        } else {
          setError('Invalid SMILES');
        }
      } catch (err) {
        console.error('RDKit rendering error:', err);
        setError('Failed to load renderer');
      } finally {
        if (mounted) {
          setLoading(false);
          if (molecule) molecule.delete();
        }
      }
    };

    renderMolecule();

    return () => {
      mounted = false;
      if (molecule) molecule.delete();
    };
  }, [smiles, width, height]);

  if (loading) {
    return (
      <div className={`flex justify-center items-center bg-slate-50 rounded ${className}`} style={{ width, height }}>
        <LoadingSpinner size="sm" />
      </div>
    );
  }

  if (error) {
    return (
      <div className={`flex justify-center items-center bg-red-50 text-red-400 text-xs rounded ${className}`} style={{ width, height }}>
        {error}
      </div>
    );
  }

  return (
    <div
      className={`molecule-viewer ${className}`}
      style={{ width, height }}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
};

export default MoleculeViewer;
