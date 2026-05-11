import React, { useState, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Molecule } from '../../types/domain';
import Card from '../ui/Card';
import Button from '../ui/Button';
import { X, Plus, AlertCircle } from 'lucide-react';

// ─── Property Definition ──────────────────────────────────────────────────

interface PropertyDef {
  key: string;
  label: string;
  unit: string;
  higherIsBetter: boolean;
  matchTypes: string[];
}

const PROPERTIES: PropertyDef[] = [
  {
    key: 'homo',
    label: 'HOMO',
    unit: 'eV',
    higherIsBetter: true, // less negative = closer to vacuum level
    matchTypes: ['HOMO', 'homo', 'Homo', 'HOMO (eV)'],
  },
  {
    key: 'lumo',
    label: 'LUMO',
    unit: 'eV',
    higherIsBetter: true,
    matchTypes: ['LUMO', 'lumo', 'Lumo', 'LUMO (eV)'],
  },
  {
    key: 'bandgap',
    label: 'Band Gap',
    unit: 'eV',
    higherIsBetter: false, // smaller bandgap = more conductive
    matchTypes: ['bandgap', 'BandGap', 'Band gap', 'Eg', 'E_g', 'BandGap (eV)'],
  },
  {
    key: 'triplet_energy',
    label: 'Triplet Energy',
    unit: 'eV',
    higherIsBetter: true,
    matchTypes: ['triplet_energy', 'Triplet Energy', 'ET', 'E_T', 'Triplet Energy (eV)'],
  },
  {
    key: 'molecular_weight',
    label: 'Molecular Weight',
    unit: 'g/mol',
    higherIsBetter: false, // lighter is generally preferred
    matchTypes: ['molecular_weight', 'Molecular Weight', 'Mw', 'MW', 'molecular weight'],
  },
  {
    key: 'logP',
    label: 'LogP',
    unit: '',
    higherIsBetter: false,
    matchTypes: ['logP', 'LogP', 'Log P', 'log P', 'LogP (octanol-water)'],
  },
];

// ─── Helpers ──────────────────────────────────────────────────────────────

function extractValue(molecule: Molecule, propDef: PropertyDef): number | null {
  // Special case: molecular_weight from top-level field
  if (propDef.key === 'molecular_weight' && molecule.molecularWeight != null) {
    return molecule.molecularWeight;
  }

  if (!molecule.properties || molecule.properties.length === 0) return null;

  for (const mp of molecule.properties) {
    if (propDef.matchTypes.includes(mp.type)) {
      const num = typeof mp.value === 'string' ? parseFloat(mp.value) : mp.value;
      if (typeof num === 'number' && !isNaN(num)) return num;
    }
  }
  return null;
}

/** Format a numeric value to a fixed number of decimal places. */
function formatValue(value: number | null, unit: string): string {
  if (value === null) return '-';
  return `${value.toFixed(3)}${unit ? ' ' + unit : ''}`;
}

/** Determine CSS class for the cell background based on best/worst. */
function getCellColor(
  value: number | null,
  allValues: (number | null)[],
  higherIsBetter: boolean,
): string {
  if (value === null) return '';

  const numericValues = allValues.filter((v): v is number => v !== null);
  if (numericValues.length < 2) return '';

  const best = higherIsBetter ? Math.max(...numericValues) : Math.min(...numericValues);
  const worst = higherIsBetter ? Math.min(...numericValues) : Math.max(...numericValues);

  if (value === best) return 'bg-green-50 text-green-800 font-semibold';
  if (value === worst) return 'bg-red-50 text-red-800 font-semibold';
  return '';
}

function getMoleculeLabel(molecule: Molecule): string {
  return molecule.name || molecule.id || molecule.smiles?.substring(0, 20) || 'Unknown';
}

// ─── Props ────────────────────────────────────────────────────────────────

interface PropertyComparisonProps {
  molecules: Molecule[];
  loading?: boolean;
  /** Called when user wants to remove a molecule from comparison. */
  onRemoveMolecule?: (id: string) => void;
  /** Called when user submits a SMILES to add. */
  onAddMolecule?: (smiles: string) => void;
  /** Error message from parent (e.g. when adding fails). */
  addError?: string | null;
  /** Whether the parent is currently fetching a new molecule. */
  adding?: boolean;
}

// ─── Component ────────────────────────────────────────────────────────────

const PropertyComparison: React.FC<PropertyComparisonProps> = ({
  molecules,
  loading = false,
  onRemoveMolecule,
  onAddMolecule,
  addError,
  adding = false,
}) => {
  const { t } = useTranslation();
  const [smilesInput, setSmilesInput] = useState('');
  const [inputError, setInputError] = useState<string | null>(null);

  // Build the property matrix: rows = properties, columns = molecules
  const propertyMatrix = useMemo(() => {
    return PROPERTIES.map((propDef) => {
      const values = molecules.map((mol) => extractValue(mol, propDef));
      return { propDef, values };
    });
  }, [molecules]);

  const moleculeLabels = useMemo(() => molecules.map(getMoleculeLabel), [molecules]);

  const handleAdd = useCallback(() => {
    const trimmed = smilesInput.trim();
    if (!trimmed) {
      setInputError('Please enter a SMILES string');
      return;
    }
    // Basic SMILES validation: should not be empty
    setInputError(null);
    onAddMolecule?.(trimmed);
    setSmilesInput('');
  }, [smilesInput, onAddMolecule]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        handleAdd();
      }
    },
    [handleAdd],
  );

  const hasData = molecules.length > 0;

  if (!hasData && !loading) {
    return (
      <Card
        header={
          <span className="font-semibold text-slate-800">
            {t('molecule.property_comparison', 'Property Comparison')}
          </span>
        }
      >
        <div className="flex flex-col items-center justify-center py-8 text-slate-400">
          <AlertCircle className="w-8 h-8 mb-2" />
          <p className="text-sm">{t('molecule.no_comparison_data', 'No molecules to compare. Add a molecule to get started.')}</p>
        </div>

        {/* Add molecule form */}
        {onAddMolecule && (
          <div className="mt-4 flex gap-2">
            <input
              type="text"
              value={smilesInput}
              onChange={(e) => {
                setSmilesInput(e.target.value);
                setInputError(null);
              }}
              onKeyDown={handleKeyDown}
              placeholder={t('molecule.enter_smiles', 'Enter SMILES or molecule ID...')}
              disabled={adding}
              className="flex-1 px-3 py-1.5 text-sm border border-slate-300 rounded-md focus:ring-blue-500 focus:border-blue-500 bg-white disabled:opacity-50"
            />
            <Button
              variant="primary"
              size="sm"
              onClick={handleAdd}
              disabled={adding || !smilesInput.trim()}
              leftIcon={<Plus className="w-3.5 h-3.5" />}
            >
              {adding ? t('common.adding', 'Adding...') : t('common.add', 'Add')}
            </Button>
          </div>
        )}
        {inputError && (
          <p className="mt-1 text-xs text-red-500">{inputError}</p>
        )}
        {addError && (
          <p className="mt-1 text-xs text-red-500">{addError}</p>
        )}
      </Card>
    );
  }

  return (
    <Card
      header={
        <span className="font-semibold text-slate-800">
          {t('molecule.property_comparison', 'Property Comparison')}
          {molecules.length > 1 && (
            <span className="ml-2 text-sm font-normal text-slate-400">
              ({molecules.length} {t('molecule.molecules', 'molecules')})
            </span>
          )}
        </span>
      }
      bodyClassName="p-0"
    >
      {/* Loading overlay */}
      {loading && (
        <div className="flex items-center justify-center py-8">
          <div className="w-5 h-5 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
          <span className="ml-2 text-sm text-slate-400">
            {t('common.loading', 'Loading...')}
          </span>
        </div>
      )}

      {!loading && (
        <>
          {/* Responsive table wrapper */}
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200">
              <thead className="bg-slate-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider sticky left-0 bg-slate-50 z-10 whitespace-nowrap min-w-[120px]">
                    {t('molecule.property', 'Property')}
                  </th>
                  {molecules.map((mol, idx) => (
                    <th
                      key={mol.id || idx}
                      className="px-4 py-3 text-center text-xs font-medium text-slate-500 uppercase tracking-wider min-w-[130px] max-w-[180px] relative"
                    >
                      <div className="flex items-center justify-center gap-1">
                        <span className="truncate block max-w-[120px]" title={getMoleculeLabel(mol)}>
                          {moleculeLabels[idx]}
                        </span>
                        {onRemoveMolecule && molecules.length > 1 && (
                          <button
                            onClick={() => onRemoveMolecule(mol.id)}
                            className="flex-shrink-0 text-slate-300 hover:text-red-500 transition-colors"
                            aria-label={t('molecule.remove_from_comparison', 'Remove from comparison')}
                            title={t('molecule.remove', 'Remove')}
                          >
                            <X className="w-3 h-3" />
                          </button>
                        )}
                      </div>
                      {/* SMILES subtitle */}
                      {mol.smiles && (
                        <div className="text-[10px] font-mono text-slate-400 normal-case mt-0.5 truncate max-w-[150px] mx-auto" title={mol.smiles}>
                          {mol.smiles.length > 24
                            ? mol.smiles.substring(0, 22) + '...'
                            : mol.smiles}
                        </div>
                      )}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-slate-200">
                {propertyMatrix.map(({ propDef, values }) => (
                  <tr key={propDef.key} className="hover:bg-slate-50/50 transition-colors">
                    <td className="px-4 py-3 text-sm font-medium text-slate-700 sticky left-0 bg-white z-10 whitespace-nowrap">
                      {propDef.label}
                      {propDef.unit && (
                        <span className="ml-1 text-xs font-normal text-slate-400">
                          ({propDef.unit})
                        </span>
                      )}
                    </td>
                    {values.map((val, idx) => (
                      <td
                        key={idx}
                        className={`px-4 py-3 text-sm text-center whitespace-nowrap transition-colors ${getCellColor(val, values, propDef.higherIsBetter)}`}
                      >
                        {formatValue(val, '')}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Legend */}
          {molecules.length >= 2 && (
            <div className="px-4 py-2 border-t border-slate-100 bg-slate-50/50 flex items-center gap-4 text-xs text-slate-500">
              <div className="flex items-center gap-1.5">
                <span className="inline-block w-3 h-3 rounded bg-green-50 border border-green-200" />
                <span>{t('molecule.best_value', 'Best value')}</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="inline-block w-3 h-3 rounded bg-red-50 border border-red-200" />
                <span>{t('molecule.worst_value', 'Worst value')}</span>
              </div>
            </div>
          )}

          {/* Add molecule form */}
          {onAddMolecule && (
            <div className="px-4 py-3 border-t border-slate-100">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={smilesInput}
                  onChange={(e) => {
                    setSmilesInput(e.target.value);
                    setInputError(null);
                  }}
                  onKeyDown={handleKeyDown}
                  placeholder={t('molecule.enter_smiles', 'Enter SMILES or molecule ID...')}
                  disabled={adding}
                  className="flex-1 px-3 py-1.5 text-sm border border-slate-300 rounded-md focus:ring-blue-500 focus:border-blue-500 bg-white disabled:opacity-50"
                />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleAdd}
                  disabled={adding || !smilesInput.trim()}
                  leftIcon={<Plus className="w-3.5 h-3.5" />}
                >
                  {adding ? t('common.adding', 'Adding...') : t('common.add', 'Add')}
                </Button>
              </div>
              {inputError && (
                <p className="mt-1 text-xs text-red-500">{inputError}</p>
              )}
              {addError && (
                <p className="mt-1 text-xs text-red-500">{addError}</p>
              )}
            </div>
          )}
        </>
      )}
    </Card>
  );
};

export { PropertyComparison, PROPERTIES, extractValue };
export default PropertyComparison;
